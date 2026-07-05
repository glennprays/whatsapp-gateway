package whatsapp_usecase

import (
	"errors"
	"testing"

	"github.com/glennprays/whatsapp-gateway/config"
	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
)

func TestValidateRecipient(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"empty", "", "", true},
		{"whitespace only", "   ", "", true},
		{"bare number", "6281910481554", "6281910481554@s.whatsapp.net", false},
		{"plus and spaces", "+62 819 1048 1554", "6281910481554@s.whatsapp.net", false},
		{"dashes", "62-819-1048-1554", "6281910481554@s.whatsapp.net", false},
		{"full user jid", "6281910481554@s.whatsapp.net", "6281910481554@s.whatsapp.net", false},
		{"group jid", "120363012345678901@g.us", "120363012345678901@g.us", false},
		{"lid jid", "12345@lid", "12345@lid", false},
		{"unknown server", "6281910481554@example.com", "", true},
		{"empty user with server", "@s.whatsapp.net", "", true},
		{"non-numeric bare", "not-a-number", "", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := validateRecipient(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got nil (result %q)", c.in, got)
				}
				var de errDomain.Error
				if !errors.As(err, &de) || de.ServiceError() != errDomain.ErrBadRequest {
					t.Errorf("expected ErrBadRequest domain error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", c.in, err)
			}
			if got != c.want {
				t.Errorf("validateRecipient(%q) = %q, want %q", c.in, got, c.want)
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
