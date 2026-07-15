package whatsapp

import "time"

// SessionStatus is the persisted lifecycle record for one account, written from
// whatsmeow events (logged out, banned, connect failure). It deliberately
// outlives the whatsmeow_device row (no FK) so a dead session is still
// reportable after whatsmeow deletes the device on logout.
type SessionStatus struct {
	State        string // connected|disconnected|never_paired|logged_out|banned
	Reason       string
	BanExpiresAt *time.Time
	UpdatedAt    time.Time
}

// SessionInventoryItem is the honest, per-instance view of one account, merged
// from the in-memory client set, the device store, and persisted status. The
// phone number is already masked.
type SessionInventoryItem struct {
	PhoneMasked  string     `json:"phone_masked"`
	State        string     `json:"state"`  // connected|disconnected|never_paired|logged_out|banned
	Source       string     `json:"source"` // in-memory | store
	Reason       string     `json:"reason,omitempty"`
	LastSeen     time.Time  `json:"last_seen"`
	BanExpiresAt *time.Time `json:"ban_expires_at,omitempty"`
}

// SessionInventory is the /admin/sessions response envelope. Instance identifies
// the reporting node (hostname) since the view is per-instance by design.
type SessionInventory struct {
	Instance string                 `json:"instance"`
	Count    int                    `json:"count"`
	Sessions []SessionInventoryItem `json:"sessions"`
}
