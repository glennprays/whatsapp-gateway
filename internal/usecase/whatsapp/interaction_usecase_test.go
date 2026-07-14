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

func TestBuildMessageContext_ResolvesJIDs(t *testing.T) {
	uc := &WhatsappMessageUsecase{}
	mc, err := uc.buildMessageContext("MID123", "6289999999999", "hi there", []string{"6281111111111", "  ", "6282222222222"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mc.ReplyToID != "MID123" || mc.ReplyToText != "hi there" {
		t.Fatalf("reply passthrough wrong: %+v", mc)
	}
	if mc.ReplyToSender != "6289999999999@s.whatsapp.net" {
		t.Fatalf("sender should resolve to a user JID, got %q", mc.ReplyToSender)
	}
	// Blank mention is skipped; the two real ones resolve to JIDs.
	if len(mc.Mentions) != 2 || mc.Mentions[0] != "6281111111111@s.whatsapp.net" {
		t.Fatalf("mentions resolution wrong: %v", mc.Mentions)
	}
	if mc.IsEmpty() {
		t.Fatal("context with a reply must not be empty")
	}
}

func TestBuildMessageContext_EmptyIsEmpty(t *testing.T) {
	uc := &WhatsappMessageUsecase{}
	mc, err := uc.buildMessageContext("", "", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !mc.IsEmpty() {
		t.Fatalf("no reply + no mentions should be empty, got %+v", mc)
	}
}

func TestBuildMessageContext_InvalidMention(t *testing.T) {
	uc := &WhatsappMessageUsecase{}
	if _, err := uc.buildMessageContext("", "", "", []string{"@@@bogus@@@"}); err == nil {
		t.Fatal("expected error for an unparseable mention")
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
