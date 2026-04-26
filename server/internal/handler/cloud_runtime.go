package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/agentra-ai/agentra/server/internal/auth"
	"github.com/agentra-ai/agentra/server/pkg/crypto"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

// RegisterCloudRuntime creates or updates cloud runtime for a workspace
func (h *Handler) RegisterCloudRuntime(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	var req struct {
		GatewayURL         *string `json:"gateway_url"`
		Provider           string  `json:"provider"`
		APIKey             string  `json:"api_key"`
		MaxConcurrentTasks int     `json:"max_concurrent_tasks"`
	}

	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 10*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if req.MaxConcurrentTasks < 1 || req.MaxConcurrentTasks > 10 {
		writeError(w, http.StatusBadRequest, "max_concurrent_tasks must be between 1 and 10")
		return
	}

	if req.Provider != "anthropic" && req.Provider != "openai" {
		writeError(w, http.StatusBadRequest, "provider must be 'anthropic' or 'openai'")
		return
	}

	// Generate workspace-specific passphrase for key encryption
	passphrase := workspaceID + string(auth.JWTSecret())

	// Encrypt API key
	encryptedKey, err := crypto.EncryptAPIKey(req.APIKey, passphrase)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encrypt API key")
		return
	}

	apiKeyHash := crypto.HashAPIKey(req.APIKey)

	existing, err := h.Queries.GetCloudRuntimeByWorkspace(r.Context(), parseUUID(workspaceID))
	if err != nil {
		// Create new
		_, err = h.Queries.CreateCloudRuntime(r.Context(), db.CreateCloudRuntimeParams{
			WorkspaceID:         parseUUID(workspaceID),
			GatewayUrl:          ptrToText(req.GatewayURL),
			Provider:            req.Provider,
			EncryptedApiKey:     []byte(encryptedKey),
			ApiKeyHash:          apiKeyHash,
			MaxConcurrentTasks: int32(req.MaxConcurrentTasks),
		})
	} else {
		// Update existing
		_, err = h.Queries.UpdateCloudRuntime(r.Context(), db.UpdateCloudRuntimeParams{
			ID:                  existing.ID,
			GatewayUrl:          ptrToText(req.GatewayURL),
			Provider:            req.Provider,
			EncryptedApiKey:     []byte(encryptedKey),
			ApiKeyHash:          apiKeyHash,
			MaxConcurrentTasks: int32(req.MaxConcurrentTasks),
		})
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save cloud runtime")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetCloudRuntime returns cloud runtime config (without API key)
func (h *Handler) GetCloudRuntime(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	runtime, err := h.Queries.GetCloudRuntimeByWorkspace(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusNotFound, "cloud runtime not configured")
		return
	}

	resp := struct {
		ID                  string  `json:"id"`
		Provider            string  `json:"provider"`
		GatewayURL         *string `json:"gateway_url"`
		IsActive           bool    `json:"is_active"`
		MaxConcurrentTasks int     `json:"max_concurrent_tasks"`
		CreatedAt          string  `json:"created_at"`
	}{
		ID:                  uuidToString(runtime.ID),
		Provider:            runtime.Provider,
		GatewayURL:          textToPtr(runtime.GatewayUrl),
		IsActive:            runtime.IsActive,
		MaxConcurrentTasks: int(runtime.MaxConcurrentTasks),
		CreatedAt:           timestampToString(runtime.CreatedAt),
	}

	writeJSON(w, http.StatusOK, resp)
}

// DeleteCloudRuntime deactivates cloud runtime
func (h *Handler) DeleteCloudRuntime(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	runtime, err := h.Queries.GetCloudRuntimeByWorkspace(r.Context(), parseUUID(workspaceID))
	if err != nil {
		writeError(w, http.StatusNotFound, "cloud runtime not found")
		return
	}

	err = h.Queries.DeleteCloudRuntime(r.Context(), runtime.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete cloud runtime")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ValidateAPIKey tests if the API key is valid
func (h *Handler) ValidateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Provider string `json:"provider"`
		APIKey  string `json:"api_key"`
	}

	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 10*1024)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	// Validate the API key by making a minimal test call
	valid, err := validateProviderKey(req.Provider, req.APIKey)
	if err != nil || !valid {
		writeError(w, http.StatusBadRequest, "invalid API key")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func validateProviderKey(provider, apiKey string) (bool, error) {
	switch provider {
	case "anthropic":
		req, _ := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer([]byte(`{"model":"claude-3-haiku-20240307","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`)))
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		req.Header.Set("content-type", "application/json")

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return false, err
		}
		defer resp.Body.Close()

		return resp.StatusCode == http.StatusOK, nil

	case "openai":
		req, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer([]byte(`{"model":"gpt-4o-mini","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`)))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("content-type", "application/json")

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return false, err
		}
		defer resp.Body.Close()

		return resp.StatusCode == http.StatusOK, nil

	default:
		return false, fmt.Errorf("unknown provider: %s", provider)
	}
}
