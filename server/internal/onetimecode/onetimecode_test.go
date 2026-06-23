package onetimecode

import (
	"sync"
	"testing"
	"time"
)

// newTestStore returns a Store with controllable time and random source
// for deterministic tests.
func newTestStore(ttl time.Duration) (*Store, *fakeClock) {
	clk := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	s := New(ttl)
	s.now = clk.Now
	return s, clk
}

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func TestStore_PutReturnsCodeAndTakeRedeems(t *testing.T) {
	s, _ := newTestStore(time.Minute)
	code := s.Put("secret-token")
	if code == "" {
		t.Fatal("Put returned empty code")
	}
	if code == "secret-token" {
		t.Fatalf("code must not equal token; got %q", code)
	}
	got, ok := s.Take(code)
	if !ok || got != "secret-token" {
		t.Fatalf("Take(%q) = (%q, %v), want (secret-token, true)", code, got, ok)
	}
}

func TestStore_TakeIsSingleUse(t *testing.T) {
	s, _ := newTestStore(time.Minute)
	code := s.Put("secret-token")
	if _, ok := s.Take(code); !ok {
		t.Fatal("first Take should succeed")
	}
	if _, ok := s.Take(code); ok {
		t.Fatal("second Take should fail — code is single-use")
	}
}

func TestStore_TakeUnknownCode(t *testing.T) {
	s, _ := newTestStore(time.Minute)
	if _, ok := s.Take("nonexistent"); ok {
		t.Fatal("Take of unknown code should fail")
	}
}

func TestStore_ExpiredCodeIsRejected(t *testing.T) {
	s, clk := newTestStore(time.Minute)
	code := s.Put("secret-token")
	clk.Advance(2 * time.Minute) // past TTL
	if _, ok := s.Take(code); ok {
		t.Fatal("Take of expired code should fail")
	}
}

func TestStore_DistinctCodesPerPut(t *testing.T) {
	s, _ := newTestStore(time.Minute)
	a := s.Put("a")
	b := s.Put("b")
	if a == b {
		t.Fatalf("expected distinct codes, both %q", a)
	}
}

func TestStore_SweepRemovesExpiredOnly(t *testing.T) {
	s, clk := newTestStore(time.Minute)
	s.Put("short-lived")
	clk.Advance(2 * time.Minute)
	live := s.Put("long-lived")
	removed := s.SweepExpired()
	if removed != 1 {
		t.Fatalf("SweepExpired removed %d entries, want 1", removed)
	}
	// live code should still redeem
	if _, ok := s.Take(live); !ok {
		t.Fatal("sweep should not remove a non-expired code")
	}
}

func TestStore_ConcurrentPutAndTake(t *testing.T) {
	// Smoke test for the race detector.
	s := New(time.Minute)
	const n = 100
	var wg sync.WaitGroup
	wg.Add(2 * n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			s.Put("token")
		}()
		go func() {
			defer wg.Done()
			// random code; mostly misses
			s.Take("nope")
		}()
	}
	wg.Wait()
}
