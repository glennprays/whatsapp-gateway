package whatsapp

import "time"

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

// ---- Group & community mutations (Phase E) ----

// GroupTargetRequest is a bare recipient wrapper reused by leave / photo-delete
// (body {chat} or ?chat=). chat wins over the deprecated msisdn alias.
type GroupTargetRequest struct {
	Chat   string `json:"chat"`
	Msisdn string `json:"msisdn,omitempty"` // deprecated alias; chat wins
}

// ParticipantResult is the canonical per-participant outcome shared by every
// batch mutation (create, add/remove/promote/demote, approve/reject). A partial
// failure never fails the whole request — it surfaces here as one entry.
type ParticipantResult struct {
	JID    string             `json:"jid"`
	LID    string             `json:"lid,omitempty"`
	Status string             `json:"status"`         // ok|invited|failed
	Code   int                `json:"code,omitempty"` // whatsmeow participant error code
	Invite *ParticipantInvite `json:"invite,omitempty"`
}

// ParticipantInvite is present when an add was converted to an invite (the
// target's privacy blocks direct add). ExpiresAt marshals to RFC3339.
type ParticipantInvite struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

// CreateGroupRequest creates a group (or community when IsCommunity). An empty
// Participants list makes a group of just the account. Adding participants at
// creation is gated by GROUP_ADD_PARTICIPANTS_ENABLED.
type CreateGroupRequest struct {
	Name                   string   `json:"name"`
	Participants           []string `json:"participants"`
	IsCommunity            bool     `json:"is_community,omitempty"`      // ReqCreateGroup.IsParent
	LinkedParentJID        string   `json:"linked_parent_jid,omitempty"` // create inside a community
	IsAnnounce             bool     `json:"is_announce,omitempty"`
	IsLocked               bool     `json:"is_locked,omitempty"`
	IsJoinApprovalRequired bool     `json:"is_join_approval_required,omitempty"`
}

// CreateGroupResponse echoes the new group's JID, full info, and per-participant
// add results (invited/failed members are reported here, not as an error).
type CreateGroupResponse struct {
	GroupJID  string              `json:"group_jid"`
	GroupInfo *GroupInfoResponse  `json:"group_info"`
	Results   []ParticipantResult `json:"results"`
}

// GroupParticipantsRequest mutates a group's roster. Action is one of
// add|remove|promote|demote; add is gated by GROUP_ADD_PARTICIPANTS_ENABLED.
type GroupParticipantsRequest struct {
	Chat         string   `json:"chat"`
	Msisdn       string   `json:"msisdn,omitempty"`
	Action       string   `json:"action"`
	Participants []string `json:"participants"`
}

// GroupParticipantsResponse carries the per-participant outcome (partial success
// is a 200, never an overall error).
type GroupParticipantsResponse struct {
	GroupJID string              `json:"group_jid"`
	Action   string              `json:"action"`
	Results  []ParticipantResult `json:"results"`
}

// GroupSettingsRequest toggles group-level settings. Pointer fields mean "only
// change what was supplied"; at least one must be set.
type GroupSettingsRequest struct {
	Chat     string `json:"chat"`
	Msisdn   string `json:"msisdn,omitempty"`
	Announce *bool  `json:"announce,omitempty"` // only admins can send
	Locked   *bool  `json:"locked,omitempty"`   // only admins can edit info
}

// GroupSettingsResponse reports which settings were applied.
type GroupSettingsResponse struct {
	GroupJID string   `json:"group_jid"`
	Applied  []string `json:"applied"` // e.g. ["announce","locked"]
}

// SetGroupNameRequest updates a group's name (subject), max 25 chars.
type SetGroupNameRequest struct {
	Chat   string `json:"chat"`
	Msisdn string `json:"msisdn,omitempty"`
	Name   string `json:"name"`
}

// SetGroupTopicRequest updates a group's topic (description). Empty topic clears it.
type SetGroupTopicRequest struct {
	Chat   string `json:"chat"`
	Msisdn string `json:"msisdn,omitempty"`
	Topic  string `json:"topic"`
}

// GroupPhotoResponse reports the new picture ID (on set) or removal.
type GroupPhotoResponse struct {
	PictureID string `json:"picture_id,omitempty"`
	Removed   bool   `json:"removed,omitempty"`
}

// GroupInviteLinkResponse is a group's invite link.
type GroupInviteLinkResponse struct {
	Chat       string `json:"chat"`        // resolved @g.us JID
	InviteLink string `json:"invite_link"` // https://chat.whatsapp.com/<code>
}

// GroupInviteResetRequest targets a group whose invite link should be revoked
// and regenerated.
type GroupInviteResetRequest struct {
	Chat   string `json:"chat"`
	Msisdn string `json:"msisdn,omitempty"`
}

// JoinGroupRequest joins a group by invite link. Code accepts either a full
// chat.whatsapp.com link or a bare code.
type JoinGroupRequest struct {
	Code string `json:"code"`
}

// JoinGroupResponse is the JID whatsmeow returned (the group, or a
// membership-approval request — whatsmeow does not expose which).
type JoinGroupResponse struct {
	GroupJID string `json:"group_jid"`
}

// GroupJoinRequestItem is one pending join request.
type GroupJoinRequestItem struct {
	JID         string `json:"jid"`
	RequestedAt string `json:"requested_at,omitempty"` // RFC3339
}

// GroupJoinRequestsResponse lists a group's pending join requests.
type GroupJoinRequestsResponse struct {
	Chat     string                 `json:"chat"`
	Requests []GroupJoinRequestItem `json:"requests"`
	Count    int                    `json:"count"`
}

// GroupJoinRequestsActionRequest approves or rejects pending join requests.
type GroupJoinRequestsActionRequest struct {
	Chat         string   `json:"chat"`
	Msisdn       string   `json:"msisdn,omitempty"`
	Action       string   `json:"action"` // approve|reject
	Participants []string `json:"participants"`
}

// GroupJoinRequestsActionResponse carries the per-participant outcome (partial
// success is a 200).
type GroupJoinRequestsActionResponse struct {
	Chat    string              `json:"chat"`
	Action  string              `json:"action"`
	Results []ParticipantResult `json:"results"`
}

// CommunityLinkRequest links (or unlinks) a subgroup under a parent community.
type CommunityLinkRequest struct {
	Chat     string `json:"chat"` // parent community @g.us
	Msisdn   string `json:"msisdn,omitempty"`
	ChildJID string `json:"child_jid"` // subgroup @g.us
}
