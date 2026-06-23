package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/agentra-ai/agentra/server/internal/onetimecode"
	"github.com/go-chi/chi/v5"
)

// exchangeFunc is the minimal interface Callback needs to turn a GitHub
// authorization code into an access token. Injected so tests don't need
// network access.
type exchangeFunc func(code string) (string, error)

// GitHubOAuthHandler handles GitHub App OAuth flow.
//
// The handler previously redirected to the frontend with the access token
// in the URL query string, leaking the token via browser history, Referer
// headers, and server access logs. It now stores the token server-side
// under a one-time code (see internal/onetimecode) and redirects with
// only that code; the frontend calls Exchange() to retrieve the token
// over an authenticated request.
type GitHubOAuthHandler struct {
	clientID     string
	clientSecret string
	redirectURL  string
	exchange     exchangeFunc
	codes        *onetimecode.Store
}

// NewGitHubOAuthHandler creates a new GitHub OAuth handler.
func NewGitHubOAuthHandler(clientID, clientSecret, redirectURL string) *GitHubOAuthHandler {
	h := &GitHubOAuthHandler{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
		codes:        onetimecode.New(5 * time.Minute),
	}
	h.exchange = h.exchangeCode
	return h
}

// NewGitHubOAuthHandlerForTest is the test-only constructor that injects
// a fake exchanger and a fresh code store. Production code should use
// NewGitHubOAuthHandler.
func NewGitHubOAuthHandlerForTest(clientID, clientSecret, redirectURL string, ex exchangeFunc) *GitHubOAuthHandler {
	return &GitHubOAuthHandler{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
		exchange:     ex,
		codes:        onetimecode.New(5 * time.Minute),
	}
}

// RegisterRoutes registers the OAuth routes.
func (h *GitHubOAuthHandler) RegisterRoutes(r chi.Router) {
	r.Get("/github/oauth/authorize", h.Authorize)
	r.Get("/github/oauth/callback", h.Callback)
	r.Get("/github/oauth/exchange", h.Exchange)
}

// Authorize redirects the user to GitHub's OAuth authorization page.
func (h *GitHubOAuthHandler) Authorize(w http.ResponseWriter, r *http.Request) {
	params := url.Values{}
	params.Set("client_id", h.clientID)
	params.Set("scope", "repo,workflow")

	redirectURL := "https://github.com/login/oauth/authorize?" + params.Encode()
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

// Callback handles the OAuth callback from GitHub. It exchanges the auth
// code for an access token, stores the token under a one-time code, then
// redirects to the frontend with only the code. The token itself is
// never put on the wire via redirect.
func (h *GitHubOAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	token, err := h.exchange(code)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to exchange code: %v", err), http.StatusInternalServerError)
		return
	}

	oneTimeCode := h.codes.Put(token)

	http.Redirect(w, r, "/settings/github?connected=true&code="+url.QueryEscape(oneTimeCode), http.StatusTemporaryRedirect)
}

// Exchange retrieves and removes a one-time code previously stored by
// Callback. It returns 404 if the code is unknown, expired, or already
// consumed. This is the endpoint the frontend calls to redeem the code
// for the access token.
func (h *GitHubOAuthHandler) Exchange(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "missing code")
		return
	}
	token, ok := h.codes.Take(code)
	if !ok {
		writeError(w, http.StatusNotFound, "code not found or already used")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

// exchangeCode exchanges an OAuth code for an access token.
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
