package store

import "sync"

// Store is a concurrency-safe in-memory map. It's the thing that
// actually persists data between commands — everything before this
// point (resp, server) exists to get bytes safely to and from here.
//
// sync.RWMutex instead of sync.Mutex: reads (GET, EXISTS) can happen
// concurrently with each other, they only need to block against
// writes (SET, DEL). Since real workloads read far more than they
// write, this matters for throughput once there's more than one
// client hammering the server at once.
type Store struct {
	mu   sync.RWMutex
	data map[string]string
}

func New() *Store {
	return &Store{data: make(map[string]string)}
}

func (s *Store) Set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}

// Get returns the value and whether the key existed — mirroring Go's
// own "comma ok" map idiom. This distinction matters at the RESP
// layer: a key that doesn't exist encodes as a null bulk string
// ($-1\r\n), not an empty one ($0\r\n\r\n), so the caller needs to
// know which case it is, not just get back "".
func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[key]
	return v, ok
}

// Del removes each given key if present and returns how many were
// actually deleted — matching real Redis's DEL semantics (it accepts
// multiple keys in one call and returns a count, not a bool).
func (s *Store) Del(keys ...string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, k := range keys {
		if _, ok := s.data[k]; ok {
			delete(s.data, k)
			count++
		}
	}
	return count
}

// Exists counts how many of the given keys are present. Also matches
// real Redis: EXISTS accepts multiple keys and counts matches, it
// doesn't just return a bool for one key.
func (s *Store) Exists(keys ...string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, k := range keys {
		if _, ok := s.data[k]; ok {
			count++
		}
	}
	return count
}
