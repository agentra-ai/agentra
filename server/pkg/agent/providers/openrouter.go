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

	"github.com/agentra-ai/agentra/server/pkg/agent"
)

const openRouterDefaultEndpoint = "https://openrouter.ai/api/v1/chat/completions"

// OpenRouterProvider implements Provider for the OpenRouter aggregated API.
type OpenRouterProvider struct {
	apiKey   string
	endpoint string
	logger   *slog.Logger
}

var _ Provider = (*OpenRouterProvider)(nil)

// NewOpenRouterProvider creates a new OpenRouter API provider.
func NewOpenRouterProvider(cfg APIConfig) *OpenRouterProvider {
	logger := slog.Default()
	return &OpenRouterProvider{
		apiKey:   cfg.APIKey,
		endpoint: cfg.Endpoint,
		logger:   logger,
	}
}

// Name returns "openrouter".
func (p *OpenRouterProvider) Name() string {
	return "openrouter"
}

// Models returns the list of commonly available OpenRouter models.
// Note: OpenRouter supports many models; this is a subset of popular ones.
func (p *OpenRouterProvider) Models() []Model {
	return []Model{
		{Provider: "openrouter", Name: "anthropic/claude-3.5-sonnet"},
		{Provider: "openrouter", Name: "anthropic/claude-3-opus"},
		{Provider: "openrouter", Name: "openai/gpt-4o"},
		{Provider: "openrouter", Name: "openai/gpt-4o-mini"},
		{Provider: "openrouter", Name: "google/gemini-pro-1.5"},
		{Provider: "openrouter", Name: "meta-llama/llama-3-70b-instruct"},
		{Provider: "openrouter", Name: "mistralai/mistral-large"},
	}
}

// Supports returns true if the model provider is openrouter.
func (p *OpenRouterProvider) Supports(model Model) bool {
	return model.Provider == "openrouter"
}

// Execute runs a prompt via the OpenRouter API.
func (p *OpenRouterProvider) Execute(ctx context.Context, prompt string, opts ExecOptions) (*agent.Session, error) {
	model := opts.Model
	if model == "" {
		model = "anthropic/claude-3.5-sonnet"
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 20 * time.Minute
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)

	msgCh := make(chan agent.Message, 256)
	resCh := make(chan agent.Result, 1)

	go func() {
		defer cancel()
		defer close(msgCh)
		defer close(resCh)

		startTime := time.Now()

		messages := []openaiMessage{{Role: "user", Content: prompt}}
		if opts.SystemPrompt != "" {
			messages = append([]openaiMessage{{Role: "system", Content: opts.SystemPrompt}}, messages...)
		}

		reqBody := openaiRequest{
			Model:    model,
			Messages: messages,
		}

		body, err := json.Marshal(reqBody)
		if err != nil {
			resCh <- agent.Result{Status: "failed", Error: fmt.Sprintf("marshal request: %v", err)}
			return
		}

		endpoint := openRouterDefaultEndpoint
		if p.endpoint != "" {
			endpoint = p.endpoint
		}

		req, err := http.NewRequestWithContext(runCtx, "POST", endpoint, strings.NewReader(string(body)))
		if err != nil {
			resCh <- agent.Result{Status: "failed", Error: fmt.Sprintf("create request: %v", err)}
			return
		}

		req.Header.Set("Authorization", "Bearer "+p.apiKey)
		req.Header.Set("content-type", "application/json")
		req.Header.Set("HTTP-Referer", "https://agentra.ai")
		req.Header.Set("X-Title", "Agentra")

		client := &http.Client{Timeout: timeout}
		resp, err := client.Do(req)
		if err != nil {
			resCh <- agent.Result{Status: "failed", Error: fmt.Sprintf("request failed: %v", err)}
			return
		}
		defer resp.Body.Close()

		// Read body once into buffer, then handle both error and success cases
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			resCh <- agent.Result{Status: "failed", Error: fmt.Sprintf("read response: %v", err)}
			return
		}

		if resp.StatusCode != http.StatusOK {
			resCh <- agent.Result{Status: "failed", Error: fmt.Sprintf("API error %d: %s", resp.StatusCode, string(bodyBytes))}
			return
		}

		var openaiResp openaiResponse
		if err := json.Unmarshal(bodyBytes, &openaiResp); err != nil {
			resCh <- agent.Result{Status: "failed", Error: fmt.Sprintf("parse response: %v", err)}
			return
		}

		duration := time.Since(startTime)
		var output strings.Builder
		for _, choice := range openaiResp.Choices {
			if choice.Message.Content != "" {
				output.WriteString(choice.Message.Content)
				trySend(msgCh, agent.Message{Type: agent.MessageText, Content: choice.Message.Content})
			}
		}

		tokenUsage := &agent.TokenUsage{
			InputTokens:  int64(openaiResp.Usage.PromptTokens),
			OutputTokens: int64(openaiResp.Usage.CompletionTokens),
		}

		resCh <- agent.Result{
			Status:     "completed",
			Output:     output.String(),
			DurationMs: duration.Milliseconds(),
			SessionID:  openaiResp.ID,
			TokenUsage: tokenUsage,
		}
	}()

	return &agent.Session{Messages: msgCh, Result: resCh}, nil
}

