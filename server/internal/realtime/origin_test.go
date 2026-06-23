package realtime

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func reqWith(origin, headerAuth, queryToken string) *http.Request {
	r := httptest.NewRequest("GET", "/ws", nil)
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	if headerAuth != "" {
		r.Header.Set("Authorization", "Bearer "+headerAuth)
	}
	if queryToken != "" {
		q := r.URL.Query()
		q.Set("token", queryToken)
		r.URL.RawQuery = q.Encode()
	}
	return r
}

func TestCheckWSOrigin_EmptyAllowList_FailsClosed(t *testing.T) {
	r := reqWith("https://app.example.com", "", "")
	if CheckWSOrigin(r, nil) {
		t.Fatal("empty allow list must reject all origins")
	}
	if CheckWSOrigin(r, []string{}) {
		t.Fatal("empty allow list must reject all origins")
	}
}

func TestCheckWSOrigin_AllowedOrigin(t *testing.T) {
	r := reqWith("https://app.example.com", "", "")
	if !CheckWSOrigin(r, []string{"https://app.example.com"}) {
		t.Fatal("exact origin should be allowed")
	}
}

func TestCheckWSOrigin_AllowedOriginCaseInsensitive(t *testing.T) {
	r := reqWith("HTTPS://APP.example.com", "", "")
	if !CheckWSOrigin(r, []string{"https://app.example.com"}) {
		t.Fatal("origin match should be case-insensitive")
	}
}

func TestCheckWSOrigin_RejectsSubdomainSpoof(t *testing.T) {
	r := reqWith("https://evil.app.example.com", "", "")
	if CheckWSOrigin(r, []string{"https://app.example.com"}) {
		t.Fatal("subdomain must not match parent host")
	}
}

func TestCheckWSOrigin_RejectsDifferentScheme(t *testing.T) {
	r := reqWith("http://app.example.com", "", "")
	if CheckWSOrigin(r, []string{"https://app.example.com"}) {
		t.Fatal("http must not match https")
	}
}

func TestCheckWSOrigin_RejectsDifferentPort(t *testing.T) {
	r := reqWith("https://app.example.com:8443", "", "")
	if CheckWSOrigin(r, []string{"https://app.example.com"}) {
		t.Fatal("different port must not match")
	}
}

func TestCheckWSOrigin_MultipleAllowed(t *testing.T) {
	allow := []string{
		"https://app.example.com",
		"http://localhost:3000",
	}
	cases := []struct {
		origin string
		want   bool
	}{
		{"https://app.example.com", true},
		{"http://localhost:3000", true},
		{"https://app.example.com:443", false},
		{"https://attacker.com", false},
	}
	for _, c := range cases {
		t.Run(c.origin, func(t *testing.T) {
			r := reqWith(c.origin, "", "")
			got := CheckWSOrigin(r, allow)
			if got != c.want {
				t.Fatalf("origin=%q want=%v got=%v", c.origin, c.want, got)
			}
		})
	}
}

func TestCheckWSOrigin_NoOriginHeader_NonBrowserClient(t *testing.T) {
	r := reqWith("", "", "jwt-token-here")
	if !CheckWSOrigin(r, []string{"https://app.example.com"}) {
		t.Fatal("server-to-server clients without Origin should be allowed when authenticated")
	}
}

func TestCheckWSOrigin_NoOriginHeader_NoAuth_Rejects(t *testing.T) {
	r := reqWith("", "", "")
	if CheckWSOrigin(r, []string{"https://app.example.com"}) {
		t.Fatal("browser-shaped request without Origin must be rejected")
	}
}

func TestCheckWSOrigin_MalformedOrigin(t *testing.T) {
	r := reqWith("not-a-url", "", "")
	if CheckWSOrigin(r, []string{"https://app.example.com"}) {
		t.Fatal("malformed Origin should be rejected")
	}
}
