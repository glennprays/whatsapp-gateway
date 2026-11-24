package whatsapp

import (
	"testing"
	"time"
)

// TestGetCurrentQR_AbsentPhone: a phone with no cached QR yields nil.
func TestGetCurrentQR_AbsentPhone(t *testing.T) {
	cs := NewClientStore()
	if qr := cs.GetCurrentQR("628111"); qr != nil {
		t.Fatalf("expected nil for absent phone, got %+v", qr)
	}
}

// TestGetCurrentQR_Expiry: the getter must treat an expired entry as nil —
// callers never re-check expiry themselves.
func TestGetCurrentQR_Expiry(t *testing.T) {
	cs := NewClientStore()
	cs.SetCurrentQR("628111", currentQR{
		png:       "data:image/png;base64,expired",
		timeout:   60,
		expiresAt: time.Now().Add(-time.Second),
	})
	if qr := cs.GetCurrentQR("628111"); qr != nil {
		t.Fatalf("expected nil for expired QR, got %+v", qr)
	}
}

// TestGetCurrentQR_HitBeforeExpiry: a stored, unexpired QR is returned
// verbatim so repeated requests serve the same live code.
func TestGetCurrentQR_HitBeforeExpiry(t *testing.T) {
	cs := NewClientStore()
	want := currentQR{
		png:       "data:image/png;base64,live",
		timeout:   60,
		expiresAt: time.Now().Add(time.Minute),
	}
	cs.SetCurrentQR("628111", want)
	got := cs.GetCurrentQR("628111")
	if got == nil {
		t.Fatal("expected cached QR before expiry, got nil")
	}
	if got.png != want.png || got.timeout != want.timeout || !got.expiresAt.Equal(want.expiresAt) {
		t.Fatalf("stored QR mutated: got %+v, want %+v", got, want)
	}
}

// TestDeleteCurrentQR: deletion removes the entry (next lookup is a miss).
func TestDeleteCurrentQR(t *testing.T) {
	cs := NewClientStore()
	cs.SetCurrentQR("628111", currentQR{png: "x", timeout: 60, expiresAt: time.Now().Add(time.Minute)})
	cs.DeleteCurrentQR("628111")
	if qr := cs.GetCurrentQR("628111"); qr != nil {
		t.Fatalf("expected nil after delete, got %+v", qr)
	}
}
