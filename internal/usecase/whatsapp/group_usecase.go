package whatsapp_usecase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
)

const (
	maxGroupNameLen  = 25  // server rejects a longer subject with 406
	maxGroupTopicLen = 512 // explicit cap; empty topic clears the description
)

// requireGroupManagement gates the whole group/community mutation surface. The
// route layer already leaves these routes unregistered when the master toggle is
// off (so the surface is a 404, hidden entirely); this is the belt-and-suspenders
// check that also gives the usecase a unit-testable seam.
func (uc *WhatsappMessageUsecase) requireGroupManagement() error {
	if !uc.config.GroupManagementEnabled {
		return errDomain.NewError(errDomain.ErrNotFound, errors.New("group management is disabled"))
	}
	return nil
}

// validParticipantAction reports whether a is a supported roster action.
func validParticipantAction(a string) bool {
	switch a {
	case "add", "remove", "promote", "demote":
		return true
	}
	return false
}

// resolveParticipants normalizes a participant list to canonical user JIDs,
// deduping and enforcing the per-request cap. allowEmpty permits an empty result
// (group creation of just the account); selfGuard rejects the account's own
// number (remove/promote/demote must go through LeaveGroup). A group JID in the
// list is a 400 — participants are always users.
func (uc *WhatsappMessageUsecase) resolveParticipants(list []string, ownPhone string, allowEmpty, selfGuard bool) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(list))
	for _, p := range list {
		jid, err := resolveChat(p, "")
		if err != nil {
			return nil, err
		}
		if strings.HasSuffix(jid, "@"+groupServerSuffix) {
			return nil, errDomain.NewError(errDomain.ErrBadRequest,
				fmt.Errorf("participant must be a user, not a group: %q", p))
		}
		if selfGuard && strings.SplitN(jid, "@", 2)[0] == ownPhone {
			return nil, errDomain.NewError(errDomain.ErrBadRequest,
				errors.New("use POST /group/leave to remove yourself"))
		}
		if !seen[jid] {
			seen[jid] = true
			out = append(out, jid)
		}
	}
	if len(out) == 0 && !allowEmpty {
		return nil, errDomain.NewError(errDomain.ErrBadRequest, errors.New("participants is required"))
	}
	if uc.config.GroupMaxParticipantsPerRequest > 0 && len(out) > uc.config.GroupMaxParticipantsPerRequest {
		return nil, errDomain.NewError(errDomain.ErrBadRequest,
			fmt.Errorf("too many participants (max %d)", uc.config.GroupMaxParticipantsPerRequest))
	}
	return out, nil
}

// CreateGroup creates a group (or community). Adding participants at creation is
// gated by GROUP_ADD_PARTICIPANTS_ENABLED; per-participant add outcomes ride back
// in the response (never an overall error).
func (uc *WhatsappMessageUsecase) CreateGroup(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.CreateGroupRequest,
) (*waDomain.CreateGroupResponse, error) {
	if err := uc.requireGroupManagement(); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errDomain.NewError(errDomain.ErrBadRequest, errors.New("name is required"))
	}
	if len(name) > maxGroupNameLen {
		return nil, errDomain.NewError(errDomain.ErrBadRequest,
			fmt.Errorf("name exceeds maximum length of %d characters", maxGroupNameLen))
	}
	participants, err := uc.resolveParticipants(req.Participants, phoneNumber, true, false)
	if err != nil {
		return nil, err
	}
	if len(participants) > 0 && !uc.config.GroupAddParticipantsEnabled {
		return nil, errDomain.NewError(errDomain.ErrForbidden, errors.New("adding participants is disabled"))
	}
	var linkedParent string
	if strings.TrimSpace(req.LinkedParentJID) != "" {
		linkedParent, err = resolveGroupJID(req.LinkedParentJID, "")
		if err != nil {
			return nil, err
		}
	}
	if err := uc.spendActionBudget(ctx, phoneNumber); err != nil {
		return nil, err
	}
	return uc.whatsappManager.CreateGroup(ctx, traceID, phoneNumber, name, participants,
		req.IsCommunity, linkedParent, req.IsAnnounce, req.IsLocked, req.IsJoinApprovalRequired)
}

// LeaveGroup removes the account from a group. Allowed for non-admins.
func (uc *WhatsappMessageUsecase) LeaveGroup(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.GroupTargetRequest,
) error {
	if err := uc.requireGroupManagement(); err != nil {
		return err
	}
	target, err := resolveGroupJID(req.Chat, req.Msisdn)
	if err != nil {
		return err
	}
	if err := uc.spendActionBudget(ctx, phoneNumber); err != nil {
		return err
	}
	return uc.whatsappManager.LeaveGroup(ctx, traceID, phoneNumber, target)
}

