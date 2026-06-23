// Package onetimecode provides a single-use, time-bounded code store used
// to hand off short-lived secrets (e.g. OAuth access tokens) across an
// HTTP redirect without putting the secret on the wire.
//
// Producers call Put(token) and embed the returned opaque code in a
// redirect URL. Consumers call Take(code) to redeem exactly once.
package onetimecode

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Store maps short opaque codes to tokens. Codes expire after TTL and are
// removed on Take (single-use).
type Store struct {
	mu   sync.Mutex
	data map[string]entry
	ttl  time.Duration
	now  func() time.Time
	rand func() string
}

type entry struct {
	token     string
	expiresAt time.Time
}

// New returns a Store with the given TTL. TTL must be positive; pass
// values like 5*time.Minute for a typical OAuth handoff window.
func New(ttl time.Duration) *Store {
	return &Store{
		data: make(map[string]entry),
		ttl:  ttl,
		now:  time.Now,
		rand: randomCode,
	}
}

// Put stores token under a fresh opaque code and returns that code.
func (s *Store) Put(token string) string {
	code := s.rand()
	expires := s.now().Add(s.ttl)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[code] = entry{token: token, expiresAt: expires}
	return code
}

// Take returns the token for code, or (zero, false) if the code is
// unknown, expired, or already consumed. The code is removed on success.
func (s *Store) Take(code string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.data[code]
	if !ok {
		return "", false
	}
	delete(s.data, code)
	if s.now().After(e.expiresAt) {
		return "", false
	}
	return e.token, true
}

// SweepExpired removes entries past their expiry. Safe to call from a
// background goroutine. Returns the number of entries removed.
func (s *Store) SweepExpired() int {
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := 0
	for k, e := range s.data {
		if now.After(e.expiresAt) {
			delete(s.data, k)
			removed++
		}
	}
	return removed
}

func randomCode() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
