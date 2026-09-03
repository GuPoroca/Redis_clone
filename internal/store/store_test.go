package store

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// Worth noting: this one isn't table-driven, on purpose. Table-driven
// shines when you have many independent input->output cases that
// don't depend on each other. This test is inherently a *sequence*
// (Set, then Get, and the second call's correctness depends on the
// first having happened) — forcing that into a table would just make
// it harder to read for no real benefit. Use the shape that fits the
// test, not table-driven-everywhere as a rule.
func TestStore_SetGet(t *testing.T) {
	s := New()

	s.Set("foo", "bar")

	got, ok := s.Get("foo")
	if !ok {
		t.Fatalf(`Get("foo") ok = false, want true`)
	}
	if got != "bar" {
		t.Errorf(`Get("foo") = %q, want "bar"`, got)
	}
}

func TestStore_Get_MissingKey(t *testing.T) {
	s := New()

	_, ok := s.Get("missing")
	if ok {
		t.Error(`Get("missing") ok = true, want false`)
	}
}

// This one's worth having specifically because Set's documented
// behavior is "overwrite" — a bug that made Set only work for new
// keys (e.g. an accidental "if key doesn't already exist" guard)
// wouldn't be caught by TestStore_SetGet alone.
func TestStore_Set_Overwrite(t *testing.T) {
	s := New()

	s.Set("foo", "bar")
	s.Set("foo", "baz")

	got, _ := s.Get("foo")
	if got != "baz" {
		t.Errorf(`Get("foo") after overwrite = %q, want "baz"`, got)
	}
}

func TestStore_Del(t *testing.T) {
	s := New()
	s.Set("a", "1")
	s.Set("b", "2")

	// "c" deliberately doesn't exist — Del's real Redis semantics are
	// "delete whichever of these exist, tell me how many actually
	// were." Only testing all-existing keys wouldn't prove that part.
	count := s.Del("a", "b", "c")
	if count != 2 {
		t.Errorf("Del(a, b, c) = %d, want 2", count)
	}

	if _, ok := s.Get("a"); ok {
		t.Error(`expected "a" to be deleted, but Get still found it`)
	}
}

func TestStore_Exists(t *testing.T) {
	s := New()
	s.Set("a", "1")

	count := s.Exists("a", "b") // "b" doesn't exist
	if count != 1 {
		t.Errorf("Exists(a, b) = %d, want 1", count)
	}
}

func TestStore_Persist_Key_With_Expiry(t *testing.T){
  s := New()
	s.Set("foo", "bar")
	s.Expire("foo", time.Second)

	remaining, exists, hasExpiry := s.TTL("foo")
	if !exists || !hasExpiry {
		t.Fatalf(`TTL("foo") exists=%v hasExpiry=%v, want true, true`, exists, hasExpiry)
	}
	if remaining <= 0 || remaining > time.Second {
			t.Errorf("TTL(\"foo\") remaining = %v, want (0, 1s]", remaining)
	}

	if ok := s.Persist("foo"); !ok {
		t.Fatalf("expected s.Persist to return true, returned false")
	}

	_, exists, hasExpiry = s.TTL("foo")
	if !exists || hasExpiry {
		t.Fatalf(`TTL("foo") exists=%v hasExpiry=%v, want true, false`, exists, hasExpiry)
		}
	}


func TestStore_Persist_Unexistent_Key(t *testing.T){
	s := New()
	s.Expire("foo", time.Second)
	if ok := s.Persist("bar"); ok {
		t.Fatalf("expected s.Persist to return false, returned true")
	}
}

func TestStore_Persist_Key_With_No_Expiry(t *testing.T){
	s := New()
	s.Set("foo", "bar")
	if ok := s.Persist("foo"); ok {
		t.Fatalf("expected s.Persist to return false, returned true")
	}
}

func TestStore_Persist_With_Expired_Key(t *testing.T){
	s := newWithActiveExpiryInterval(time.Hour)
	s.Set("foo", "bar")
	s.Expire("foo", 10*time.Millisecond)
	time.Sleep(20*time.Millisecond)
	if ok := s.Persist("foo"); ok {
		t.Fatalf("expected s.Persist to return false, returned true")
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

func TestStore_TTL(t *testing.T) {
	s := New()
	s.Set("foo", "bar")
	s.Expire("foo", time.Second)
	remaining, exists, hasExpiry := s.TTL("foo")
	if !exists || !hasExpiry {
		t.Fatalf(`TTL("foo") exists=%v hasExpiry=%v, want true, true`, exists, hasExpiry)
		// Not asserting an exact duration (that's inherently flaky — real
		// time passes between Expire and TTL, if only by nanoseconds) but
		// checking it's in a sane range catches a real class of bug, e.g.
		// accidentally returning the full TTL instead of what's *left*,
		// or a sign error making it negative.
		if remaining <= 0 || remaining > time.Second {
			t.Errorf("TTL(\"foo\") remaining = %v, want (0, 1s]", remaining)
		}
	}
}

func TestStore_GetSet_Concurrency(t *testing.T) {
	s := New()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", i)
			s.Set(key, "bar")
			if _, ok := s.Get(key); !ok {
				t.Errorf("expected %q to be found, but Get didn't find it", key)
			}
		}(i)
	}
	wg.Wait()
}