// UpdateGroupParticipants mutates a group's roster. action=add is gated by
// GROUP_ADD_PARTICIPANTS_ENABLED; remove/promote/demote self-guard the caller's
// own number. Per-participant failures are 200 partial success, never an error.
func (uc *WhatsappMessageUsecase) UpdateGroupParticipants(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.GroupParticipantsRequest,
) (*waDomain.GroupParticipantsResponse, error) {
	if err := uc.requireGroupManagement(); err != nil {
		return nil, err
	}
	target, err := resolveGroupJID(req.Chat, req.Msisdn)
	if err != nil {
		return nil, err
	}
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if !validParticipantAction(action) {
		return nil, errDomain.NewError(errDomain.ErrBadRequest,
			errors.New("action must be one of add, remove, promote, demote"))
	}
	participants, err := uc.resolveParticipants(req.Participants, phoneNumber, false, action != "add")
	if err != nil {
		return nil, err
	}
	if action == "add" && !uc.config.GroupAddParticipantsEnabled {
		return nil, errDomain.NewError(errDomain.ErrForbidden, errors.New("adding participants is disabled"))
	}
	if err := uc.spendActionBudget(ctx, phoneNumber); err != nil {
		return nil, err
	}
	results, err := uc.whatsappManager.UpdateGroupParticipants(ctx, traceID, phoneNumber, target, action, participants)
	if err != nil {
		return nil, err
	}
	return &waDomain.GroupParticipantsResponse{GroupJID: target, Action: action, Results: results}, nil
}

// SetGroupSettings toggles announce/locked. At least one flag must be supplied.
// Flags apply sequentially; the first server error is returned (a partial apply
// is possible and documented as best-effort).
func (uc *WhatsappMessageUsecase) SetGroupSettings(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.GroupSettingsRequest,
) (*waDomain.GroupSettingsResponse, error) {
	if err := uc.requireGroupManagement(); err != nil {
		return nil, err
	}
	target, err := resolveGroupJID(req.Chat, req.Msisdn)
	if err != nil {
		return nil, err
	}
	if req.Announce == nil && req.Locked == nil {
		return nil, errDomain.NewError(errDomain.ErrBadRequest,
			errors.New("no settings supplied (announce and/or locked)"))
	}
	if err := uc.spendActionBudget(ctx, phoneNumber); err != nil {
		return nil, err
	}
	applied := make([]string, 0, 2)
	if req.Announce != nil {
		if err := uc.whatsappManager.SetGroupAnnounce(ctx, traceID, phoneNumber, target, *req.Announce); err != nil {
			return nil, err
		}
		applied = append(applied, "announce")
	}
	if req.Locked != nil {
		if err := uc.whatsappManager.SetGroupLocked(ctx, traceID, phoneNumber, target, *req.Locked); err != nil {
			return nil, err
		}
		applied = append(applied, "locked")
	}
	return &waDomain.GroupSettingsResponse{GroupJID: target, Applied: applied}, nil
}

// SetGroupName updates the group name (subject); max 25 chars, non-empty.
func (uc *WhatsappMessageUsecase) SetGroupName(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.SetGroupNameRequest,
) error {
	if err := uc.requireGroupManagement(); err != nil {
		return err
	}
	target, err := resolveGroupJID(req.Chat, req.Msisdn)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return errDomain.NewError(errDomain.ErrBadRequest, errors.New("name is required"))
	}
	if len(name) > maxGroupNameLen {
		return errDomain.NewError(errDomain.ErrBadRequest,
			fmt.Errorf("name exceeds maximum length of %d characters", maxGroupNameLen))
	}
	if err := uc.spendActionBudget(ctx, phoneNumber); err != nil {
		return err
	}
	return uc.whatsappManager.SetGroupName(ctx, traceID, phoneNumber, target, name)
}

// SetGroupTopic updates the group topic (description); max 512 chars. An empty
// topic is valid and clears the description.
func (uc *WhatsappMessageUsecase) SetGroupTopic(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.SetGroupTopicRequest,
) error {
	if err := uc.requireGroupManagement(); err != nil {
		return err
	}
	target, err := resolveGroupJID(req.Chat, req.Msisdn)
	if err != nil {
		return err
	}
	if len(req.Topic) > maxGroupTopicLen {
		return errDomain.NewError(errDomain.ErrBadRequest,
			fmt.Errorf("topic exceeds maximum length of %d characters", maxGroupTopicLen))
	}
	if err := uc.spendActionBudget(ctx, phoneNumber); err != nil {
		return err
	}
	return uc.whatsappManager.SetGroupTopic(ctx, traceID, phoneNumber, target, req.Topic)
}

