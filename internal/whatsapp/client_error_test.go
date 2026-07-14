package whatsapp

import (
	"errors"
	"fmt"
	"testing"

	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	"go.mau.fi/whatsmeow"
)

// TestMapWhatsmeowErr_IQError covers the errors.As(*IQError) branch that maps a
// raw IQ error (as returned by GetSubGroups / GetLinkedGroupsParticipants) to a
// domain error by its server code, and confirms a wrapped ErrGroupNotFound
// sentinel still wins (it is checked before the generic IQError branch).
func TestMapWhatsmeowErr_IQError(t *testing.T) {
	c := &client{}
	cases := []struct {
		name string
		err  error
		want error
	}{
		{"bad request 400", &whatsmeow.IQError{Code: 400, Text: "bad-request"}, errDomain.ErrBadRequest},
		{"not allowed 405", &whatsmeow.IQError{Code: 405, Text: "not-allowed"}, errDomain.ErrBadRequest},
		{"not authorized 401", &whatsmeow.IQError{Code: 401, Text: "not-authorized"}, errDomain.ErrForbidden},
		{"forbidden 403", &whatsmeow.IQError{Code: 403, Text: "forbidden"}, errDomain.ErrForbidden},
		{"not found 404", &whatsmeow.IQError{Code: 404, Text: "item-not-found"}, errDomain.ErrNotFound},
		{"rate over limit 429", &whatsmeow.IQError{Code: 429, Text: "rate-overlimit"}, errDomain.ErrTooManyRequests},
		{"wrapped 404", fmt.Errorf("subgroups: %w", &whatsmeow.IQError{Code: 404}), errDomain.ErrNotFound},
		{"unknown code 500 -> internal", &whatsmeow.IQError{Code: 500, Text: "internal-server-error"}, errDomain.ErrInternalFailure},
		{"wrapped group-not-found sentinel wins", fmt.Errorf("wrap: %w", whatsmeow.ErrGroupNotFound), errDomain.ErrNotFound},
		{"not-in-group sentinel", fmt.Errorf("wrap: %w", whatsmeow.ErrNotInGroup), errDomain.ErrForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mapped := c.mapWhatsmeowErr("trace", "628111", tc.err)
			var de errDomain.Error
			if !errors.As(mapped, &de) {
				t.Fatalf("expected errDomain.Error, got %v", mapped)
			}
			if de.ServiceError() != tc.want {
				t.Fatalf("code %v: got %v, want %v", tc.err, de.ServiceError(), tc.want)
			}
		})
	}
}

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
