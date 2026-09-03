package store

import (
	"testing"
	"time"
)

// This uses the newWithActiveExpiryInterval testing seam to push the
// active sweep interval out to an hour — effectively disabling it
// for the test's lifetime. That's what makes this test actually
// prove something about LAZY expiry specifically: if it used New()
// (real 100ms sweep) instead, we'd have no way to know whether Get
// found the key already-gone (active sweep's doing) or found it
// present-but-expired and cleaned it up itself (lazy's doing) — both
// look identical from Get's return value alone.
//
// The two direct s.data checks (bypassing Get, reading the map
// straight — legal because this test file is `package store`, not
// `package store_test`, so it can see unexported fields) are what
// actually pin down *which* mechanism did the deleting: proving the
// key is still physically present right up until the Get call, and
// gone immediately after it.
func TestStore_LazyExpiry_OnRead(t *testing.T) {
	s := newWithActiveExpiryInterval(time.Hour)
	s.Set("foo", "bar")
	s.Expire("foo", 10*time.Millisecond)

	time.Sleep(20 * time.Millisecond) // now logically expired, but nothing has swept it yet

	s.mu.RLock()
	_, presentBeforeGet := s.data["foo"]
	s.mu.RUnlock()
	if !presentBeforeGet {
		t.Fatal(`test setup invalid: "foo" was already gone before Get ever ran`)
	}

	if _, ok := s.Get("foo"); ok {
		t.Error(`expected Get("foo") to report not-found once expired`)
	}

	s.mu.RLock()
	_, presentAfterGet := s.data["foo"]
	s.mu.RUnlock()
	if presentAfterGet {
		t.Error(`expected Get to have lazily removed "foo" from the map, but it's still there`)
	}
}

// Mirror image of the lazy test: a fast sweep interval, and —
// critically — Get is NEVER called here. If this test passes, the
// only possible explanation is the background goroutine did it.
func TestStore_ActiveExpiry_PurgesInBackground(t *testing.T) {
	s := newWithActiveExpiryInterval(20 * time.Millisecond)
	s.Set("foo", "bar")
	s.Expire("foo", 10*time.Millisecond)

	time.Sleep(100 * time.Millisecond) // several sweep ticks' worth of time

	s.mu.RLock()
	_, stillPresent := s.data["foo"]
	s.mu.RUnlock()
	if stillPresent {
		t.Error(`expected "foo" to have been purged by the active sweep, but it's still in the map`)
	}
}

func TestStore_TTL(t *testing.T) {
	s := New()
	s.Set("foo", "bar")
	s.Expire("foo", time.Second)

	remaining, exists, hasExpiry := s.TTL("foo")
	if !exists || !hasExpiry {
		t.Fatalf(`TTL("foo") exists=%v hasExpiry=%v, want true, true`, exists, hasExpiry)
	}
	// Not asserting an exact duration (that's inherently flaky — real
	// time passes between Expire and TTL, if only by nanoseconds) but
	// checking it's in a sane range catches a real class of bug, e.g.
	// accidentally returning the full TTL instead of what's *left*,
	// or a sign error making it negative.
	if remaining <= 0 || remaining > time.Second {
		t.Errorf(`TTL("foo") remaining = %v, want (0, 1s]`, remaining)
	}
}

func TestStore_TTL_Unexistant_Key(t *testing.T) {
	s := New()
	if _, exists, _ := s.TTL("foo"); exists{
		t.Errorf(`s.TTL("foo") exists = true, want false`)
	}
}

func TestStore_Expire_Unexistant_Key(t *testing.T) {
	s := New()
	if ok := s.Expire("foo", 10*time.Millisecond); ok{
		t.Errorf(`s.Expire("foo") = true, want false`)
	}
}

func TestStore_Expire_Expired_Not_Purged_Key(t *testing.T) {
	s := newWithActiveExpiryInterval(time.Hour)
	s.Set("foo", "bar")
	s.Expire("foo", 10*time.Millisecond)
	time.Sleep(100 * time.Millisecond) // several sweep ticks' worth of time

	if ok := s.Expire("foo", 10*time.Millisecond); ok{
		t.Errorf(`s.Expire("foo") = true, want false`)
	}
}

// One table, one case per branch of Persist's actual guard —
//
//	if !ok || isExpired(e, time.Now()) || e.expiresAt.IsZero() {
//
// — deliberately laid out to mirror that condition term for term, so
// the table itself documents what each branch means. Every case uses
// a store with the active sweep pushed out to an hour, which matters
// specifically for the "expired but not yet swept" case: without
// that, the background sweep could remove the key before Persist
// even runs, and the test would no longer be proving what it claims.
func TestStore_Persist(t *testing.T) {
	cases := []struct {
		name  string
		setup func(s *Store) // populates "foo" (or doesn't) before Persist runs
		want  bool
	}{
		{
			name:  "key does not exist",
			setup: func(s *Store) {},
			want:  false,
		},
		{
			name: "key exists, no expiry set",
			setup: func(s *Store) {
				s.Set("foo", "bar")
			},
			want: false,
		},
		{
			name: "key exists, has an active expiry",
			setup: func(s *Store) {
				s.Set("foo", "bar")
				s.Expire("foo", time.Minute)
			},
			want: true,
		},
		{
			name: "key exists but already expired, not yet swept",
			setup: func(s *Store) {
				s.Set("foo", "bar")
				s.Expire("foo", 10*time.Millisecond)
				time.Sleep(20 * time.Millisecond)
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newWithActiveExpiryInterval(time.Hour)
			tc.setup(s)

			got := s.Persist("foo")
			if got != tc.want {
				t.Errorf(`Persist("foo") = %v, want %v`, got, tc.want)
			}

			// For the one case where Persist actually succeeds, also
			// confirm it did what it claims — the TTL is really gone
			// afterward, not just that it returned true.
			if tc.want {
				_, _, hasExpiry := s.TTL("foo")
				if hasExpiry {
					t.Error(`expected "foo" to have no expiry after Persist, but it still has one`)
				}
			}
		})
	}
}


func TestStore_Del_Expired_Key(t *testing.T) {
	s := newWithActiveExpiryInterval(time.Hour)
	s.Set("foo", "bar")
	s.Expire("foo", 10*time.Millisecond)
	time.Sleep(20*time.Millisecond)

	if count := s.Del("foo"); count != 0{
			t.Errorf(`expected s.Del("foo") count to be 0, it was %v`, count)
	}
}

func TestStore_Exists_Expired_Key(t *testing.T) {
	s := newWithActiveExpiryInterval(time.Hour)
	s.Set("foo", "bar")
	s.Expire("foo", 10*time.Millisecond)
	time.Sleep(20*time.Millisecond)

	if count := s.Exists("foo"); count != 0{
			t.Errorf(`expected s.Exists("foo") count to be 0, it was %v`, count)
	}
}
