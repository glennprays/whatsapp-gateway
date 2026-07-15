package whatsapp

import (
	"testing"
	"time"

	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
)

func TestDeriveItem(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	tests := []struct {
		name       string
		phone      string
		inMem      map[string]clientState
		devices    map[string]time.Time
		status     map[string]waDomain.SessionStatus
		wantState  string
		wantSource string
		wantBan    bool
	}{
		{
			name:       "never_paired",
			phone:      "628001",
			inMem:      map[string]clientState{"628001": {paired: false}},
			wantState:  "never_paired",
			wantSource: "in-memory",
		},
		{
			name:       "connected",
			phone:      "628002",
			inMem:      map[string]clientState{"628002": {paired: true, connected: true, loggedIn: true}},
			wantState:  "connected",
			wantSource: "in-memory",
		},
		{
			name:       "in_memory_disconnected",
			phone:      "628003",
			inMem:      map[string]clientState{"628003": {paired: true, connected: false, loggedIn: false}},
			wantState:  "disconnected",
			wantSource: "in-memory",
		},
		{
			name:       "device_row_only",
			phone:      "628004",
			devices:    map[string]time.Time{"628004": now},
			wantState:  "disconnected",
			wantSource: "store",
		},
		{
			name:       "logged_out_wins_over_client",
			phone:      "628005",
			inMem:      map[string]clientState{"628005": {paired: true, connected: true, loggedIn: true}},
			status:     map[string]waDomain.SessionStatus{"628005": {State: "logged_out", Reason: "stream_error", UpdatedAt: past}},
			wantState:  "logged_out",
			wantSource: "store",
		},
		{
			name:       "banned_unexpired_wins_over_client",
			phone:      "628006",
			inMem:      map[string]clientState{"628006": {paired: true, connected: true, loggedIn: true}},
			status:     map[string]waDomain.SessionStatus{"628006": {State: "banned", Reason: "temp", BanExpiresAt: &future, UpdatedAt: past}},
			wantState:  "banned",
			wantSource: "store",
			wantBan:    true,
		},
		{
			name:       "banned_expired_downgrades",
			phone:      "628007",
			status:     map[string]waDomain.SessionStatus{"628007": {State: "banned", BanExpiresAt: &past, UpdatedAt: past}},
			wantState:  "disconnected",
			wantSource: "store",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := deriveItem(tc.phone, tc.inMem, tc.devices, tc.status, now)
			if got.State != tc.wantState {
				t.Fatalf("state = %q, want %q", got.State, tc.wantState)
			}
			if got.Source != tc.wantSource {
				t.Fatalf("source = %q, want %q", got.Source, tc.wantSource)
			}
			if (got.BanExpiresAt != nil) != tc.wantBan {
				t.Fatalf("ban_expires_at present = %v, want %v", got.BanExpiresAt != nil, tc.wantBan)
			}
			if got.PhoneMasked != MaskedPhoneNumber(tc.phone) {
				t.Fatalf("phone not masked: %q", got.PhoneMasked)
			}
		})
	}
}

func TestMergeInventorySortedAndDeduped(t *testing.T) {
	now := time.Now()
	// Distinct numbers whose masked forms also differ.
	const live, deviceOnly, gone = "6281111111", "6282222222", "6283333333"
	inMem := map[string]clientState{live: {paired: true, connected: true, loggedIn: true}}
	devices := map[string]time.Time{live: now, deviceOnly: now} // live also has a client row
	status := map[string]waDomain.SessionStatus{gone: {State: "logged_out", UpdatedAt: now}}

	items := mergeInventory(inMem, devices, status, now)
	if len(items) != 3 {
		t.Fatalf("want 3 unique accounts, got %d", len(items))
	}
	// Deterministic, ascending order (by unmasked phone).
	if items[0].PhoneMasked > items[1].PhoneMasked || items[1].PhoneMasked > items[2].PhoneMasked {
		t.Fatalf("not sorted: %v", items)
	}
	// The live account has an in-memory client -> in-memory source wins over its device row.
	for _, it := range items {
		if it.PhoneMasked == MaskedPhoneNumber(live) && it.Source != "in-memory" {
			t.Fatalf("live source = %q, want in-memory", it.Source)
		}
	}
}
