// Package fileauth decides whether a user is allowed to download a
// file by its storage key. The decision is enforced by the
// /api/files/{key} handler in the handler package.
package fileauth

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgtype"
)

// Store is the minimal data access layer that fileauth needs to decide
// whether a user is allowed to download a file by storage key. It is a
// seam that lets tests substitute a fake without a real database.
type Store interface {
	// GetAttachmentWorkspaceID returns the workspace ID for the
	// attachment whose URL matches the given key, or
	// ErrAttachmentNotFound if no such attachment exists.
	GetAttachmentWorkspaceID(ctx context.Context, fileKey string) (pgtype.UUID, error)
	// IsMember reports whether the user is a member of the workspace.
	IsMember(ctx context.Context, userID, workspaceID pgtype.UUID) (bool, error)
}

// ErrAttachmentNotFound is returned by Store implementations when no
// attachment matches the requested file key.
var ErrAttachmentNotFound = errors.New("attachment not found")

// Decision is the result of an authorization check for downloading a
// file. If Allowed is true, the request may proceed. Otherwise, Status
// and Message describe the response to send back.
type Decision struct {
	Allowed      bool
	Status       int
	Message      string
	WorkspaceID  pgtype.UUID
	AttachmentID pgtype.UUID
}

// Allow returns a Decision that permits the download.
func Allow(workspaceID, attachmentID pgtype.UUID) Decision {
	return Decision{Allowed: true, WorkspaceID: workspaceID, AttachmentID: attachmentID}
}

// Deny returns a Decision that rejects the request with the given HTTP
// status and message.
func Deny(status int, message string) Decision {
	return Decision{Status: status, Message: message}
}

// Decide checks whether the given user is allowed to download a file
// stored at the given key.
//
//   - 401 if userID is empty (defence-in-depth; the route is auth-gated).
//   - 400 if fileKey is empty.
//   - 404 if no attachment matches the key (do not leak existence).
//   - 403 if the user is not a member of the attachment's workspace.
//   - 500 if the underlying data store errors unexpectedly.
//   - Allowed=true otherwise.
func Decide(ctx context.Context, store Store, userID, fileKey string) Decision {
	if userID == "" {
		return Deny(401, "unauthorized")
	}
	if fileKey == "" {
		return Deny(400, "missing file key")
	}
	workspaceID, err := store.GetAttachmentWorkspaceID(ctx, fileKey)
	if err != nil {
		// Treat not-found and DB errors as 404 to avoid leaking existence.
		return Deny(404, "file not found")
	}
	userUUID := parseUUID(userID)
	wsUUID := normalizeUUID(workspaceID)
	ok, err := store.IsMember(ctx, userUUID, wsUUID)
	if err != nil {
		return Deny(500, "internal error")
	}
	if !ok {
		return Deny(403, "forbidden")
	}
	return Allow(wsUUID, pgtype.UUID{})
}
