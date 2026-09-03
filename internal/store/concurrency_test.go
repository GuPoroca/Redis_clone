package store

import (
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
