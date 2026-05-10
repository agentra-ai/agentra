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

const togetherDefaultEndpoint = "https://api.together.xyz/v1/chat/completions"

// TogetherProvider implements Provider for the Together AI API.
type TogetherProvider struct {
	apiKey   string
	endpoint string
	logger   *slog.Logger
}

var _ Provider = (*TogetherProvider)(nil)

// NewTogetherProvider creates a new Together AI provider.
func NewTogetherProvider(cfg APIConfig) *TogetherProvider {
	logger := slog.Default()
	return &TogetherProvider{
		apiKey:   cfg.APIKey,
		endpoint: cfg.Endpoint,
		logger:   logger,
	}
}

// Name returns "together".
func (p *TogetherProvider) Name() string {
	return "together"
}

// Models returns the list of available Together AI models.
func (p *TogetherProvider) Models() []Model {
	return []Model{
		{Provider: "together", Name: "mistralai/Mistral-7B-Instruct-v0.2"},
		{Provider: "together", Name: "meta-llama/Llama-3-70b-chat-hf"},
		{Provider: "together", Name: "meta-llama/Llama-3-8b-chat-hf"},
		{Provider: "together", Name: "mistralai/Mixtral-8x7B-Instruct-v0.1"},
		{Provider: "together", Name: "deepseek-ai/DeepSeek-V3"},
	}
}

// Supports returns true if the model provider is together.
func (p *TogetherProvider) Supports(model Model) bool {
	return model.Provider == "together"
}

// Execute runs a prompt via the Together AI API.
func (p *TogetherProvider) Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	model := opts.Model
	if model == "" {
		model = "mistralai/Mixtral-8x7B-Instruct-v0.1"
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
			endpoint = togetherDefaultEndpoint
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
		var lastFinishReason string

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
					Delta        map[string]any `json:"delta"`
					FinishReason string          `json:"finish_reason"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(line), &chunk); err != nil {
				continue
			}

			for _, choice := range chunk.Choices {
				if choice.Delta["content"] != nil {
					content := fmt.Sprintf("%v", choice.Delta["content"])
					fullOutput.WriteString(content)
					msgCh <- Message{Type: MessageText, Content: content}
				}
				if choice.FinishReason != "" {
					lastFinishReason = choice.FinishReason
				}
			}
		}

		status := "completed"
		if lastFinishReason == "length" {
			status = "failed"
		}

		resCh <- Result{
			Status: status,
			Output: fullOutput.String(),
		}
	}()

	return &Session{Messages: msgCh, Result: resCh}, nil
}

// StreamExecute runs a streaming prompt via the Together AI API.
func (p *TogetherProvider) StreamExecute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	return p.Execute(ctx, prompt, opts)
}