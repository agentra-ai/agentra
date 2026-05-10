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

const mistralDefaultEndpoint = "https://api.mistral.ai/v1/chat/completions"

// MistralProvider implements Provider for the Mistral AI API.
type MistralProvider struct {
	apiKey   string
	endpoint string
	logger   *slog.Logger
}

var _ Provider = (*MistralProvider)(nil)

// NewMistralProvider creates a new Mistral AI provider.
func NewMistralProvider(cfg APIConfig) *MistralProvider {
	logger := slog.Default()
	return &MistralProvider{
		apiKey:   cfg.APIKey,
		endpoint: cfg.Endpoint,
		logger:   logger,
	}
}

// Name returns "mistral".
func (p *MistralProvider) Name() string {
	return "mistral"
}

// Models returns the list of available Mistral models.
func (p *MistralProvider) Models() []Model {
	return []Model{
		{Provider: "mistral", Name: "mistral-large-latest"},
		{Provider: "mistral", Name: "mistral-medium-latest"},
		{Provider: "mistral", Name: "mistral-small-latest"},
		{Provider: "mistral", Name: "codestral-latest"},
	}
}

// Supports returns true if the model provider is mistral.
func (p *MistralProvider) Supports(model Model) bool {
	return model.Provider == "mistral"
}

// Execute runs a prompt via the Mistral AI API.
func (p *MistralProvider) Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	model := opts.Model
	if model == "" {
		model = "mistral-large-latest"
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

		endpoint := p.endpoint
		if endpoint == "" {
			endpoint = mistralDefaultEndpoint
		}

		reqBody := map[string]any{
			"model": model,
			"messages": []map[string]any{
				{"role": "user", "content": prompt},
			},
			"max_tokens": 4096,
			"stream":     true,
		}

		reqBytes, _ := json.Marshal(reqBody)
		req, err := http.NewRequestWithContext(runCtx, "POST", endpoint, strings.NewReader(string(reqBytes)))
		if err != nil {
			resCh <- Result{Status: "failed", Error: fmt.Sprintf("create request: %v", err)}
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+p.apiKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			resCh <- Result{Status: "failed", Error: fmt.Sprintf("request failed: %v", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resCh <- Result{Status: "failed", Error: fmt.Sprintf("status %d: %s", resp.StatusCode, string(body))}
			return
		}

		reader := bufio.NewReader(resp.Body)
		var fullOutput strings.Builder

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				resCh <- Result{Status: "failed", Error: fmt.Sprintf("read error: %v", err)}
				return
			}

			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			line = strings.TrimPrefix(line, "data: ")
			if line == "[DONE]" {
				break
			}

			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(line), &chunk); err != nil {
				continue
			}

			for _, choice := range chunk.Choices {
				if choice.Delta.Content != "" {
					fullOutput.WriteString(choice.Delta.Content)
					msgCh <- Message{Type: MessageText, Content: choice.Delta.Content}
				}
			}
		}

		resCh <- Result{
			Status: "completed",
			Output: fullOutput.String(),
		}
	}()

	return &Session{Messages: msgCh, Result: resCh}, nil
}

// StreamExecute runs a streaming prompt via the Mistral AI API.
func (p *MistralProvider) StreamExecute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	return p.Execute(ctx, prompt, opts)
}