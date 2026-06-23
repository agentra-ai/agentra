package fileauth

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

// notFoundFake returns ErrAttachmentNotFound for any key.
type notFoundFake struct{ calls int }

func (f *notFoundFake) GetAttachmentWorkspaceID(_ context.Context, _ string) (pgtype.UUID, error) {
	f.calls++
	return pgtype.UUID{}, ErrAttachmentNotFound
}
func (f *notFoundFake) IsMember(_ context.Context, _, _ pgtype.UUID) (bool, error) {
	return false, nil
}

// errorGetFake returns a generic error from GetAttachmentWorkspaceID.
type errorGetFake struct{ calls int }

func (f *errorGetFake) GetAttachmentWorkspaceID(_ context.Context, _ string) (pgtype.UUID, error) {
	f.calls++
	return pgtype.UUID{}, errors.New("db down")
}
func (f *errorGetFake) IsMember(_ context.Context, _, _ pgtype.UUID) (bool, error) {
	return false, nil
}

// fixedStore is a Store that returns a fixed workspace and the configured
// membership decision.
type fixedStore struct {
	workspace pgtype.UUID
	member    bool
	getErr    error
	memberErr error
	getCalls  int
	memCalls  int
}

func (f *fixedStore) GetAttachmentWorkspaceID(_ context.Context, _ string) (pgtype.UUID, error) {
	f.getCalls++
	if f.getErr != nil {
		return pgtype.UUID{}, f.getErr
	}
	return f.workspace, nil
}

func (f *fixedStore) IsMember(_ context.Context, _, _ pgtype.UUID) (bool, error) {
	f.memCalls++
	if f.memberErr != nil {
		return false, f.memberErr
	}
	return f.member, nil
}

func ws() pgtype.UUID {
	return pgtype.UUID{Bytes: [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}, Valid: true}
}

func TestDecide_EmptyUser_Denied401(t *testing.T) {
	store := &fixedStore{workspace: ws(), member: true}
	d := Decide(context.Background(), store, "", "k")
	if d.Allowed {
		t.Fatalf("expected denial, got allowed")
	}
	if d.Status != 401 {
		t.Fatalf("expected 401, got %d", d.Status)
	}
}

func TestDecide_EmptyKey_Denied400(t *testing.T) {
	store := &fixedStore{workspace: ws(), member: true}
	d := Decide(context.Background(), store, "user", "")
	if d.Status != 400 {
		t.Fatalf("expected 400, got %d", d.Status)
	}
}

func TestDecide_AttachmentNotFound_Denied404(t *testing.T) {
	store := &notFoundFake{}
	d := Decide(context.Background(), store, "user", "missing")
	if d.Status != 404 {
		t.Fatalf("expected 404, got %d", d.Status)
	}
	if store.calls != 1 {
		t.Fatalf("expected 1 attachment lookup, got %d", store.calls)
	}
}

func TestDecide_StoreError_Denied404(t *testing.T) {
	store := &errorGetFake{}
	d := Decide(context.Background(), store, "user", "any")
	if d.Status != 404 {
		t.Fatalf("expected 404 on store error, got %d", d.Status)
	}
}

func TestDecide_NonMember_Denied403(t *testing.T) {
	store := &fixedStore{workspace: ws(), member: false}
	d := Decide(context.Background(), store, "00000000-0000-0000-0000-000000000001", "k")
	if d.Allowed {
		t.Fatalf("expected denial, got %+v", d)
	}
	if d.Status != 403 {
		t.Fatalf("expected 403, got %d", d.Status)
	}
	if store.getCalls != 1 || store.memCalls != 1 {
		t.Fatalf("expected one of each call, got getCalls=%d memCalls=%d", store.getCalls, store.memCalls)
	}
}

func TestDecide_Member_Allowed(t *testing.T) {
	store := &fixedStore{workspace: ws(), member: true}
	d := Decide(context.Background(), store, "00000000-0000-0000-0000-000000000001", "k")
	if !d.Allowed {
		t.Fatalf("expected allow, got status=%d msg=%q", d.Status, d.Message)
	}
}

func TestDecide_MemberError_Denied500(t *testing.T) {
	store := &fixedStore{workspace: ws(), memberErr: errors.New("boom")}
	d := Decide(context.Background(), store, "00000000-0000-0000-0000-000000000001", "k")
	if d.Status != 500 {
		t.Fatalf("expected 500, got %d", d.Status)
	}
}

func TestDecide_InvalidUserUUID_AllowedIfMember(t *testing.T) {
	store := &fixedStore{workspace: ws(), member: true}
	d := Decide(context.Background(), store, "not-a-uuid", "k")
	if !d.Allowed {
		t.Fatalf("expected allow (member), got status=%d msg=%q", d.Status, d.Message)
	}
}
