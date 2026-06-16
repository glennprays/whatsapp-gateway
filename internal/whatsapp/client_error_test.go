package whatsapp

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsRecipientError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"unknown server", errors.New("can't send message to unknown server "), true},
		{"wrapped unknown server", fmt.Errorf("failed to send message: %w", errors.New("can't send message to unknown server")), true},
		{"invalid jid", errors.New("invalid JID format"), true},
		{"recipient", errors.New("recipient is invalid"), true},
		{"server fault", errors.New("connection reset by peer"), false},
		{"timeout", errors.New("context deadline exceeded"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isRecipientError(c.err); got != c.want {
				t.Errorf("isRecipientError(%q) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}
