package store

import (
	"path/filepath"
	"testing"
	"time"
	"os"
)
 
func TestStore_SaveLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dump.json")
 
	s := New()
	s.Set("permanent", "value1")
	s.Set("temporary", "value2")
	s.Expire("temporary", time.Minute)
 
	if err := s.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile: %v", err)
	}
 
	// A fresh Store, not the one we saved from — this is what makes
	// the test actually prove the data came back FROM DISK, rather
	// than just still being present in the original store's memory
	// regardless of whether loading did anything at all.
	loaded := New()
	if err := loaded.LoadFromFile(path); err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
 
	if got, ok := loaded.Get("permanent"); !ok || got != "value1" {
		t.Errorf(`Get("permanent") = %q, ok=%v; want "value1", true`, got, ok)
	}
	if got, ok := loaded.Get("temporary"); !ok || got != "value2" {
		t.Errorf(`Get("temporary") = %q, ok=%v; want "value2", true`, got, ok)
	}
 
	// The value coming back isn't the whole story — the TTL itself
	// has to survive the round trip too, or a restarted server would
	// silently make every key permanent.
	if _, exists, hasExpiry := loaded.TTL("temporary"); !exists || !hasExpiry {
		t.Errorf(`TTL("temporary") exists=%v hasExpiry=%v, want true, true`, exists, hasExpiry)
	}
	if _, _, hasExpiry := loaded.TTL("permanent"); hasExpiry {
		t.Error(`expected "permanent" to have no expiry after round trip`)
	}
}
 
func TestStore_Load_From_Unexistent_File(t *testing.T){
	dir := t.TempDir()
	path := filepath.Join(dir, "dump.json")
 
	loaded := New()
	if err := loaded.LoadFromFile(path); err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
}

func TestStore_Load_From_Corrupt_Content(t *testing.T){
	dir := t.TempDir()
	path := filepath.Join(dir, "dump.json")
	content := "invalid json"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write invalid file error: %v", err)
	}

	loaded := New()
	if err := loaded.LoadFromFile(path); err == nil {
		t.Fatalf("LoadFromFile didnt error, expected to error")
	}


}
