package queue

import "testing"

func TestEventMatches(t *testing.T) {
	cases := []struct {
		name      string
		subEvents string
		event     string
		want      bool
	}{
		{"empty filter matches all", "", "message.incoming", true},
		{"empty filter matches session event", "", "session.logged_out", true},
		{"whitespace-only filter matches all", "   ", "message.sent", true},
		{"exact single match", "message.sent", "message.sent", true},
		{"single no match", "message.sent", "message.queued", false},
		{"multi with spaces matches", "message.sent, message.failed", "message.failed", true},
		{"multi with spaces non-member", "message.sent, message.failed", "message.queued", false},
		{"unknown-in-filter does not match and does not panic", "not.a.real.event", "message.sent", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := EventMatches(tc.subEvents, tc.event); got != tc.want {
				t.Fatalf("EventMatches(%q, %q) = %v, want %v", tc.subEvents, tc.event, got, tc.want)
			}
		})
	}
}

func TestIsKnownEvent(t *testing.T) {
	known := []string{
		"message.incoming", "message.queued", "message.sent", "message.failed",
		"session.logged_out", "session.banned", "session.connect_failure",
		"session.connected", "session.disconnected", "session.replaced",
	}
	for _, e := range known {
		if !IsKnownEvent(e) {
			t.Fatalf("IsKnownEvent(%q) = false, want true", e)
		}
	}
	unknown := []string{"", "message.read", "session.unknown", "garbage", "message.sent ", " message.sent"}
	for _, e := range unknown {
		if IsKnownEvent(e) {
			t.Fatalf("IsKnownEvent(%q) = true, want false", e)
		}
	}
}

// TestDispatcherSelectionSet exercises the pure fan-out selection: given a set
// of subscriptions with different event filters, dispatching an event selects
// exactly those whose filter matches. This is the load-bearing dispatch logic
// tested without any live client or sender.
func TestDispatcherSelectionSet(t *testing.T) {
	type sub struct {
		url    string
		events string
	}
	subs := []sub{
		{"https://all.example", ""}, // all events
		{"https://incoming.example", "message.incoming"},
		{"https://sent.example", "message.sent"},
	}
	selected := func(event string) []string {
		var urls []string
		for _, s := range subs {
			if EventMatches(s.events, event) {
				urls = append(urls, s.url)
			}
		}
		return urls
	}

	got := selected("message.incoming")
	want := []string{"https://all.example", "https://incoming.example"}
	if !equalStrings(got, want) {
		t.Fatalf("message.incoming selected %v, want %v", got, want)
	}

	got = selected("session.logged_out")
	want = []string{"https://all.example"}
	if !equalStrings(got, want) {
		t.Fatalf("session.logged_out selected %v, want %v", got, want)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
