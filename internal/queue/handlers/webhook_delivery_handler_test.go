package handlers

import "testing"

// TestWebhookDedupKey proves a fan-out of one event to several subscriptions
// produces a distinct dedup key per destination (so none is dropped), while a
// redelivery of the same message to the same URL collides (still deduped).
func TestWebhookDedupKey(t *testing.T) {
	const msg = "ABC"
	a := webhookDedupKey(msg, "https://a.example/hook")
	b := webhookDedupKey(msg, "https://b.example/hook")
	if a == b {
		t.Fatalf("same message to different URLs must not share a dedup key: %q == %q", a, b)
	}
	if again := webhookDedupKey(msg, "https://a.example/hook"); again != a {
		t.Fatalf("same (message,url) must produce the same key: %q != %q", again, a)
	}
	// Different events (distinct MessageID) to the same URL must not collide.
	if webhookDedupKey("XYZ", "https://a.example/hook") == a {
		t.Fatalf("different messages to the same URL must not share a dedup key")
	}
}
