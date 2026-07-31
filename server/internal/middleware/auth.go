package middleware

import (
	"context"
	"errors"
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

// UserIdentity is the authenticated user information shared by HTTP and
// WebSocket entry points.
type UserIdentity struct {
	UserID string
	Email  string
}

// AuthenticateUserToken validates either a personal access token or JWT.
// PAT last-used attribution is updated inline with a bounded detached context,
// matching the HTTP middleware's audit semantics without leaking goroutines.
func AuthenticateUserToken(ctx context.Context, store PATStore, tokenString string) (UserIdentity, error) {
	if strings.HasPrefix(tokenString, "mul_") {
		if store == nil {
			return UserIdentity{}, errors.New("invalid token")
		}
		pat, err := store.GetPersonalAccessTokenByHash(ctx, auth.HashToken(tokenString))
		if err != nil {
			return UserIdentity{}, errors.New("invalid token")
		}

		updateCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = store.UpdatePersonalAccessTokenLastUsed(updateCtx, pat.ID)

		userID := uuidToString(pat.UserID)
		if strings.TrimSpace(userID) == "" {
			return UserIdentity{}, errors.New("invalid token")
		}
		return UserIdentity{UserID: userID}, nil
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return auth.JWTSecret(), nil
	})
	if err != nil || !token.Valid {
		return UserIdentity{}, errors.New("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return UserIdentity{}, errors.New("invalid claims")
	}
	userID, ok := claims["sub"].(string)
	if !ok || strings.TrimSpace(userID) == "" {
		return UserIdentity{}, errors.New("invalid claims")
	}
	identity := UserIdentity{UserID: userID}
	if email, ok := claims["email"].(string); ok {
		identity.Email = email
	}
	return identity, nil
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

			identity, err := AuthenticateUserToken(r.Context(), store, tokenString)
			if err != nil {
				slog.Warn("auth: invalid token", "path", r.URL.Path, "error", err)
				http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
				return
			}
			r.Header.Set("X-User-ID", identity.UserID)
			if identity.Email != "" {
				r.Header.Set("X-User-Email", identity.Email)
			}

			next.ServeHTTP(w, r)
		})
	}
}
