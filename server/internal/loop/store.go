package loop

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	dbpkg "github.com/agentra-ai/agentra/server/pkg/db/generated"
	"github.com/agentra-ai/agentra/server/internal/util"
)

// CreateLoopInput captures the caller-supplied fields needed to create a new
// engineering loop. All fields except IssueID and WorkspaceID are optional.
type CreateLoopInput struct {
	IssueID       string
	WorkspaceID   string
	MaxIterations *int
	AgentID       *string
	Config        []byte // JSON; nil becomes "{}"
}

// UpdateStatusInput captures the mutable fields of a loop's state machine
// record. Each pointer is optional; nil means "leave unchanged" (for fields
// the SQL COALESCE-style semantics support) or "don't set" otherwise.
type UpdateStatusInput struct {
	Status        *Status
	CurrentStage  *Stage
	Iteration     *int
	FailureReason *string
	StartedAt     *time.Time // nil leaves the column unchanged (COALESCE in SQL)
	CompletedAt   *time.Time
}

// Store is a thin CRUD wrapper around the sqlc-generated loops queries.
// It converts between sqlc types (pgtype.UUID/Text/Timestamptz/Int4) and the
// domain Loop type defined in loop.go.
type Store struct {
	q *dbpkg.Queries
}

func NewStore(q *dbpkg.Queries) *Store { return &Store{q: q} }

// ErrLoopNotFound is returned by GetLoop when no row matches the given id.
var ErrLoopNotFound = errors.New("loop not found")

// CreateLoop inserts a new loop row in 'pending' status and returns the
// persisted domain record. The id is generated server-side via uuid.New.
func (s *Store) CreateLoop(ctx context.Context, in CreateLoopInput) (*Loop, error) {
	config := in.Config
	if config == nil {
		config = []byte("{}")
	}
	row, err := s.q.CreateLoop(ctx, dbpkg.CreateLoopParams{
		IssueID:       util.ParseUUID(in.IssueID),
		WorkspaceID:   util.ParseUUID(in.WorkspaceID),
		Status:        nil, // COALESCE('pending')
		CurrentStage:  pgtype.Text{},
		Iteration:     nil, // COALESCE(0)
		MaxIterations: int32PtrOrZero(in.MaxIterations),
		AgentID:       uuidOrNil(in.AgentID),
		Config:        config,
	})
	if err != nil {
		return nil, fmt.Errorf("CreateLoop: %w", err)
	}
	return rowToLoop(row)
}

// GetLoop returns the loop with the given id, or ErrLoopNotFound if no row matches.
func (s *Store) GetLoop(ctx context.Context, id string) (*Loop, error) {
	row, err := s.q.GetLoop(ctx, util.ParseUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrLoopNotFound
		}
		return nil, fmt.Errorf("GetLoop: %w", err)
	}
	return rowToLoop(row)
}

// UpdateStatus advances the state machine fields on a loop. The Status
// pointer is required by callers that want to actually change state; nil
// is permitted but the SQL will write an empty string, which violates the
// status CHECK constraint. Callers should always set Status when intent is
// to change it.
func (s *Store) UpdateStatus(ctx context.Context, id string, in UpdateStatusInput) (*Loop, error) {
	var statusVal string
	if in.Status != nil {
		statusVal = string(*in.Status)
	}
	row, err := s.q.UpdateLoopStatus(ctx, dbpkg.UpdateLoopStatusParams{
		ID:            util.ParseUUID(id),
		Status:        statusVal,
		CurrentStage:  stageToText(in.CurrentStage),
		Iteration:     int32(derefOrZero(in.Iteration)),
		FailureReason: util.PtrToText(in.FailureReason),
		StartedAt:     timeToTimestamptz(in.StartedAt),
		CompletedAt:   timeToTimestamptz(in.CompletedAt),
	})
	if err != nil {
		return nil, fmt.Errorf("UpdateLoopStatus: %w", err)
	}
	return rowToLoop(row)
}

