package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/agentra-ai/agentra/server/internal/auth"
	"github.com/agentra-ai/agentra/server/internal/util"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func uuidToString(u pgtype.UUID) string { return util.UUIDToString(u) }

// PATStore is the minimal interface the auth middleware needs to validate
// and update personal access tokens. It is the seam that lets tests
// substitute a fake without a database. Production code passes a
// *db.Queries which implements this interface.
type PATStore interface {
	GetPersonalAccessTokenByHash(ctx context.Context, hash string) (db.PersonalAccessToken, error)
	UpdatePersonalAccessTokenLastUsed(ctx context.Context, id pgtype.UUID) error
}

// Auth middleware validates JWT tokens or Personal Access Tokens from the Authorization header.
// Sets X-User-ID and X-User-Email headers on the request for downstream handlers.
func Auth(queries *db.Queries) func(http.Handler) http.Handler {
	return AuthWithPATStore(queries)
}

// AuthWithPATStore is the testable form of Auth. Production callers should
// use Auth(*db.Queries); tests pass a fake PATStore.
func AuthWithPATStore(store PATStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				slog.Debug("auth: missing authorization header", "path", r.URL.Path)
				http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
				return
			}

			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == authHeader {
				slog.Debug("auth: invalid format", "path", r.URL.Path)
				http.Error(w, `{"error":"invalid authorization format"}`, http.StatusUnauthorized)
				return
			}

			// PAT: tokens starting with "mul_"
			if strings.HasPrefix(tokenString, "mul_") {
				if store == nil {
					http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
					return
				}
				hash := auth.HashToken(tokenString)
				pat, err := store.GetPersonalAccessTokenByHash(r.Context(), hash)
				if err != nil {
					slog.Warn("auth: invalid PAT", "path", r.URL.Path, "error", err)
					http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
					return
				}

				r.Header.Set("X-User-ID", uuidToString(pat.UserID))

				// Update last_used_at in a short-lived detached context so
				// request cancellation does not abandon the write, but with
				// a hard timeout. Do NOT spawn an unbounded goroutine: a
				// flood of PAT requests would otherwise grow the goroutine
				// count without bound and starve the server.
				updateCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				_ = store.UpdatePersonalAccessTokenLastUsed(updateCtx, pat.ID)

				next.ServeHTTP(w, r)
				return
			}

			// JWT
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return auth.JWTSecret(), nil
			})
			if err != nil || !token.Valid {
				slog.Warn("auth: invalid token", "path", r.URL.Path, "error", err)
				http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				slog.Warn("auth: invalid claims", "path", r.URL.Path)
				http.Error(w, `{"error":"invalid claims"}`, http.StatusUnauthorized)
				return
			}

			sub, ok := claims["sub"].(string)
			if !ok || strings.TrimSpace(sub) == "" {
				slog.Warn("auth: invalid claims", "path", r.URL.Path)
				http.Error(w, `{"error":"invalid claims"}`, http.StatusUnauthorized)
				return
			}
			r.Header.Set("X-User-ID", sub)
			if email, ok := claims["email"].(string); ok {
				r.Header.Set("X-User-Email", email)
			}

			next.ServeHTTP(w, r)
		})
	}
}
