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

const deepseekDefaultEndpoint = "https://api.deepseek.com/v1/chat/completions"

// DeepSeekProvider implements Provider for the DeepSeek API.
type DeepSeekProvider struct {
	apiKey   string
	endpoint string
	logger   *slog.Logger
}

var _ Provider = (*DeepSeekProvider)(nil)

// NewDeepSeekProvider creates a new DeepSeek API provider.
func NewDeepSeekProvider(cfg APIConfig) *DeepSeekProvider {
	logger := slog.Default()
	return &DeepSeekProvider{
		apiKey:   cfg.APIKey,
		endpoint: cfg.Endpoint,
		logger:   logger,
	}
}

// Name returns "deepseek".
func (p *DeepSeekProvider) Name() string {
	return "deepseek"
}

// Models returns the list of available DeepSeek models.
func (p *DeepSeekProvider) Models() []Model {
	return []Model{
		{Provider: "deepseek", Name: "deepseek-chat"},
		{Provider: "deepseek", Name: "deepseek-coder"},
	}
}

// Supports returns true if the model provider is deepseek.
func (p *DeepSeekProvider) Supports(model Model) bool {
	return model.Provider == "deepseek"
}

// Execute runs a prompt via the DeepSeek API.
func (p *DeepSeekProvider) Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	model := opts.Model
	if model == "" {
		model = "deepseek-chat"
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
			endpoint = deepseekDefaultEndpoint
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

// StreamExecute runs a streaming prompt via the DeepSeek API.
func (p *DeepSeekProvider) StreamExecute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	return p.Execute(ctx, prompt, opts)
}