// SetPR records the GitHub PR URL, number, and branch produced by a
// successful develop stage. All three fields are required (the develop
// stage must produce all of them).
func (s *Store) SetPR(ctx context.Context, id, url string, num int, branch string) (*Loop, error) {
	row, err := s.q.SetLoopPR(ctx, dbpkg.SetLoopPRParams{
		ID:         util.ParseUUID(id),
		PrUrl:      util.StrToText(url),
		PrNumber:   pgtype.Int4{Int32: int32(num), Valid: true},
		BranchName: util.StrToText(branch),
	})
	if err != nil {
		return nil, fmt.Errorf("SetLoopPR: %w", err)
	}
	return rowToLoop(row)
}

// LoadActive returns all loops in 'running' or 'paused' status, ordered by
// creation time. Used by the coordinator on startup to resume in-flight
// loops.
func (s *Store) LoadActive(ctx context.Context) ([]*Loop, error) {
	rows, err := s.q.LoadActiveLoops(ctx)
	if err != nil {
		return nil, fmt.Errorf("LoadActiveLoops: %w", err)
	}
	out := make([]*Loop, 0, len(rows))
	for _, r := range rows {
		l, err := rowToLoop(r)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, nil
}

// rowToLoop converts a sqlc-generated db.Loop row into a *Loop domain value.
// The conversion handles all 17 columns including the four timestamps, the
// three nullable text fields, the nullable int (pr_number), and the nullable
// UUID (agent_id).
func rowToLoop(r dbpkg.Loop) (*Loop, error) {
	out := &Loop{
		ID:            util.UUIDToString(r.ID),
		IssueID:       util.UUIDToString(r.IssueID),
		WorkspaceID:   util.UUIDToString(r.WorkspaceID),
		Status:        Status(r.Status),
		CurrentStage:  stageFromText(r.CurrentStage),
		Iteration:     int(r.Iteration),
		MaxIterations: int(r.MaxIterations),
		PRURL:         util.TextToPtr(r.PrUrl),
		PRNumber:      int4ToPtr(r.PrNumber),
		BranchName:    util.TextToPtr(r.BranchName),
		AgentID:       util.UUIDToPtr(r.AgentID),
		Config:        r.Config,
		FailureReason: util.TextToPtr(r.FailureReason),
		StartedAt:     timestamptzToPtr(r.StartedAt),
		CompletedAt:   timestamptzToPtr(r.CompletedAt),
		CreatedAt:     timestamptzToTime(r.CreatedAt),
		UpdatedAt:     timestamptzToTime(r.UpdatedAt),
	}
	return out, nil
}

// stageToText converts a *Stage (domain) to pgtype.Text (sqlc).
func stageToText(s *Stage) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: string(*s), Valid: true}
}

// stageFromText converts a pgtype.Text (sqlc) to *Stage (domain).
func stageFromText(t pgtype.Text) *Stage {
	if !t.Valid {
		return nil
	}
	s := Stage(t.String)
	return &s
}

// int4ToPtr converts a pgtype.Int4 to *int, returning nil when invalid.
func int4ToPtr(n pgtype.Int4) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int32)
	return &v
}

// timeToTimestamptz converts a *time.Time to pgtype.Timestamptz.
// A nil pointer produces an invalid Timestamptz, which the SQL treats as
// "do not set" (the column is nullable; for started_at the SQL also has
// COALESCE to preserve any existing value).
func timeToTimestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// timestamptzToPtr converts a pgtype.Timestamptz to *time.Time.
// Returns nil for invalid (null) columns.
func timestamptzToPtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

// timestamptzToTime converts a pgtype.Timestamptz to time.Time.
// Used for the NOT NULL created_at/updated_at columns; the zero value is
// returned for invalid (which should not occur in practice).
func timestamptzToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

// uuidOrNil converts a *string UUID to pgtype.UUID, returning an invalid
// UUID for nil (which writes SQL NULL to the column).
func uuidOrNil(s *string) pgtype.UUID {
	if s == nil {
		return pgtype.UUID{}
	}
	return util.ParseUUID(*s)
}

// int32PtrOrZero dereferences a *int to a plain int, returning 0 for nil.
// sqlc-generated CreateLoopParams.MaxIterations is interface{} so we wrap.
func int32PtrOrZero(p *int) interface{} {
	if p == nil {
		return nil
	}
	return int32(*p)
}

// derefOrZero dereferences a *int, returning 0 for nil. Used to map
// UpdateStatusInput.Iteration into the sqlc UpdateLoopStatusParams field
// (int32, not nullable).
func derefOrZero(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
