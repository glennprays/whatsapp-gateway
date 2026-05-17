package whatsapp

import (
	"fmt"
	"sync"
	"testing"
)

func mkMsg(id string, ts int64) *IncomingMessage {
	return &IncomingMessage{MessageID: id, Timestamp: ts, Type: "text"}
}

func TestIncomingBuffer_PushAndLatest(t *testing.T) {
	b := newIncomingBuffer()
	for i := 0; i < 5; i++ {
		b.Push(mkMsg(fmt.Sprintf("M%d", i), int64(i)))
	}
	got := b.Latest(3)
	if len(got) != 3 {
		t.Fatalf("expected 3, got %d", len(got))
	}
	if got[0].MessageID != "M4" || got[1].MessageID != "M3" || got[2].MessageID != "M2" {
		t.Errorf("expected newest-first M4,M3,M2; got %v %v %v", got[0].MessageID, got[1].MessageID, got[2].MessageID)
	}
}

func TestIncomingBuffer_LatestMoreThanLen(t *testing.T) {
	b := newIncomingBuffer()
	for i := 0; i < 3; i++ {
		b.Push(mkMsg(fmt.Sprintf("M%d", i), int64(i)))
	}
	got := b.Latest(100)
	if len(got) != 3 {
		t.Errorf("expected 3 (capped to len), got %d", len(got))
	}
}

func TestIncomingBuffer_Empty(t *testing.T) {
	b := newIncomingBuffer()
	if got := b.Latest(10); len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

func TestIncomingBuffer_OverflowWrap(t *testing.T) {
	b := newIncomingBuffer()
	// Push capacity+5 entries; oldest 5 should be evicted.
	total := incomingBufferCapacity + 5
	for i := 0; i < total; i++ {
		b.Push(mkMsg(fmt.Sprintf("M%d", i), int64(i)))
	}
	got := b.Latest(incomingBufferCapacity)
	if len(got) != incomingBufferCapacity {
		t.Fatalf("expected %d, got %d", incomingBufferCapacity, len(got))
	}
	if got[0].MessageID != fmt.Sprintf("M%d", total-1) {
		t.Errorf("expected newest=M%d, got %s", total-1, got[0].MessageID)
	}
	// Oldest retained should be M5 (the first surviving entry).
	if got[len(got)-1].MessageID != "M5" {
		t.Errorf("expected oldest surviving=M5, got %s", got[len(got)-1].MessageID)
	}
}

func TestIncomingBuffer_ConcurrentPushReadIsRaceFree(t *testing.T) {
	b := newIncomingBuffer()
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				b.Push(mkMsg(fmt.Sprintf("W%d-%d", id, i), int64(i)))
			}
		}(w)
	}
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				_ = b.Latest(50)
			}
		}()
	}
	wg.Wait()
	// Just confirm we can still read after the storm.
	got := b.Latest(10)
	if len(got) == 0 {
		t.Error("expected non-empty buffer after concurrent activity")
	}
}
