package fileauth

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// parseUUID parses a string UUID and returns a zero UUID on failure.
// We never return the error to the caller because Decide coerces
// invalid input into a 404/500 path via the downstream DB query.
func parseUUID(s string) pgtype.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: id, Valid: true}
}

// normalizeUUID returns the same UUID unchanged. It exists so that
// callers reading the Decide source can see that the workspaceID
// coming out of GetAttachmentWorkspaceID is already a pgtype.UUID
// ready to be passed back into IsMember.
func normalizeUUID(u pgtype.UUID) pgtype.UUID {
	return u
}
