package whatsapp

import (
	"errors"
	"testing"
	"time"

	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// The group-mutation named sentinels and the extra IQ codes (409/423/410/419)
// must map to the right domain error — the invite/photo sentinels win over the
// generic IQ fallback for their specific human message.
func TestMapWhatsmeowErr_GroupMutationSentinels(t *testing.T) {
	c := &client{}
	cases := []struct {
		name string
		err  error
		want error
	}{
		{"invalid image -> bad request", whatsmeow.ErrInvalidImageFormat, errDomain.ErrBadRequest},
		{"invite revoked -> gone", whatsmeow.ErrInviteLinkRevoked, errDomain.ErrGone},
		{"invite invalid -> bad request", whatsmeow.ErrInviteLinkInvalid, errDomain.ErrBadRequest},
		{"invite unauthorized -> forbidden", whatsmeow.ErrGroupInviteLinkUnauthorized, errDomain.ErrForbidden},
		{"IQ 410 gone -> gone", &whatsmeow.IQError{Code: 410, Text: "gone"}, errDomain.ErrGone},
		{"IQ 409 conflict -> conflict", &whatsmeow.IQError{Code: 409, Text: "conflict"}, errDomain.ErrConflict},
		{"IQ 423 locked -> conflict", &whatsmeow.IQError{Code: 423, Text: "locked"}, errDomain.ErrConflict},
		{"IQ 419 resource-limit -> too many", &whatsmeow.IQError{Code: 419}, errDomain.ErrTooManyRequests},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mapped := c.mapWhatsmeowErr("trace", "628111", tc.err)
			var de errDomain.Error
			if !errors.As(mapped, &de) {
				t.Fatalf("expected errDomain.Error, got %v", mapped)
			}
			if de.ServiceError() != tc.want {
				t.Fatalf("got %v, want %v", de.ServiceError(), tc.want)
			}
		})
	}
}

func TestMapIQError(t *testing.T) {
	if _, ok := mapIQError(errors.New("plain error")); ok {
		t.Fatal("a non-IQError must return ok=false")
	}
	cases := map[int]error{
		400: errDomain.ErrBadRequest,
		401: errDomain.ErrForbidden, // authenticated caller, group-role issue -> 403 (not 401)
		403: errDomain.ErrForbidden,
		404: errDomain.ErrNotFound,
		406: errDomain.ErrBadRequest,
		409: errDomain.ErrConflict,
		410: errDomain.ErrGone,
		419: errDomain.ErrTooManyRequests,
		429: errDomain.ErrTooManyRequests,
		500: errDomain.ErrInternalFailure,
	}
	for code, want := range cases {
		mapped, ok := mapIQError(&whatsmeow.IQError{Code: code})
		if !ok {
			t.Fatalf("code %d: expected ok=true", code)
		}
		var de errDomain.Error
		if !errors.As(mapped, &de) || de.ServiceError() != want {
			t.Fatalf("code %d: got %v, want %v", code, mapped, want)
		}
	}
}

// A batch mutation returns one response carrying every per-participant outcome:
// success, privacy-blocked-add-turned-invite, and hard failure — all at once.
func TestParticipantResults_MixedOutcomes(t *testing.T) {
	exp := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	ps := []types.GroupParticipant{
		{JID: types.NewJID("628001", types.DefaultUserServer)},
		{
			JID:        types.NewJID("628002", types.DefaultUserServer),
			Error:      403,
			AddRequest: &types.GroupParticipantAddRequest{Code: "inv-code", Expiration: exp},
		},
		{JID: types.NewJID("628003", types.DefaultUserServer), Error: 404},
	}
	got := participantResults(ps)
	if len(got) != 3 {
		t.Fatalf("expected 3 results, got %d", len(got))
	}
	if got[0].Status != "ok" || got[0].Code != 0 || got[0].Invite != nil {
		t.Fatalf("result[0] (ok) wrong: %+v", got[0])
	}
	if got[1].Status != "invited" || got[1].Code != 403 || got[1].Invite == nil ||
		got[1].Invite.Code != "inv-code" || !got[1].Invite.ExpiresAt.Equal(exp) {
		t.Fatalf("result[1] (invited) wrong: %+v", got[1])
	}
	if got[2].Status != "failed" || got[2].Code != 404 || got[2].Invite != nil {
		t.Fatalf("result[2] (failed) wrong: %+v", got[2])
	}
}
