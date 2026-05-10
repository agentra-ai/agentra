package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
)

// GitHubOAuthHandler handles GitHub App OAuth flow
type GitHubOAuthHandler struct {
	clientID     string
	clientSecret string
	redirectURL  string
}

// NewGitHubOAuthHandler creates a new GitHub OAuth handler
func NewGitHubOAuthHandler(clientID, clientSecret, redirectURL string) *GitHubOAuthHandler {
	return &GitHubOAuthHandler{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
	}
}

// RegisterRoutes registers the OAuth routes
func (h *GitHubOAuthHandler) RegisterRoutes(r chi.Router) {
	r.Get("/github/oauth/authorize", h.Authorize)
	r.Get("/github/oauth/callback", h.Callback)
}

// Authorize redirects the user to GitHub's OAuth authorization page
func (h *GitHubOAuthHandler) Authorize(w http.ResponseWriter, r *http.Request) {
	params := url.Values{}
	params.Set("client_id", h.clientID)
	params.Set("scope", "repo,workflow")

	redirectURL := "https://github.com/login/oauth/authorize?" + params.Encode()
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

// Callback handles the OAuth callback from GitHub
func (h *GitHubOAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	// Exchange code for access token
	token, err := h.exchangeCode(code)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to exchange code: %v", err), http.StatusInternalServerError)
		return
	}

	// Redirect to settings page with token
	redirectURL := "/settings/github?connected=true&token=" + url.QueryEscape(token)
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

// exchangeCode exchanges an OAuth code for an access token
func (h *GitHubOAuthHandler) exchangeCode(code string) (string, error) {
	params := url.Values{}
	params.Set("client_id", h.clientID)
	params.Set("client_secret", h.clientSecret)
	params.Set("code", code)

	resp, err := http.PostForm("https://github.com/login/oauth/access_token", params)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Error       string `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Error != "" {
		return "", fmt.Errorf("github error: %s", result.Error)
	}

	return result.AccessToken, nil
}
