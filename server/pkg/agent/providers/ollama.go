package agentproviders

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

)

const ollamaDefaultEndpoint = "http://localhost:11434/api/chat"

// OllamaProvider implements Provider for a local Ollama server.
type OllamaProvider struct {
	apiKey   string
	endpoint string
	logger   *slog.Logger
}

var _ Provider = (*OllamaProvider)(nil)

// NewOllamaProvider creates a new Ollama API provider.
func NewOllamaProvider(cfg APIConfig) *OllamaProvider {
	logger := slog.Default()
	return &OllamaProvider{
		apiKey:   cfg.APIKey,
		endpoint: cfg.Endpoint,
		logger:   logger,
	}
}

// Name returns "ollama".
func (p *OllamaProvider) Name() string {
	return "ollama"
}

// Models returns an empty list since models are dynamic on the Ollama server.
// Use ListModels to discover available models at runtime.
func (p *OllamaProvider) Models() []Model {
	return []Model{
		{Provider: "ollama", Name: "llama3"},
		{Provider: "ollama", Name: "llama3.1"},
		{Provider: "ollama", Name: "mistral"},
		{Provider: "ollama", Name: "codellama"},
		{Provider: "ollama", Name: "phi3"},
	}
}

// Supports returns true if the model provider is ollama.
func (p *OllamaProvider) Supports(model Model) bool {
	return model.Provider == "ollama"
}

// ListModels queries the Ollama server for available models.
func (p *OllamaProvider) ListModels(ctx context.Context) ([]string, error) {
	endpoint := p.endpoint
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama API error: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var ollamaResp ollamaTagsResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return nil, err
	}

	models := make([]string, len(ollamaResp.Models))
	for i, m := range ollamaResp.Models {
		models[i] = m.Name
	}
	return models, nil
}

// Execute runs a prompt via the Ollama Chat API.
func (p *OllamaProvider) Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	model := opts.Model
	if model == "" {
		model = "llama3"
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 20 * time.Minute
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)

	msgCh := make(chan Message, 256)
	resCh := make(chan Result, 1)

	go func() {
		defer cancel()
		defer close(msgCh)
		defer close(resCh)

		startTime := time.Now()

		reqBody := ollamaRequest{
			Model:    model,
			Messages: []ollamaMessage{{Role: "user", Content: prompt}},
			Stream:   false,
		}

		if opts.SystemPrompt != "" {
			reqBody.Messages = append([]ollamaMessage{{Role: "system", Content: opts.SystemPrompt}}, reqBody.Messages...)
		}

		body, err := json.Marshal(reqBody)
		if err != nil {
			resCh <- Result{Status: "failed", Error: fmt.Sprintf("marshal request: %v", err)}
			return
		}

		endpoint := ollamaDefaultEndpoint
		if p.endpoint != "" {
			endpoint = p.endpoint + "/api/chat"
		}

		req, err := http.NewRequestWithContext(runCtx, "POST", endpoint, strings.NewReader(string(body)))
		if err != nil {
			resCh <- Result{Status: "failed", Error: fmt.Sprintf("create request: %v", err)}
			return
		}

		req.Header.Set("content-type", "application/json")
		if p.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+p.apiKey)
		}

		client := &http.Client{Timeout: timeout}
		resp, err := client.Do(req)
		if err != nil {
			resCh <- Result{Status: "failed", Error: fmt.Sprintf("request failed: %v", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resCh <- Result{Status: "failed", Error: fmt.Sprintf("API error %d: %s", resp.StatusCode, truncateErrorBody(bodyBytes))}
			return
		}

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			resCh <- Result{Status: "failed", Error: fmt.Sprintf("read response: %v", err)}
			return
		}

		var ollamaResp ollamaResponse
		if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
			resCh <- Result{Status: "failed", Error: fmt.Sprintf("parse response: %v", err)}
			return
		}

		duration := time.Since(startTime)
		var output strings.Builder
		output.WriteString(ollamaResp.Message.Content)
		trySend(msgCh, Message{Type: MessageText, Content: ollamaResp.Message.Content})

		resCh <- Result{
			Status:     "completed",
			Output:     output.String(),
			DurationMs: duration.Milliseconds(),
			SessionID:  ollamaResp.Model,
		}
	}()

	return &Session{Messages: msgCh, Result: resCh}, nil
}

// StreamExecute streams results via SSE from Ollama.
func (p *OllamaProvider) StreamExecute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	model := opts.Model
	if model == "" {
		model = "llama3"
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 20 * time.Minute
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)

	msgCh := make(chan Message, 256)
	resCh := make(chan Result, 1)

	go func() {
		defer cancel()
		defer close(msgCh)
		defer close(resCh)

		startTime := time.Now()

		reqBody := ollamaRequest{
			Model:    model,
			Messages: []ollamaMessage{{Role: "user", Content: prompt}},
			Stream:   true,
		}

		if opts.SystemPrompt != "" {
			reqBody.Messages = append([]ollamaMessage{{Role: "system", Content: opts.SystemPrompt}}, reqBody.Messages...)
		}

		body, err := json.Marshal(reqBody)
		if err != nil {
			resCh <- Result{Status: "failed", Error: fmt.Sprintf("marshal request: %v", err)}
			return
		}

		endpoint := ollamaDefaultEndpoint
		if p.endpoint != "" {
			endpoint = p.endpoint + "/api/chat"
		}

		req, err := http.NewRequestWithContext(runCtx, "POST", endpoint, strings.NewReader(string(body)))
		if err != nil {
			resCh <- Result{Status: "failed", Error: fmt.Sprintf("create request: %v", err)}
			return
		}

		req.Header.Set("content-type", "application/json")
		if p.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+p.apiKey)
		}

		client := &http.Client{Timeout: timeout}
		resp, err := client.Do(req)
		if err != nil {
			resCh <- Result{Status: "failed", Error: fmt.Sprintf("request failed: %v", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resCh <- Result{Status: "failed", Error: fmt.Sprintf("API error %d: %s", resp.StatusCode, truncateErrorBody(bodyBytes))}
			return
		}

		reader := bufio.NewReader(resp.Body)
		var output strings.Builder

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			var event ollamaStreamEvent
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				continue
			}

			if event.Message.Content != "" {
				output.WriteString(event.Message.Content)
				trySend(msgCh, Message{Type: MessageText, Content: event.Message.Content})
			}
			if event.Done {
				duration := time.Since(startTime)
				resCh <- Result{
					Status:     "completed",
					Output:     output.String(),
					DurationMs: duration.Milliseconds(),
					SessionID:  event.Model,
				}
				return
			}
		}

		duration := time.Since(startTime)
		resCh <- Result{
			Status:     "completed",
			Output:     output.String(),
			DurationMs: duration.Milliseconds(),
		}
	}()

	return &Session{Messages: msgCh, Result: resCh}, nil
}

// Ollama request/response types
type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaResponse struct {
	Model   string `json:"model"`
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

type ollamaStreamEvent struct {
	Model    string `json:"model"`
	Message  struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done bool `json:"done"`
}

type ollamaTagsResponse struct {
	Models []struct {
		Name       string `json:"name"`
		Model      string `json:"model"`
		Size       int64  `json:"size"`
		ModifiedAt string `json:"modified_at"`
	} `json:"models"`
}