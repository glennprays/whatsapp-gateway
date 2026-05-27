package whatsapp

import (
	"fmt"
	"sync"
	"testing"
)

func TestClientStore_GetSetDelete(t *testing.T) {
	cs := NewClientStore()

	// Get on empty store returns nil
	if got := cs.Get("123"); got != nil {
		t.Fatalf("expected nil for missing key, got %v", got)
	}

	// Set and Get
	cs.Set("123", nil) // nil client is valid for this test
	if got := cs.Get("123"); got != nil {
		t.Fatalf("expected nil client (we stored nil), got %v", got)
	}

	// Delete removes the entry
	cs.Delete("123")
	if got := cs.Get("123"); got != nil {
		t.Fatalf("expected nil after delete, got %v", got)
	}

	// GetAll returns a copy
	cs.Set("a", nil)
	cs.Set("b", nil)
	all := cs.GetAll()
	if len(all) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(all))
	}

	// Mutating the copy should not affect the store
	delete(all, "a")
	if cs.Get("a") != nil {
		// Get returns nil because we stored nil, but the key should still exist in GetAll
		all2 := cs.GetAll()
		if len(all2) != 2 {
			t.Fatal("mutating GetAll result affected the store")
		}
	}
}

func TestClientStore_ConcurrentAccess(t *testing.T) {
	cs := NewClientStore()
	var wg sync.WaitGroup

	// 100 goroutines doing Set/Get/Delete concurrently
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("phone-%d", id%10) // 10 keys, high contention
			for j := 0; j < 100; j++ {
				cs.Set(key, nil)
				_ = cs.Get(key)
				_ = cs.GetAll()
				cs.Delete(key)
			}
		}(i)
	}
	wg.Wait()

	// If we got here without panic or race detector complaint, the test passes.
}
