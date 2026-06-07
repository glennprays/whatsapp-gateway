package queue

import (
	"sync"
	"time"
)

// DedupCache is an in-memory TTL cache used to suppress duplicate processing
// of redelivered queue messages (the queue guarantees at-least-once delivery,
// so handlers can see the same message twice after crashes or requeues).
//
// Entries are recorded only after successful processing (MarkProcessed) so
// scheduled retries of failed messages are never mistaken for duplicates.
// Keys are scoped per queue because the same message ID legitimately flows
// through multiple queues (e.g. incoming events then webhook delivery).
//
// LIMITATION: the cache is process-local. It does not survive restarts and is
// not shared across replicas; duplicates can still occur in those windows.
type DedupCache struct {
	mu        sync.Mutex
	entries   map[string]time.Time // key -> expiry
	ttl       time.Duration
	nextSweep time.Time
}

// NewDedupCache creates a cache whose entries expire after ttl.
func NewDedupCache(ttl time.Duration) *DedupCache {
	return &DedupCache{
		entries:   make(map[string]time.Time),
		ttl:       ttl,
		nextSweep: time.Now().Add(ttl),
	}
}

// IsDuplicate reports whether (scope, id) was marked processed within the
// TTL. Empty ids and nil caches are never duplicates, so callers need no
// guards.
func (d *DedupCache) IsDuplicate(scope, id string) bool {
	if d == nil || id == "" {
		return false
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	d.sweepLocked()

	expiry, ok := d.entries[scope+":"+id]
	return ok && time.Now().Before(expiry)
}

// MarkProcessed records that (scope, id) was processed successfully.
func (d *DedupCache) MarkProcessed(scope, id string) {
	if d == nil || id == "" {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	d.sweepLocked()
	d.entries[scope+":"+id] = time.Now().Add(d.ttl)
}

// sweepLocked lazily evicts expired entries at most once per TTL window,
// avoiding a dedicated janitor goroutine. Caller must hold d.mu.
func (d *DedupCache) sweepLocked() {
	now := time.Now()
	if now.Before(d.nextSweep) {
		return
	}
	for key, expiry := range d.entries {
		if now.After(expiry) {
			delete(d.entries, key)
		}
	}
	d.nextSweep = now.Add(d.ttl)
}
