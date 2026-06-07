package queue

import (
	"testing"
	"time"
)

func TestDedupCache_MarkThenDuplicate(t *testing.T) {
	d := NewDedupCache(time.Minute)

	if d.IsDuplicate("q", "msg-1") {
		t.Fatal("unmarked id must not be a duplicate")
	}

	d.MarkProcessed("q", "msg-1")

	if !d.IsDuplicate("q", "msg-1") {
		t.Fatal("marked id must be a duplicate")
	}
}

func TestDedupCache_ScopesAreIsolated(t *testing.T) {
	d := NewDedupCache(time.Minute)

	d.MarkProcessed("incoming", "msg-1")

	if d.IsDuplicate("webhooks", "msg-1") {
		t.Fatal("same id in a different scope must not be a duplicate")
	}
}

func TestDedupCache_EntriesExpire(t *testing.T) {
	d := NewDedupCache(20 * time.Millisecond)

	d.MarkProcessed("q", "msg-1")
	time.Sleep(40 * time.Millisecond)

	if d.IsDuplicate("q", "msg-1") {
		t.Fatal("expired entry must not be a duplicate")
	}
}

func TestDedupCache_EmptyIDNeverDuplicate(t *testing.T) {
	d := NewDedupCache(time.Minute)

	d.MarkProcessed("q", "")

	if d.IsDuplicate("q", "") {
		t.Fatal("empty id must never be a duplicate")
	}
}

func TestDedupCache_NilReceiverIsSafe(t *testing.T) {
	var d *DedupCache

	d.MarkProcessed("q", "msg-1") // must not panic

	if d.IsDuplicate("q", "msg-1") {
		t.Fatal("nil cache must never report duplicates")
	}
}