// SetGroupPhoto sets the group picture from an uploaded JPEG. The file is size-
// capped and sniffed for image/jpeg before hitting the server.
func (uc *WhatsappMessageUsecase) SetGroupPhoto(
	ctx context.Context,
	traceID, phoneNumber, chat, msisdn string,
	file *multipart.FileHeader,
) (*waDomain.GroupPhotoResponse, error) {
	if err := uc.requireGroupManagement(); err != nil {
		return nil, err
	}
	target, err := resolveGroupJID(chat, msisdn)
	if err != nil {
		return nil, err
	}
	if file == nil {
		return nil, errDomain.NewError(errDomain.ErrBadRequest, errors.New("photo is required"))
	}
	if err := uc.validateMediaSize(file.Size); err != nil {
		return nil, err
	}
	f, err := file.Open()
	if err != nil {
		return nil, errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("failed to read photo: %w", err))
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, errDomain.NewError(errDomain.ErrInternalFailure, fmt.Errorf("failed to read photo: %w", err))
	}
	if http.DetectContentType(data) != "image/jpeg" {
		return nil, errDomain.NewError(errDomain.ErrBadRequest, errors.New("group photo must be a JPEG"))
	}
	if err := uc.spendActionBudget(ctx, phoneNumber); err != nil {
		return nil, err
	}
	pictureID, err := uc.whatsappManager.SetGroupPhoto(ctx, traceID, phoneNumber, target, data)
	if err != nil {
		return nil, err
	}
	return &waDomain.GroupPhotoResponse{PictureID: pictureID}, nil
}

// RemoveGroupPhoto clears the group picture.
func (uc *WhatsappMessageUsecase) RemoveGroupPhoto(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.GroupTargetRequest,
) (*waDomain.GroupPhotoResponse, error) {
	if err := uc.requireGroupManagement(); err != nil {
		return nil, err
	}
	target, err := resolveGroupJID(req.Chat, req.Msisdn)
	if err != nil {
		return nil, err
	}
	if err := uc.spendActionBudget(ctx, phoneNumber); err != nil {
		return nil, err
	}
	if _, err := uc.whatsappManager.SetGroupPhoto(ctx, traceID, phoneNumber, target, nil); err != nil {
		return nil, err
	}
	return &waDomain.GroupPhotoResponse{Removed: true}, nil
}

// GetGroupInviteLink returns a group's invite link. Not cached — the link can be
// reset out-of-band, so a stale cache would hand out a dead link.
func (uc *WhatsappMessageUsecase) GetGroupInviteLink(
	ctx context.Context,
	traceID, phoneNumber, chat, msisdn string,
) (*waDomain.GroupInviteLinkResponse, error) {
	if err := uc.requireGroupManagement(); err != nil {
		return nil, err
	}
	target, err := resolveGroupJID(chat, msisdn)
	if err != nil {
		return nil, err
	}
	if err := uc.spendActionBudget(ctx, phoneNumber); err != nil {
		return nil, err
	}
	link, err := uc.whatsappManager.GetGroupInviteLink(ctx, traceID, phoneNumber, target, false)
	if err != nil {
		return nil, err
	}
	return &waDomain.GroupInviteLinkResponse{Chat: target, InviteLink: link}, nil
}

// ResetGroupInviteLink revokes the current invite link and returns a fresh one.
func (uc *WhatsappMessageUsecase) ResetGroupInviteLink(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.GroupInviteResetRequest,
) (*waDomain.GroupInviteLinkResponse, error) {
	if err := uc.requireGroupManagement(); err != nil {
		return nil, err
	}
	target, err := resolveGroupJID(req.Chat, req.Msisdn)
	if err != nil {
		return nil, err
	}
	if err := uc.spendActionBudget(ctx, phoneNumber); err != nil {
		return nil, err
	}
	link, err := uc.whatsappManager.GetGroupInviteLink(ctx, traceID, phoneNumber, target, true)
	if err != nil {
		return nil, err
	}
	return &waDomain.GroupInviteLinkResponse{Chat: target, InviteLink: link}, nil
}

// GetGroupInviteInfo previews a group from an invite code without joining. Served
// through the shared read cache + budget (keyed by code).
func (uc *WhatsappMessageUsecase) GetGroupInviteInfo(
	ctx context.Context,
	traceID, phoneNumber, code string,
) (*waDomain.GroupInfoResponse, error) {
	if err := uc.requireGroupManagement(); err != nil {
		return nil, err
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, errDomain.NewError(errDomain.ErrBadRequest, errors.New("code is required"))
	}
	return queryWithBudget(uc, ctx, phoneNumber, "inviteinfo:"+phoneNumber+":"+code,
		func() (*waDomain.GroupInfoResponse, error) {
			return uc.whatsappManager.GetGroupInfoFromLink(ctx, traceID, phoneNumber, code)
		})
}

