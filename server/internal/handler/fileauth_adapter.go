package handler

import (
	"context"

	"github.com/agentra-ai/agentra/server/internal/fileauth"
	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
	"github.com/jackc/pgx/v5/pgtype"
)

// fileAuthStore adapts the existing *db.Queries to the fileauth.Store
// interface. Production callers construct it inside the handler; tests
// in the fileauth package use their own fakes.
type fileAuthStore struct {
	q *db.Queries
}

func (s fileAuthStore) GetAttachmentWorkspaceID(ctx context.Context, fileKey string) (pgtype.UUID, error) {
	url := "/api/files/" + fileKey
	att, err := s.q.GetAttachmentByURL(ctx, url)
	if err != nil {
		// sqlc returns pgx.ErrNoRows for missing rows. We propagate as
		// fileauth.ErrAttachmentNotFound so the Decide function can
		// map it to a 404 without leaking existence.
		return pgtype.UUID{}, fileauth.ErrAttachmentNotFound
	}
	return att.WorkspaceID, nil
}

func (s fileAuthStore) IsMember(ctx context.Context, userID, workspaceID pgtype.UUID) (bool, error) {
	_, err := s.q.GetMemberByUserAndWorkspace(ctx, db.GetMemberByUserAndWorkspaceParams{
		UserID:      userID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		// pgx.ErrNoRows is the "not a member" case; treat as false.
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
