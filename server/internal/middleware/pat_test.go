package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	db "github.com/agentra-ai/agentra/server/pkg/db/generated"
	"github.com/jackc/pgx/v5/pgtype"
)

// fakePATStore implements PATStore for tests. It records the number of
// concurrent UpdatePersonalAccessTokenLastUsed calls and the total
// invocations. The PAT itself is fixed so test assertions are simple.
type fakePATStore struct {
	pat                 db.PersonalAccessToken
	updateCalls         atomic.Int32
	maxConcurrentUpdate atomic.Int32
	currentUpdate       atomic.Int32
	slowUpdate          time.Duration
}

func (f *fakePATStore) GetPersonalAccessTokenByHash(_ context.Context, _ string) (db.PersonalAccessToken, error) {
	return f.pat, nil
}

func (f *fakePATStore) UpdatePersonalAccessTokenLastUsed(_ context.Context, _ pgtype.UUID) error {
	cur := f.currentUpdate.Add(1)
	defer f.currentUpdate.Add(-1)
	if cur > f.maxConcurrentUpdate.Load() {
		f.maxConcurrentUpdate.Store(cur)
	}
	f.updateCalls.Add(1)
	if f.slowUpdate > 0 {
		time.Sleep(f.slowUpdate)
	}
	return nil
}

func newFakeStore() *fakePATStore {
	return &fakePATStore{
		pat: db.PersonalAccessToken{
			ID:     pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
			UserID: pgtype.UUID{Bytes: [16]byte{2}, Valid: true},
		},
	}
}

// TestAuth_PATUpdateDoesNotLeakGoroutines is the regression test for the
// goroutine leak: previously, Auth() called
//
//	go queries.UpdatePersonalAccessTokenLastUsed(context.Background(), pat.ID)
//
// on every PAT-authenticated request, growing the goroutine count without
// bound. The fix runs the update in a detached context but inline (no
// `go` statement), so each request's goroutine count returns to
// baseline when the handler returns.
func TestAuth_PATUpdateDoesNotLeakGoroutines(t *testing.T) {
	store := newFakeStore()
	handler := AuthWithPATStore(store)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Warm up — let the runtime settle.
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/api/me", nil)
		req.Header.Set("Authorization", "Bearer mul_test_token")
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := runtime.NumGoroutine()
	baselineCalls := store.updateCalls.Load()

	const requests = 200
	for i := 0; i < requests; i++ {
		req := httptest.NewRequest("GET", "/api/me", nil)
		req.Header.Set("Authorization", "Bearer mul_test_token")
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}

	// Allow any stray goroutines to complete.
	time.Sleep(200 * time.Millisecond)
	runtime.GC()
	after := runtime.NumGoroutine()

	if delta := after - baseline; delta > 5 {
		t.Fatalf("goroutine count grew by %d after %d requests (baseline=%d, after=%d) — likely a leak",
			delta, requests, baseline, after)
	}

	if calls := store.updateCalls.Load() - baselineCalls; calls != requests {
		t.Fatalf("expected %d new update calls, got %d", requests, calls)
	}
}

// TestAuth_PATUpdateDoesNotSpawnGoroutineUnderLoad asserts that even
// with slow updates (representing contended DB), the number of in-flight
// update goroutines never exceeds 1 — the fix runs the update inline,
// not in a `go` statement.
func TestAuth_PATUpdateDoesNotSpawnGoroutineUnderLoad(t *testing.T) {
	store := newFakeStore()
	store.slowUpdate = 50 * time.Millisecond

	handler := AuthWithPATStore(store)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Run sequentially. With `go` detached, the slow update would
	// overlap with the next request. With inline execution, each
	// request blocks for ~50ms, and the in-flight update count
	// stays at 1.
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/api/me", nil)
		req.Header.Set("Authorization", "Bearer mul_test_token")
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}

	if max := store.maxConcurrentUpdate.Load(); max > 1 {
		t.Fatalf("update ran concurrently up to %d times — fix should be inline, not detached", max)
	}
}
