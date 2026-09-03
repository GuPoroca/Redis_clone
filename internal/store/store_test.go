package store

import (
	"testing"
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
