package seed

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	dbpkg "github.com/agentra-ai/agentra/server/pkg/db/generated"
)

// fakeQ is an in-memory Q implementation. It only tracks the rows that
// would have been inserted; the actual database columns we exercise here
// are Name (used for idempotency) and CreateAgentParams.WorkspaceID.
type fakeQ struct {
	existing []dbpkg.Agent
	created  []dbpkg.CreateAgentParams
	listErr  error
}

func (f *fakeQ) ListAgents(_ context.Context, _ pgtype.UUID) ([]dbpkg.Agent, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.existing, nil
}

func (f *fakeQ) CreateAgent(_ context.Context, arg dbpkg.CreateAgentParams) (dbpkg.Agent, error) {
	f.created = append(f.created, arg)
	return dbpkg.Agent{ID: pgtype.UUID{}, Name: arg.Name, WorkspaceID: arg.WorkspaceID}, nil
}

func validUUID() pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}
}

func TestSeed_NoRuntime_ReturnsEarly(t *testing.T) {
	q := &fakeQ{}
	res, err := SeedForWorkspace(context.Background(), q, validUUID(), pgtype.UUID{}, pgtype.UUID{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Created) != 0 || len(res.Skipped) != 0 {
		t.Fatalf("expected empty result, got created=%v skipped=%v", res.Created, res.Skipped)
	}
	if len(q.created) != 0 {
		t.Fatalf("expected no create calls, got %d", len(q.created))
	}
}

func TestSeed_FreshWorkspace_CreatesAllTemplates(t *testing.T) {
	q := &fakeQ{}
	res, err := SeedForWorkspace(context.Background(), q, validUUID(), validUUID(), validUUID())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := len(res.Created), len(DefaultTemplates); got != want {
		t.Fatalf("created slugs: got %d, want %d (%v)", got, want, res.Created)
	}
	if len(res.Skipped) != 0 {
		t.Fatalf("expected no skipped, got %v", res.Skipped)
	}
	if got, want := len(q.created), len(DefaultTemplates); got != want {
		t.Fatalf("create calls: got %d, want %d", got, want)
	}
}

func TestSeed_Idempotent_SkipsExisting(t *testing.T) {
	// Populate "existing" with all template names so the second run skips
	// every template — this is the steady-state path on every daemon
	// reconnect.
	existing := make([]dbpkg.Agent, 0, len(DefaultTemplates))
	for _, t := range DefaultTemplates {
		existing = append(existing, dbpkg.Agent{Name: t.Name})
	}
	q := &fakeQ{existing: existing}

	res, err := SeedForWorkspace(context.Background(), q, validUUID(), validUUID(), validUUID())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Created) != 0 {
		t.Fatalf("expected no creations, got %v", res.Created)
	}
	if got, want := len(res.Skipped), len(DefaultTemplates); got != want {
		t.Fatalf("skipped slugs: got %d, want %d", got, want)
	}
	if len(q.created) != 0 {
		t.Fatalf("expected no create calls, got %d", len(q.created))
	}
}

func TestSeed_PartialExisting_CreatesMissing(t *testing.T) {
	// Pretend the workspace already has the Frontend Engineer (from a
	// prior partial seed or a manual create). The other 5 should be
	// inserted.
	q := &fakeQ{existing: []dbpkg.Agent{{Name: "Frontend Engineer"}}}

	res, err := SeedForWorkspace(context.Background(), q, validUUID(), validUUID(), validUUID())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := len(res.Created), len(DefaultTemplates)-1; got != want {
		t.Fatalf("created slugs: got %d (%v), want %d", got, res.Created, want)
	}
	if got, want := len(res.Skipped), 1; got != want {
		t.Fatalf("skipped slugs: got %d (%v), want %d", got, res.Skipped, want)
	}
	if res.Skipped[0] != "frontend-engineer" {
		t.Fatalf("expected frontend-engineer skipped, got %v", res.Skipped)
	}
}

func TestSeed_DeterministicOrder(t *testing.T) {
	// Two fresh runs against fresh fakes must produce the same Created
	// slice in the same order. We sort by slug inside SeedForWorkspace
	// for exactly this property.
	q1 := &fakeQ{}
	q2 := &fakeQ{}
	r1, _ := SeedForWorkspace(context.Background(), q1, validUUID(), validUUID(), validUUID())
	r2, _ := SeedForWorkspace(context.Background(), q2, validUUID(), validUUID(), validUUID())
	if len(r1.Created) != len(r2.Created) {
		t.Fatalf("created length differs: %d vs %d", len(r1.Created), len(r2.Created))
	}
	for i := range r1.Created {
		if r1.Created[i] != r2.Created[i] {
			t.Fatalf("created[%d] differs: %q vs %q", i, r1.Created[i], r2.Created[i])
		}
	}
}
