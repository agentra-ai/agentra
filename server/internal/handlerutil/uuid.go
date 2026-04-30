package handlerutil

import (
	"github.com/agentra-ai/agentra/server/internal/util"
	"github.com/jackc/pgx/v5/pgtype"
)

// Thin wrappers around util functions — these preserve existing handler call sites unchanged.
func ParseUUID(s string) pgtype.UUID      { return util.ParseUUID(s) }
func UUIDToString(u pgtype.UUID) string  { return util.UUIDToString(u) }
func TextToPtr(t pgtype.Text) *string     { return util.TextToPtr(t) }
func PtrToText(s *string) pgtype.Text    { return util.PtrToText(s) }
func StrToText(s string) pgtype.Text     { return util.StrToText(s) }
func TimestampToString(t pgtype.Timestamptz) string { return util.TimestampToString(t) }
func TimestampToPtr(t pgtype.Timestamptz) *string   { return util.TimestampToPtr(t) }
func UUIDToPtr(u pgtype.UUID) *string     { return util.UUIDToPtr(u) }