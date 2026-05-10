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

const openAIDefaultEndpoint = "https://api.openai.com/v1/chat/completions"

// OpenAIProvider implements Provider for the OpenAI API.
type OpenAIProvider struct {
	apiKey   string
	endpoint string
	logger   *slog.Logger
}

var _ Provider = (*OpenAIProvider)(nil)

// NewOpenAIProvider creates a new OpenAI API provider.
func NewOpenAIProvider(cfg APIConfig) *OpenAIProvider {
	logger := slog.Default()
	return &OpenAIProvider{
		apiKey:   cfg.APIKey,
		endpoint: cfg.Endpoint,
		logger:   logger,
	}
}

// Name returns "openai".
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// Models returns the list of supported OpenAI models.
func (p *OpenAIProvider) Models() []Model {
	return []Model{
		{Provider: "openai", Name: "gpt-4o"},
		{Provider: "openai", Name: "gpt-4o-mini"},
		{Provider: "openai", Name: "gpt-4-turbo"},
		{Provider: "openai", Name: "gpt-4"},
		{Provider: "openai", Name: "gpt-3.5-turbo"},
	}
}

// Supports returns true if the model is a known OpenAI model.
func (p *OpenAIProvider) Supports(model Model) bool {
	if model.Provider != "openai" {
		return false
	}
	for _, m := range p.Models() {
		if m.Name == model.Name {
			return true
		}
	}
	// Allow arbitrary OpenAI model names (custom fine-tuned models)
	return true
}

// Execute runs a prompt via the OpenAI Chat Completions API.
func (p *OpenAIProvider) Execute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	model := opts.Model
	if model == "" {
		model = "gpt-4o-mini"
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
			resCh <- Result{Status: "failed", Error: fmt.Sprintf("marshal request: %v", err)}
			return
		}

		endpoint := openAIDefaultEndpoint
		if p.endpoint != "" {
			endpoint = p.endpoint
		}

		req, err := http.NewRequestWithContext(runCtx, "POST", endpoint, strings.NewReader(string(body)))
		if err != nil {
			resCh <- Result{Status: "failed", Error: fmt.Sprintf("create request: %v", err)}
			return
		}

		req.Header.Set("Authorization", "Bearer "+p.apiKey)
		req.Header.Set("content-type", "application/json")

		client := &http.Client{Timeout: timeout}
		resp, err := client.Do(req)
		if err != nil {
			resCh <- Result{Status: "failed", Error: fmt.Sprintf("request failed: %v", err)}
			return
		}
		defer resp.Body.Close()

		// Read body once into buffer, then handle both error and success cases
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			resCh <- Result{Status: "failed", Error: fmt.Sprintf("read response: %v", err)}
			return
		}

		if resp.StatusCode != http.StatusOK {
			resCh <- Result{Status: "failed", Error: fmt.Sprintf("API error %d: %s", resp.StatusCode, truncateErrorBody(bodyBytes))}
			return
		}

		var openaiResp openaiResponse
		if err := json.Unmarshal(bodyBytes, &openaiResp); err != nil {
			resCh <- Result{Status: "failed", Error: fmt.Sprintf("parse response: %v", err)}
			return
		}

		duration := time.Since(startTime)
		var output strings.Builder
		for _, choice := range openaiResp.Choices {
			if choice.Message.Content != "" {
				output.WriteString(choice.Message.Content)
				trySend(msgCh, Message{Type: MessageText, Content: choice.Message.Content})
			}
		}

		tokenUsage := &TokenUsage{
			InputTokens:  int64(openaiResp.Usage.PromptTokens),
			OutputTokens: int64(openaiResp.Usage.CompletionTokens),
		}

		resCh <- Result{
			Status:     "completed",
			Output:     output.String(),
			DurationMs: duration.Milliseconds(),
			SessionID:  openaiResp.ID,
			TokenUsage: tokenUsage,
		}
	}()

	return &Session{Messages: msgCh, Result: resCh}, nil
}

// OpenAI request/response types
type openaiRequest struct {
	Model    string          `json:"model"`
	Messages []openaiMessage `json:"messages"`
	Stream   bool            `json:"stream,omitempty"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int           `json:"index"`
		Message      openaiMessage `json:"message"`
		FinishReason string        `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// StreamExecute streams results via SSE from OpenAI.
func (p *OpenAIProvider) StreamExecute(ctx context.Context, prompt string, opts ExecOptions) (*Session, error) {
	model := opts.Model
	if model == "" {
		model = "gpt-4o-mini"
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
			resCh <- Result{Status: "failed", Error: fmt.Sprintf("marshal request: %v", err)}
			return
		}

		endpoint := openAIDefaultEndpoint
		if p.endpoint != "" {
			endpoint = p.endpoint
		}

		req, err := http.NewRequestWithContext(runCtx, "POST", endpoint, strings.NewReader(string(body)))
		if err != nil {
			resCh <- Result{Status: "failed", Error: fmt.Sprintf("create request: %v", err)}
			return
		}

		req.Header.Set("Authorization", "Bearer "+p.apiKey)
		req.Header.Set("content-type", "application/json")

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
					trySend(msgCh, Message{Type: MessageText, Content: choice.Delta.Content})
				}
			}
			if event.ID != "" {
				sessionID = event.ID
			}
		}

		duration := time.Since(startTime)
		resCh <- Result{
			Status:     "completed",
			Output:     output.String(),
			DurationMs: duration.Milliseconds(),
			SessionID:  sessionID,
		}
	}()

	return &Session{Messages: msgCh, Result: resCh}, nil
}

type openaiStreamEvent struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}