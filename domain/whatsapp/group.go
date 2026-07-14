package whatsapp

// GroupListItem is one entry in the account's joined-groups list. It is a
// lightweight summary (no participant roster); use group-info for a single
// group's full detail.
type GroupListItem struct {
	JID              string `json:"jid"` // the group's @g.us JID
	Name             string `json:"name,omitempty"`
	Topic            string `json:"topic,omitempty"`
	OwnerJID         string `json:"owner_jid,omitempty"`
	ParticipantCount int    `json:"participant_count"`
	IsAnnounce       bool   `json:"is_announce"`  // only admins can send
	IsLocked         bool   `json:"is_locked"`    // only admins can edit group info
	IsCommunity      bool   `json:"is_community"` // this group is a community parent
}

// GroupListResponse is the account's joined groups. WhatsApp returns the full
// set in one call, so this is not paginated; Count always equals len(Groups).
type GroupListResponse struct {
	Groups []GroupListItem `json:"groups"`
	Count  int             `json:"count"`
}
