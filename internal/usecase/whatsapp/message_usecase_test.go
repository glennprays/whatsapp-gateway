package whatsapp_usecase

import (
	"errors"
	"testing"

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
