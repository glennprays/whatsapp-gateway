package whatsapp_usecase

import (
	"context"
	"errors"
	"testing"

	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
)

func TestResolvePresenceState(t *testing.T) {
	cases := []struct {
		in        string
		wantState string
		wantMedia string
		wantErr   bool
	}{
		{"composing", "composing", "", false},
		{"typing", "composing", "", false},
		{"recording", "composing", "audio", false},
		{"paused", "paused", "", false},
		{"stop", "paused", "", false},
		{"COMPOSING", "composing", "", false}, // case-insensitive
		{"", "", "", true},                    // empty is invalid
		{"bogus", "", "", true},
	}
	for _, c := range cases {
		state, media, err := resolvePresenceState(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("resolvePresenceState(%q): expected error", c.in)
			}
			continue
		}
		if err != nil || state != c.wantState || media != c.wantMedia {
			t.Errorf("resolvePresenceState(%q) = (%q,%q,%v), want (%q,%q,nil)", c.in, state, media, err, c.wantState, c.wantMedia)
		}
	}
}

func TestMarkRead_GroupRequiresSender(t *testing.T) {
	uc := &WhatsappMessageUsecase{}
	// A group chat without a sender must 400 before touching the (nil) manager.
	err := uc.MarkRead(context.Background(), "trace", "628111", waDomain.MarkReadRequest{
		Chat:       "120363000000000000@g.us",
		MessageIDs: []string{"MID1"},
	})
	if err == nil {
		t.Fatal("expected 400 for group mark-read without sender")
	}
	var de errDomain.Error
	if !errors.As(err, &de) || de.ServiceError() != errDomain.ErrBadRequest {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
}

func TestMarkRead_RequiresMessageIDs(t *testing.T) {
	uc := &WhatsappMessageUsecase{}
	err := uc.MarkRead(context.Background(), "trace", "628111", waDomain.MarkReadRequest{
		Chat: "6282222222222",
	})
	if err == nil {
		t.Fatal("expected 400 for empty message_ids")
	}
	var de errDomain.Error
	if !errors.As(err, &de) || de.ServiceError() != errDomain.ErrBadRequest {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
}
