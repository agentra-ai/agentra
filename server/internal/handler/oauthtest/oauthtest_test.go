package oauthtest

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/agentra-ai/agentra/server/internal/handler"
)

// fakeExchanger returns a fixed token and records calls. It stands in for
// the real GitHub POST so tests don't need network access.
type fakeExchanger struct {
	token string
	err   error
	calls int
}

func (f *fakeExchanger) exchange(_ string) (string, error) {
	f.calls++
	return f.token, f.err
}

func newHandler(ex func(string) (string, error)) *handler.GitHubOAuthHandler {
	return handler.NewGitHubOAuthHandlerForTest("client-id", "client-secret", "https://example.com/cb", ex)
}

// TestCallback_RedirectDoesNotLeakAccessToken is the regression test for
// the bug where Callback() redirected with ?token=<access_token> in the
// URL, leaking the secret via browser history, Referer, and server logs.
func TestCallback_RedirectDoesNotLeakAccessToken(t *testing.T) {
	ex := &fakeExchanger{token: "gh-secret-access-token-AAAAA"}
	h := newHandler(ex.exchange)

	req := httptest.NewRequest("GET", "/github/oauth/callback?code=authcode", nil)
	w := httptest.NewRecorder()

	h.Callback(w, req)

	if w.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 302, got %d", w.Code)
	}

	loc := w.Header().Get("Location")
	if loc == "" {
		t.Fatal("expected Location header on redirect")
	}

	if strings.Contains(loc, "gh-secret-access-token-AAAAA") {
		t.Fatalf("redirect URL leaks access token: %s", loc)
	}

	parsed, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("invalid Location URL %q: %v", loc, err)
	}
	if parsed.Query().Get("token") != "" {
		t.Fatalf("Location has raw token query param: %s", loc)
	}
	if parsed.Query().Get("code") == "" {
		t.Fatalf("Location missing one-time code param: %s", loc)
	}
}

func TestCallback_MissingCode_Returns400(t *testing.T) {
	ex := &fakeExchanger{token: "any"}
	h := newHandler(ex.exchange)

	req := httptest.NewRequest("GET", "/github/oauth/callback", nil)
	w := httptest.NewRecorder()

	h.Callback(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d (body=%s)", w.Code, w.Body.String())
	}
	if ex.calls != 0 {
		t.Fatalf("exchanger should not be called on missing code, got %d calls", ex.calls)
	}
}

func TestCallback_ExchangerError_Returns500(t *testing.T) {
	ex := &fakeExchanger{err: errors.New("boom")}
	h := newHandler(ex.exchange)

	req := httptest.NewRequest("GET", "/github/oauth/callback?code=authcode", nil)
	w := httptest.NewRecorder()

	h.Callback(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d (body=%s)", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != "" {
		t.Fatalf("must not redirect on exchanger error, got Location=%s", loc)
	}
}

func TestExchange_RoundTrip(t *testing.T) {
	ex := &fakeExchanger{token: "gh-secret-access-token-AAAAA"}
	h := newHandler(ex.exchange)

	cb := httptest.NewRequest("GET", "/github/oauth/callback?code=authcode", nil)
	cbw := httptest.NewRecorder()
	h.Callback(cbw, cb)
	if cbw.Code != http.StatusTemporaryRedirect {
		t.Fatalf("callback: expected 302, got %d", cbw.Code)
	}
	loc, _ := url.Parse(cbw.Header().Get("Location"))
	oneTime := loc.Query().Get("code")
	if oneTime == "" {
		t.Fatal("callback Location missing one-time code")
	}
	if strings.Contains(cbw.Header().Get("Location"), "gh-secret-access-token-AAAAA") {
		t.Fatal("callback Location must not contain raw token")
	}

	exReq := httptest.NewRequest("GET", "/github/oauth/exchange?code="+oneTime, nil)
	exW := httptest.NewRecorder()
	h.Exchange(exW, exReq)

	if exW.Code != http.StatusOK {
		t.Fatalf("exchange: expected 200, got %d (body=%s)", exW.Code, exW.Body.String())
	}
	if !strings.Contains(exW.Body.String(), "gh-secret-access-token-AAAAA") {
		t.Fatalf("exchange response missing token: %s", exW.Body.String())
	}

	exReq2 := httptest.NewRequest("GET", "/github/oauth/exchange?code="+oneTime, nil)
	exW2 := httptest.NewRecorder()
	h.Exchange(exW2, exReq2)
	if exW2.Code != http.StatusNotFound {
		t.Fatalf("second exchange: expected 404, got %d", exW2.Code)
	}
}

func TestExchange_UnknownCode_Returns404(t *testing.T) {
	h := newHandler(func(string) (string, error) { return "", nil })

	req := httptest.NewRequest("GET", "/github/oauth/exchange?code=nope", nil)
	w := httptest.NewRecorder()
	h.Exchange(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestExchange_MissingCode_Returns400(t *testing.T) {
	h := newHandler(func(string) (string, error) { return "", nil })

	req := httptest.NewRequest("GET", "/github/oauth/exchange", nil)
	w := httptest.NewRecorder()
	h.Exchange(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
