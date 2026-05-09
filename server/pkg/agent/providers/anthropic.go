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

const anthropicEndpoint = "https://api.anthropic.com/v1/messages"

// AnthropicProvider implements Provider for the Anthropic API.
type AnthropicProvider struct {
	apiKey string
	logger *slog.Logger
}

var _ Provider = (*AnthropicProvider)(nil)

// NewAnthropicProvider creates a new Anthropic API provider.
func NewAnthropicProvider(cfg APIConfig) *AnthropicProvider {
	logger := slog.Default()
	if cfg.Extra != nil {
		if l, ok := cfg.Extra["logger"]; ok {
			logger = slog.Default()
			_ = l // suppress unused warning
		}
	}
	return &AnthropicProvider{
		apiKey: cfg.APIKey,
		logger: logger,
	}
}

// Name returns "anthropic".
func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

// Models returns the list of supported Anthropic models.
func (p *AnthropicProvider) Models() []Model {
	return []Model{
		{Provider: "anthropic", Name: "claude-3-5-sonnet-20241022"},
		{Provider: "anthropic", Name: "claude-3-5-sonnet-latest"},
		{Provider: "anthropic", Name: "claude-3-opus-20240229"},
		{Provider: "anthropic", Name: "claude-3-opus-latest"},
		{Provider: "anthropic", Name: "claude-3-haiku-20240307"},
		{Provider: "anthropic", Name: "claude-3-haiku-latest"},
		{Provider: "anthropic", Name: "claude-sonnet-4-20250514"},
	}
}

// Supports returns true if the model provider is anthropic.
func (p *AnthropicProvider) Supports(model Model) bool {
	return model.Provider == "anthropic"
}

// Execute runs a prompt via the Anthropic Messages API.
func (p *AnthropicProvider) Execute(ctx context.Context, prompt string, opts ExecOptions) (*agent.Session, error) {
	model := opts.Model
	if model == "" {
		model = "claude-3-5-sonnet-20241022"
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

		reqBody := anthropicRequest{
			Model:         model,
			MaxTokens:     8192,
			SystemPrompt:  opts.SystemPrompt,
			Messages:      []anthropicMessage{{Role: "user", Content: prompt}},
		}

		body, err := json.Marshal(reqBody)
		if err != nil {
			resCh <- agent.Result{Status: "failed", Error: fmt.Sprintf("marshal request: %v", err)}
			return
		}

		endpoint := anthropicEndpoint
		req, err := http.NewRequestWithContext(runCtx, "POST", endpoint, strings.NewReader(string(body)))
		if err != nil {
			resCh <- agent.Result{Status: "failed", Error: fmt.Sprintf("create request: %v", err)}
			return
		}

		req.Header.Set("x-api-key", p.apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		req.Header.Set("content-type", "application/json")

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

		// Parse response from buffered bytes
		var anthropicResp anthropicResponse
		if err := json.Unmarshal(bodyBytes, &anthropicResp); err != nil {
			resCh <- agent.Result{Status: "failed", Error: fmt.Sprintf("parse response: %v", err)}
			return
		}

		duration := time.Since(startTime)
		var output strings.Builder
		for _, block := range anthropicResp.Content {
			if block.Type == "text" {
				output.WriteString(block.Text)
				trySend(msgCh, agent.Message{Type: agent.MessageText, Content: block.Text})
			}
		}

		tokenUsage := &agent.TokenUsage{
			InputTokens:     anthropicResp.Usage.InputTokens,
			OutputTokens:    anthropicResp.Usage.OutputTokens,
			CacheReadTokens: anthropicResp.Usage.CacheRead,
			CacheWriteTokens: anthropicResp.Usage.CacheCreate,
		}

		resCh <- agent.Result{
			Status:     "completed",
			Output:     output.String(),
			DurationMs: duration.Milliseconds(),
			TokenUsage: tokenUsage,
		}
	}()

	return &agent.Session{Messages: msgCh, Result: resCh}, nil
}

// Anthropic request/response types
type anthropicRequest struct {
	Model       string              `json:"model"`
	MaxTokens   int                 `json:"max_tokens"`
	SystemPrompt string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
	} `json:"content"`
	Model       string `json:"model"`
	StopReason  string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence,omitempty"`
	Usage       struct {
		InputTokens  int64 `json:"input_tokens"`
		OutputTokens int64 `json:"output_tokens"`
		CacheRead   int64 `json:"cache_read_input_tokens,omitempty"`
		CacheCreate int64 `json:"cache_creation_input_tokens,omitempty"`
	} `json:"usage"`
}

// StreamExecute runs a prompt and streams results via SSE.
// This is an alternative to Execute that provides real-time streaming.
func (p *AnthropicProvider) StreamExecute(ctx context.Context, prompt string, opts ExecOptions) (*agent.Session, error) {
	model := opts.Model
	if model == "" {
		model = "claude-3-5-sonnet-20241022"
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

		reqBody := anthropicRequest{
			Model:         model,
			MaxTokens:     8192,
			SystemPrompt:  opts.SystemPrompt,
			Messages:      []anthropicMessage{{Role: "user", Content: prompt}},
		}

		body, err := json.Marshal(reqBody)
		if err != nil {
			resCh <- agent.Result{Status: "failed", Error: fmt.Sprintf("marshal request: %v", err)}
			return
		}

		req, err := http.NewRequestWithContext(runCtx, "POST", anthropicEndpoint, strings.NewReader(string(body)))
		if err != nil {
			resCh <- agent.Result{Status: "failed", Error: fmt.Sprintf("create request: %v", err)}
			return
		}

		req.Header.Set("x-api-key", p.apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		req.Header.Set("content-type", "application/json")
		req.Header.Set("anthropic-beta", "interleaved-access-2025-05-14")

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
			if err != err {
				break
			}
			line = strings.TrimSpace(line)
			if line == "" || !strings.HasPrefix(line, "data:") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")

			var event anthropicStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			switch event.Type {
			case "content_block_start":
				if event.ContentBlock.Type == "thinking" {
					trySend(msgCh, agent.Message{Type: agent.MessageStatus, Status: "thinking"})
				}
			case "content_block_delta":
				if event.Delta.Type == "text_delta" {
					output.WriteString(event.Delta.Text)
					trySend(msgCh, agent.Message{Type: agent.MessageText, Content: event.Delta.Text})
				} else if event.Delta.Type == "thinking_delta" {
					trySend(msgCh, agent.Message{Type: agent.MessageThinking, Content: event.Delta.Text})
				}
			case "message_delta":
				if event.Usage != nil {
					// Final usage — already captured in final message
				}
			case "message_stop":
				duration := time.Since(startTime)
				resCh <- agent.Result{
					Status:     "completed",
					Output:     output.String(),
					DurationMs: duration.Milliseconds(),
					SessionID:  sessionID,
				}
				return
			}
		}

		// Fallback: if we exit loop without message_stop, use accumulated output
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

type anthropicStreamEvent struct {
	Type         string `json:"type"`
	Index        int    `json:"index,omitempty"`
	ContentBlock *struct {
		Type string `json:"type"`
	} `json:"content_block,omitempty"`
	Delta *struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
	} `json:"delta,omitempty"`
	Usage *struct {
		InputTokens  int64 `json:"input_tokens,omitempty"`
		OutputTokens int64 `json:"output_tokens,omitempty"`
	} `json:"usage,omitempty"`
}

func trySend(ch chan<- agent.Message, msg agent.Message) {
	select {
	case ch <- msg:
	default:
		// Channel full — drop message
	}
}