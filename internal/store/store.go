package store

import (
	"sync"
	"time"
)

// entry pairs a value with an optional expiry time. expiresAt's zero
// value (time.Time{}) means "no expiry" — deliberately chosen so a
// freshly-SET key (which has no TTL) doesn't need any special-casing
// beyond just... not setting the field.
type entry struct {
	value     string
	expiresAt time.Time
}

func isExpired(e entry, now time.Time) bool {
	if e.expiresAt.IsZero() {
		return false
	}
	return now.After(e.expiresAt)
}

type Store struct {
	mu   sync.RWMutex
	data map[string]entry
}

// New starts the store and its active-expiry sweep goroutine. The
// sweep runs for the lifetime of the process — there's no stop
// channel yet, which is a fine simplification for a single-process
// learning project (worth revisiting with context.Context if this
// ever needs graceful shutdown, e.g. for tests that create many
// Stores).
func New() *Store {
	s := &Store{data: make(map[string]entry)}
	go s.runActiveExpiry(100 * time.Millisecond)
	return s
}

func (s *Store) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Overwriting with a fresh entry{} clears any prior TTL, matching
	// real Redis: a plain SET removes an existing expiry on that key.
	s.data[key] = entry{value: value}
}

// get is the shared lazy-expiry-aware lookup used by every read path
// (Get, Exists, TTL). Centralizing it means the expiry logic exists
// in exactly one place instead of being copy-pasted per command.
//
// The two-step locking here is deliberate: take a read lock first
// (cheap, allows concurrent reads) and only escalate to a write lock
// if we actually find something to clean up. Between releasing the
// read lock and acquiring the write lock, another goroutine could
// have changed the key (e.g. re-SET it) — so we re-check under the
// write lock before deleting. This "check, then double-check under
// the stronger lock" shape is a standard pattern anytime you want to
// avoid holding an expensive lock for the common case.
func (s *Store) get(key string) (entry, bool) {
	s.mu.RLock()
	e, ok := s.data[key]
	s.mu.RUnlock()

	if !ok {
		return entry{}, false
	}
	if !isExpired(e, time.Now()) {
		return e, true
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if e2, ok := s.data[key]; ok && isExpired(e2, time.Now()) {
		delete(s.data, key)
	}
	return entry{}, false
}

func (s *Store) Get(key string) (string, bool) {
	e, ok := s.get(key)
	if !ok {
		return "", false
	}
	return e.value, true
}

func (s *Store) Exists(keys ...string) int {
	count := 0
	for _, k := range keys {
		if _, ok := s.get(k); ok {
			count++
		}
	}
	return count
}

// Del removes each key unconditionally, but only counts a key toward
// the returned total if it was still logically alive (not already
// expired). This matches how a client would perceive it: an expired
// key "doesn't exist" from their point of view even if the active
// sweep hasn't physically removed it yet.
func (s *Store) Del(keys ...string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	count := 0
	for _, k := range keys {
		e, ok := s.data[k]
		if !ok {
			continue
		}
		delete(s.data, k)
		if !isExpired(e, now) {
			count++
		}
	}
	return count
}

// Expire sets key to expire after ttl from now. Returns false if the
// key doesn't exist (or is already expired) — matching real Redis's
// EXPIRE, which returns 0 in that case instead of erroring.
func (s *Store) Expire(key string, ttl time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.data[key]
	if !ok || isExpired(e, time.Now()) {
		return false
	}
	e.expiresAt = time.Now().Add(ttl)
	s.data[key] = e // structs in maps aren't addressable — must re-store, can't mutate e.expiresAt in place
	return true
}

// Persist removes a key's expiry, making it permanent again. Returns
// false if the key doesn't exist or already had no expiry — matching
// real Redis's PERSIST (returns 1 only if a timeout was actually
// removed).
func (s *Store) Persist(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.data[key]
	if !ok || isExpired(e, time.Now()) || e.expiresAt.IsZero() {
		return false
	}
	e.expiresAt = time.Time{}
	s.data[key] = e
	return true
}

// TTL reports a key's remaining lifetime. The three-value return
// mirrors the three states a client actually cares about — "doesn't
// exist" and "exists but permanent" both need to be distinguishable
// from an actual remaining duration, and neither can be represented
// as a time.Duration on its own without an out-of-band signal.
func (s *Store) TTL(key string) (remaining time.Duration, exists bool, hasExpiry bool) {
	e, ok := s.get(key)
	if !ok {
		return 0, false, false
	}
	if e.expiresAt.IsZero() {
		return 0, true, false
	}
	return time.Until(e.expiresAt), true, true
}

// purgeExpired does one full pass over the map, deleting anything
// past its expiry. A full scan is O(n) per sweep — perfectly fine at
// the size this toy store will ever reach. Real Redis instead
// samples a small random subset of keys with TTLs each cycle, since
// scanning millions of keys every 100ms would be its own performance
// problem — worth knowing that trade-off exists, not worth building
// here.
func (s *Store) purgeExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, e := range s.data {
		if isExpired(e, now) {
			delete(s.data, k)
		}
	}
}

func (s *Store) runActiveExpiry(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		s.purgeExpired()
	}
}
