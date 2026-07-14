package whatsapp

import (
	"testing"
	"time"

	"go.mau.fi/whatsmeow/types/events"
)

// TestSessionEventPayloadMapping locks the struct->payload mapping for the rich
// session events against manually constructed whatsmeow event values (no socket,
// no live client). If whatsmeow renames a field or the mapping drifts, this
// fails loudly.
func TestSessionEventPayloadMapping(t *testing.T) {
	const phone = "628111"
	const jid = "628111@s.whatsapp.net"

	t.Run("temporary_ban", func(t *testing.T) {
		v := &events.TemporaryBan{Code: events.TempBanSentTooManySameMessage, Expire: 2 * time.Hour}
		p := bannedPayload(phone, jid, v)
		if p["event"] != "session.banned" {
			t.Fatalf("event = %v", p["event"])
		}
		if p["code"] != 104 {
			t.Fatalf("code = %v, want 104", p["code"])
		}
		if p["reason_text"] != v.Code.String() {
			t.Fatalf("reason_text = %v, want %v", p["reason_text"], v.Code.String())
		}
		if p["expires_in"] != 7200 {
			t.Fatalf("expires_in = %v, want 7200", p["expires_in"])
		}
	})

	t.Run("logged_out_on_connect", func(t *testing.T) {
		v := &events.LoggedOut{OnConnect: true, Reason: events.ConnectFailureLoggedOut}
		p := loggedOutPayload(phone, jid, v)
		if p["event"] != "session.logged_out" {
			t.Fatalf("event = %v", p["event"])
		}
		if p["on_connect"] != true {
			t.Fatalf("on_connect = %v, want true", p["on_connect"])
		}
		if p["reason"] != 401 {
			t.Fatalf("reason = %v, want 401", p["reason"])
		}
		if p["reason_text"] != v.Reason.String() {
			t.Fatalf("reason_text = %v", p["reason_text"])
		}
	})

	t.Run("connect_failure", func(t *testing.T) {
		v := &events.ConnectFailure{Reason: events.ConnectFailureServiceUnavailable, Message: "boom"}
		p := connectFailurePayload(phone, jid, v)
		if p["event"] != "session.connect_failure" {
			t.Fatalf("event = %v", p["event"])
		}
		if p["reason"] != 503 {
			t.Fatalf("reason = %v, want 503", p["reason"])
		}
		if p["message"] != "boom" {
			t.Fatalf("message = %v, want boom", p["message"])
		}
	})

	t.Run("base_envelope", func(t *testing.T) {
		p := baseSessionPayload("session.connected", phone, jid)
		if p["event"] != "session.connected" || p["phone_number"] != phone || p["jid"] != jid {
			t.Fatalf("base envelope wrong: %+v", p)
		}
		if _, ok := p["timestamp"]; !ok {
			t.Fatal("base envelope missing timestamp")
		}
	})
}