// JoinGroup joins a group by invite link. This is the mass-join vector, gated by
// GROUP_JOIN_VIA_LINK_ENABLED (403 when off) on top of the master toggle.
func (uc *WhatsappMessageUsecase) JoinGroup(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.JoinGroupRequest,
) (*waDomain.JoinGroupResponse, error) {
	if err := uc.requireGroupManagement(); err != nil {
		return nil, err
	}
	if !uc.config.GroupJoinViaLinkEnabled {
		return nil, errDomain.NewError(errDomain.ErrForbidden, errors.New("joining via link is disabled"))
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		return nil, errDomain.NewError(errDomain.ErrBadRequest, errors.New("code is required"))
	}
	if err := uc.spendActionBudget(ctx, phoneNumber); err != nil {
		return nil, err
	}
	jid, err := uc.whatsappManager.JoinGroupWithLink(ctx, traceID, phoneNumber, code)
	if err != nil {
		return nil, err
	}
	return &waDomain.JoinGroupResponse{GroupJID: jid}, nil
}

// ListJoinRequests lists a group's pending join requests. Served through the
// shared read cache + budget (keyed by group JID). Requires admin.
func (uc *WhatsappMessageUsecase) ListJoinRequests(
	ctx context.Context,
	traceID, phoneNumber, chat, msisdn string,
) (*waDomain.GroupJoinRequestsResponse, error) {
	if err := uc.requireGroupManagement(); err != nil {
		return nil, err
	}
	target, err := resolveGroupJID(chat, msisdn)
	if err != nil {
		return nil, err
	}
	return queryWithBudget(uc, ctx, phoneNumber, "joinreqs:"+phoneNumber+":"+target,
		func() (*waDomain.GroupJoinRequestsResponse, error) {
			items, err := uc.whatsappManager.GetGroupRequestParticipants(ctx, traceID, phoneNumber, target)
			if err != nil {
				return nil, err
			}
			return &waDomain.GroupJoinRequestsResponse{Chat: target, Requests: items, Count: len(items)}, nil
		})
}

// UpdateJoinRequests approves or rejects pending join requests. Per-participant
// failures are 200 partial success, never an overall error.
func (uc *WhatsappMessageUsecase) UpdateJoinRequests(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.GroupJoinRequestsActionRequest,
) (*waDomain.GroupJoinRequestsActionResponse, error) {
	if err := uc.requireGroupManagement(); err != nil {
		return nil, err
	}
	target, err := resolveGroupJID(req.Chat, req.Msisdn)
	if err != nil {
		return nil, err
	}
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action != "approve" && action != "reject" {
		return nil, errDomain.NewError(errDomain.ErrBadRequest, errors.New("action must be approve or reject"))
	}
	participants, err := uc.resolveParticipants(req.Participants, phoneNumber, false, false)
	if err != nil {
		return nil, err
	}
	if err := uc.spendActionBudget(ctx, phoneNumber); err != nil {
		return nil, err
	}
	results, err := uc.whatsappManager.UpdateGroupRequestParticipants(ctx, traceID, phoneNumber, target, participants, action == "approve")
	if err != nil {
		return nil, err
	}
	return &waDomain.GroupJoinRequestsActionResponse{Chat: target, Action: action, Results: results}, nil
}

// LinkSubGroup links a subgroup under a parent community. Both parent and child
// must be explicit @g.us JIDs.
func (uc *WhatsappMessageUsecase) LinkSubGroup(
	ctx context.Context,
	traceID, phoneNumber string,
	req waDomain.CommunityLinkRequest,
) error {
	if err := uc.requireGroupManagement(); err != nil {
		return err
	}
	parent, err := resolveGroupJID(req.Chat, req.Msisdn)
	if err != nil {
		return err
	}
	child, err := resolveGroupJID(req.ChildJID, "")
	if err != nil {
		return err
	}
	if err := uc.spendActionBudget(ctx, phoneNumber); err != nil {
		return err
	}
	return uc.whatsappManager.LinkSubGroup(ctx, traceID, phoneNumber, parent, child)
}

// UnlinkSubGroup removes a subgroup from a parent community. Both parent (chat)
// and child must be explicit @g.us JIDs.
func (uc *WhatsappMessageUsecase) UnlinkSubGroup(
	ctx context.Context,
	traceID, phoneNumber, chat, msisdn, child string,
) error {
	if err := uc.requireGroupManagement(); err != nil {
		return err
	}
	parent, err := resolveGroupJID(chat, msisdn)
	if err != nil {
		return err
	}
	childJID, err := resolveGroupJID(child, "")
	if err != nil {
		return err
	}
	if err := uc.spendActionBudget(ctx, phoneNumber); err != nil {
		return err
	}
	return uc.whatsappManager.UnlinkSubGroup(ctx, traceID, phoneNumber, parent, childJID)
}
