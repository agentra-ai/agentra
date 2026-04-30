package handlerutil

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/agentra-ai/agentra/server/internal/middleware"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

func RequestUserID(r *http.Request) string {
	return r.Header.Get("X-User-ID")
}

func RequireUserID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := RequestUserID(r)
	if id == "" {
		WriteError(w, http.StatusUnauthorized, "user not authenticated")
		return "", false
	}
	return id, true
}

func ResolveWorkspaceID(r *http.Request) string {
	if id := middleware.WorkspaceIDFromContext(r.Context()); id != "" {
		return id
	}
	if id := r.URL.Query().Get("workspace_id"); id != "" {
		return id
	}
	return r.Header.Get("X-Workspace-ID")
}

func CtxMember(ctx context.Context) (db.Member, bool) {
	return middleware.MemberFromContext(ctx)
}

func CtxWorkspaceID(ctx context.Context) string {
	return middleware.WorkspaceIDFromContext(ctx)
}

func WorkspaceIDFromURL(r *http.Request, param string) string {
	if id := middleware.WorkspaceIDFromContext(r.Context()); id != "" {
		return id
	}
	return chi.URLParam(r, param)
}