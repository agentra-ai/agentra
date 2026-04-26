package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

// Proxy handles API key injection and request logging
type Proxy struct {
	apiKey    string
	provider  string
	logChan   chan<- RequestLog
	transport *http.Transport
}

type RequestLog struct {
	Provider   string  `json:"provider"`
	Method    string  `json:"method"`
	Endpoint   string  `json:"endpoint"`
	StatusCode int    `json:"status_code"`
	TokensIn   int     `json:"input_tokens,omitempty"`
	TokensOut  int     `json:"output_tokens,omitempty"`
	CostUSD    float64 `json:"cost_usd"`
	Timestamp  string  `json:"timestamp"`
}

func NewProxy(apiKey, provider string, logChan chan<- RequestLog) *Proxy {
	return &Proxy{
		apiKey:   apiKey,
		provider: provider,
		logChan:  logChan,
		transport: &http.Transport{
			MaxIdleConns:       100,
			IdleConnTimeout:    90 * time.Second,
			DisableKeepAlives:   false,
		},
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Inject API key based on provider
	switch p.provider {
	case "anthropic":
		r.Header.Set("x-api-key", p.apiKey)
		r.Header.Set("anthropic-version", "2023-06-01")
	case "openai":
		r.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	target := p.getTargetURL(r.URL.Path)
	proxy := &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			r.URL.Host = target.Host
			r.URL.Scheme = target.Scheme
			r.Host = target.Host
		},
		Transport: p.transport,
	}

	// Capture response
	recorder := &responseRecorder{ResponseWriter: w}
	proxy.ServeHTTP(recorder, r)

	// Log the request
	p.logRequest(r, recorder.StatusCode, recorder.Body())
}

func (p *Proxy) getTargetURL(path string) *url.URL {
	switch {
	case strings.HasPrefix(path, "/v1/messages"):
		return &url.URL{Host: "api.anthropic.com:443", Scheme: "https"}
	case strings.HasPrefix(path, "/v1/chat/completions"):
		return &url.URL{Host: "api.openai.com:443", Scheme: "https"}
	}
	return &url.URL{Host: "api.anthropic.com:443", Scheme: "https"}
}

func (p *Proxy) logRequest(r *http.Request, statusCode int, body []byte) {
	log := RequestLog{
		Provider:   p.provider,
		Method:    r.Method,
		Endpoint:   r.URL.Path,
		StatusCode: statusCode,
		Timestamp:  time.Now().Format(time.RFC3339),
	}

	// Try to parse tokens from response
	if strings.HasPrefix(r.URL.Path, "/v1/messages") {
		if usage := parseAnthropicUsage(body); usage != nil {
			log.TokensIn = usage.InputTokens
			log.TokensOut = usage.OutputTokens
		}
	}

	log.CostUSD = calculateCost(p.provider, log.TokensIn, log.TokensOut)

	select {
	case p.logChan <- log:
	default:
	}
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicResponse struct {
	Usage anthropicUsage `json:"usage"`
}

func parseAnthropicUsage(body []byte) *anthropicUsage {
	var resp anthropicResponse
	if err := json.Unmarshal(body, &resp); err == nil {
		return &resp.Usage
	}
	return nil
}

func calculateCost(provider string, in, out int) float64 {
	switch provider {
	case "anthropic":
		return float64(in)*0.003/1000 + float64(out)*0.015/1000
	case "openai":
		return float64(in)*0.005/1000 + float64(out)*0.015/1000
	}
	return 0
}

type responseRecorder struct {
	http.ResponseWriter
	StatusCode int
	Body_     []byte
}

func (r *responseRecorder) WriteHeader(code int) {
	r.StatusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.Body_ = append(r.Body_, b...)
	return r.ResponseWriter.Write(b)
}

func (r *responseRecorder) Body() []byte {
	return r.Body_
}