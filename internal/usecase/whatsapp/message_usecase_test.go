package whatsapp_usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/glennprays/whatsapp-gateway/config"
	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
)

// assertBadRequest fails unless err is a domain ErrBadRequest.
func assertBadRequest(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected a 400 error")
	}
	var de errDomain.Error
	if !errors.As(err, &de) || de.ServiceError() != errDomain.ErrBadRequest {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
}

// An invalid mention must 400 in buildMessageContext before SendImageMessage
// touches the (nil) fileHeader/manager — guards that buildMessageContext runs
// ahead of the file read.
func TestSendImage_InvalidMention400(t *testing.T) {
	uc := &WhatsappMessageUsecase{}
	_, _, err := uc.SendImageMessage(context.Background(), "trace", "628111",
		waDomain.SendImageMessageRequest{
			Chat:     "6282222222222",
			Mentions: []string{"@@@bad@@@"},
		}, nil, false)
	assertBadRequest(t, err)
}

// Poll is the JSON path: an invalid mention must 400 before the (nil)
// queue/limiter/manager is touched — guards buildMessageContext ordering ahead
// of the queue branch.
func TestSendPoll_InvalidMention400(t *testing.T) {
	uc := &WhatsappMessageUsecase{}
	_, _, err := uc.SendPollMessage(context.Background(), "trace", "628111",
		waDomain.SendPollMessageRequest{
			Chat:     "6282222222222",
			Question: "q",
			Options:  []string{"a", "b"},
			Mentions: []string{"@@@bad@@@"},
		})
	assertBadRequest(t, err)
}

func TestResolveChat(t *testing.T) {
	cases := []struct {
		name    string
		chat    string
		msisdn  string
		want    string
		wantErr bool
	}{
		// Requiredness (both empty).
		{"both empty", "", "", "", true},
		{"whitespace only", "   ", "  ", "", true},
		// msisdn fallback (chat empty) — legacy behavior preserved.
		{"msisdn bare number", "", "6281910481554", "6281910481554@s.whatsapp.net", false},
		{"msisdn plus and spaces", "", "+62 819 1048 1554", "6281910481554@s.whatsapp.net", false},
		{"msisdn dashes", "", "62-819-1048-1554", "6281910481554@s.whatsapp.net", false},
		// chat forms.
		{"chat bare number", "6281910481554", "", "6281910481554@s.whatsapp.net", false},
		{"chat pn jid", "6281910481554@s.whatsapp.net", "", "6281910481554@s.whatsapp.net", false},
		{"chat group jid", "120363012345678901@g.us", "", "120363012345678901@g.us", false},
		{"chat lid jid", "12345@lid", "", "12345@lid", false},
		// chat wins over msisdn (precedence).
		{"chat wins over msisdn", "120363012345678901@g.us", "6281910481554", "120363012345678901@g.us", false},
		// Normalization: device/agent suffix stripped, server lowercased.
		{"device suffix stripped", "6281910481554:2@s.whatsapp.net", "", "6281910481554@s.whatsapp.net", false},
		{"uppercase server", "6281910481554@S.WHATSAPP.NET", "", "6281910481554@s.whatsapp.net", false},
		// Rejections.
		{"unknown server", "6281910481554@example.com", "", "", true},
		{"broadcast dropped", "1234567890@broadcast", "", "", true},
		{"empty user with server", "@s.whatsapp.net", "", "", true},
		{"non-numeric user jid", "12ab34@s.whatsapp.net", "", "", true},
		{"non-numeric bare", "not-a-number", "", "", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := resolveChat(c.chat, c.msisdn)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error for chat=%q msisdn=%q, got nil (result %q)", c.chat, c.msisdn, got)
				}
				var de errDomain.Error
				if !errors.As(err, &de) || de.ServiceError() != errDomain.ErrBadRequest {
					t.Errorf("expected ErrBadRequest domain error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for chat=%q msisdn=%q: %v", c.chat, c.msisdn, err)
			}
			if got != c.want {
				t.Errorf("resolveChat(%q, %q) = %q, want %q", c.chat, c.msisdn, got, c.want)
			}
		})
	}
}

func TestValidateMediaMime(t *testing.T) {
	cases := []struct {
		name    string
		kind    string
		mime    string
		wantErr bool
	}{
		{"image jpeg", "image", "image/jpeg", false},
		{"image png", "image", "image/png", false},
		{"image bad", "image", "application/octet-stream", true},
		{"sticker webp", "sticker", "image/webp", false},
		{"sticker png rejected", "sticker", "image/png", true},
		{"audio mpeg", "audio", "audio/mpeg", false},
		{"audio empty allowed (PTT)", "audio", "", false},
		{"video mp4", "video", "video/mp4", false},
		{"video mkv rejected", "video", "video/x-matroska", true},
		{"document unrestricted", "document", "application/zip", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateMediaMime(c.kind, c.mime)
			if (err != nil) != c.wantErr {
				t.Errorf("validateMediaMime(%q,%q) err=%v, wantErr=%v", c.kind, c.mime, err, c.wantErr)
			}
		})
	}
}

func TestValidateMediaSize(t *testing.T) {
	t.Run("disabled when zero", func(t *testing.T) {
		uc := &WhatsappMessageUsecase{config: &config.Config{MaxUploadBytes: 0}}
		if err := uc.validateMediaSize(1 << 30); err != nil {
			t.Errorf("disabled cap should allow any size, got %v", err)
		}
	})
	t.Run("rejects over cap", func(t *testing.T) {
		uc := &WhatsappMessageUsecase{config: &config.Config{MaxUploadBytes: 1024}}
		if err := uc.validateMediaSize(2048); err == nil {
			t.Error("expected size violation, got nil")
		}
	})
	t.Run("allows under cap", func(t *testing.T) {
		uc := &WhatsappMessageUsecase{config: &config.Config{MaxUploadBytes: 1024}}
		if err := uc.validateMediaSize(512); err != nil {
			t.Errorf("expected ok for small upload, got %v", err)
		}
	})
}
