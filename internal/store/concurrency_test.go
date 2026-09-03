package store

import (
	"path/filepath"
	"fmt"
	"sync"
	"testing"
)

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


func TestStore_SaveMutex_Concurrent_Save(t *testing.T) {
	s := New()
	dir := t.TempDir()
	path := filepath.Join(dir, "dump.json")
 
	s.Set("a", "1")
	s.Set("b", "2")
	s.Set("c", "3")
 
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.SaveToFile(path); err != nil {
				t.Errorf("SaveToFile: %v", err)
			}
		}()
	}
	wg.Wait()
 
	loaded := New()
	if err := loaded.LoadFromFile(path); err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
 
	for key, want := range map[string]string{"a": "1", "b": "2", "c": "3"} {
		if got, ok := loaded.Get(key); !ok || got != want {
			t.Errorf("Get(%q) = %q, ok=%v; want %q, true", key, got, ok, want)
		}
	}
}