// StreamExecute streams results via SSE from OpenRouter.
func (p *OpenRouterProvider) StreamExecute(ctx context.Context, prompt string, opts ExecOptions) (*agent.Session, error) {
	model := opts.Model
	if model == "" {
		model = "anthropic/claude-3.5-sonnet"
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 20 * time.Minute
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)

	msgCh := make(chan agent.Message, 256)
	resCh := make(chan agent.Result, 1)

	go func() {
		defer cancel()
		defer close(msgCh)
		defer close(resCh)

		startTime := time.Now()

		messages := []openaiMessage{{Role: "user", Content: prompt}}
		if opts.SystemPrompt != "" {
			messages = append([]openaiMessage{{Role: "system", Content: opts.SystemPrompt}}, messages...)
		}

		reqBody := openaiRequest{
			Model:    model,
			Messages: messages,
			Stream:   true,
		}

		body, err := json.Marshal(reqBody)
		if err != nil {
			resCh <- agent.Result{Status: "failed", Error: fmt.Sprintf("marshal request: %v", err)}
			return
		}

		endpoint := openRouterDefaultEndpoint
		if p.endpoint != "" {
			endpoint = p.endpoint
		}

		req, err := http.NewRequestWithContext(runCtx, "POST", endpoint, strings.NewReader(string(body)))
		if err != nil {
			resCh <- agent.Result{Status: "failed", Error: fmt.Sprintf("create request: %v", err)}
			return
		}

		req.Header.Set("Authorization", "Bearer "+p.apiKey)
		req.Header.Set("content-type", "application/json")
		req.Header.Set("HTTP-Referer", "https://agentra.ai")
		req.Header.Set("X-Title", "Agentra")

		client := &http.Client{Timeout: timeout}
		resp, err := client.Do(req)
		if err != nil {
			resCh <- agent.Result{Status: "failed", Error: fmt.Sprintf("request failed: %v", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resCh <- agent.Result{Status: "failed", Error: fmt.Sprintf("API error %d: %s", resp.StatusCode, string(bodyBytes))}
			return
		}

		reader := bufio.NewReader(resp.Body)
		var output strings.Builder
		var sessionID string

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if !strings.HasPrefix(line, "data:") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			var event openaiStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			for _, choice := range event.Choices {
				if choice.Delta.Content != "" {
					output.WriteString(choice.Delta.Content)
					trySend(msgCh, agent.Message{Type: agent.MessageText, Content: choice.Delta.Content})
				}
			}
			if event.ID != "" {
				sessionID = event.ID
			}
		}

		duration := time.Since(startTime)
		resCh <- agent.Result{
			Status:     "completed",
			Output:     output.String(),
			DurationMs: duration.Milliseconds(),
			SessionID:  sessionID,
		}
	}()

	return &agent.Session{Messages: msgCh, Result: resCh}, nil
}