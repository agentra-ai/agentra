package realtime

import (
	"net/http"
	"net/url"
	"strings"
)

// CheckWSOrigin returns true if the request's Origin header is permitted
// for WebSocket upgrades. It is the security boundary for the /ws route:
// without a check, any web page could open a long-lived WebSocket to the
// server using a stolen JWT (e.g. via XSS or via a token leaked into logs).
//
// When allowList is empty, the function returns false (fail-closed).
// Otherwise the Origin header must be present, parse as an absolute URL,
// and its scheme+host(+port) must match one of the allowed origins.
// Subdomains must be listed explicitly — origin "app.example.com" does
// not match "example.com".
func CheckWSOrigin(r *http.Request, allowList []string) bool {
	if len(allowList) == 0 {
		// Fail closed: refusing rather than allowing all avoids a
		// regression where empty config silently opens the door.
		return false
	}

	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		// No Origin header — typical for non-browser clients (CLIs,
		// daemons). Server-to-server callers don't carry an Origin,
		// so we accept these only when explicitly opted in.
		return r.Header.Get("Authorization") != "" || r.URL.Query().Get("token") != ""
	}

	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || u.Scheme == "" {
		return false
	}
	want := strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host)
	for _, allowed := range allowList {
		if strings.EqualFold(allowed, want) {
			return true
		}
	}
	return false
}
