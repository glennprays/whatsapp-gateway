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

// GroupParticipantItem is one member in a group's roster.
type GroupParticipantItem struct {
	JID          string `json:"jid"`
	PhoneNumber  string `json:"phone_number,omitempty"`
	LID          string `json:"lid,omitempty"`
	IsAdmin      bool   `json:"is_admin"`
	IsSuperAdmin bool   `json:"is_super_admin"`
}

// GroupInfoResponse is the full detail of a single group, including its member
// roster. Requires the account to be a participant (403 otherwise).
type GroupInfoResponse struct {
	JID              string                 `json:"jid"`
	Name             string                 `json:"name,omitempty"`
	Topic            string                 `json:"topic,omitempty"`
	OwnerJID         string                 `json:"owner_jid,omitempty"`
	ParticipantCount int                    `json:"participant_count"`
	IsAnnounce       bool                   `json:"is_announce"`
	IsLocked         bool                   `json:"is_locked"`
	IsCommunity      bool                   `json:"is_community"`
	IsEphemeral      bool                   `json:"is_ephemeral"`
	Participants     []GroupParticipantItem `json:"participants"`
}

// SubGroupItem is one linked group under a community (whatsmeow
// types.GroupLinkTarget). The default sub-group is the community's
// announcement group.
type SubGroupItem struct {
	JID               string `json:"jid"` // the sub-group's @g.us JID
	Name              string `json:"name,omitempty"`
	IsDefaultSubGroup bool   `json:"is_default_sub_group"`
}

// SubGroupListResponse is a community's linked sub-groups. Count always equals
// len(SubGroups).
type SubGroupListResponse struct {
	SubGroups []SubGroupItem `json:"sub_groups"`
	Count     int            `json:"count"`
}

// CommunityParticipantItem is one member across the community's linked groups.
// whatsmeow returns only the primary JID (@lid or @s.whatsapp.net); no PN/LID
// split.
type CommunityParticipantItem struct {
	JID string `json:"jid"`
}

// CommunityParticipantsResponse is every participant across the community's
// linked groups. Count always equals len(Participants).
type CommunityParticipantsResponse struct {
	Participants []CommunityParticipantItem `json:"participants"`
	Count        int                        `json:"count"`
}
