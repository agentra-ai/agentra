// Package corsconfig resolves the CORS allowed-origins list from environment
// variables. It is split out of cmd/server so the resolution logic can be
// unit-tested without booting a database (cmd/server has integration tests
// that os.Exit(0) when no DB is reachable, which would block unit tests).
package corsconfig

import (
	"os"
	"strings"
)

// AllowedOrigins returns the list of origins the server should allow for
// CORS requests, or nil if no origin is configured. Callers MUST treat a
// nil return as "do not enable CORS at all" — passing an empty slice to
// go-chi/cors results in allowedOriginsAll = true (all origins allowed),
// which is a security footgun.
//
// Resolution order (first non-empty wins):
//  1. CORS_ALLOWED_ORIGINS — comma-separated list
//  2. FRONTEND_ORIGIN      — single origin
//  3. NEXT_PUBLIC_SITE_URL — single origin (most specific site URL)
//  4. AGENTRA_APP_URL      — public app URL
func AllowedOrigins() []string {
	raw := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("FRONTEND_ORIGIN"))
	}
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("NEXT_PUBLIC_SITE_URL"))
	}
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("AGENTRA_APP_URL"))
	}
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin != "" {
			origins = append(origins, origin)
		}
	}
	if len(origins) == 0 {
		return nil
	}
	return origins
}
