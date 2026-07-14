package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	customLog "github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/constant"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/binary"
	waE2E "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

type (
	client struct {
		container  *sqlstore.Container
		cfg        *config.Config
		repository WhatsAppRepository
		logger     *customLog.Logger
	}
	Client interface {
		InitClient(traceID string, phoneNumber string, device *store.Device, eventHandler func(string, any))
		Reconnect(traceID string, phoneNumber string) error
		LoginQRCode(ctx context.Context, traceID string, phoneNumber string) (string, int, error)
		LoginPairCode(ctx context.Context, traceID string, phoneNumber string) (string, int, error)
		LoginStatus(traceID string, phoneNumber string) (bool, error)
		Logout(ctx context.Context, traceID string, phoneNumber string) error
		GetWebhookURL(ctx context.Context, traceID string, phoneNumber string) (*string, error)
		CheckNumber(ctx context.Context, traceID string, phoneNumber string, msisdn string) (waDomain.ContactCheckResponse, error)
		ListContacts(ctx context.Context, traceID string, phoneNumber string) ([]waDomain.ContactListItem, error)
		ListGroups(ctx context.Context, traceID string, phoneNumber string) ([]waDomain.GroupListItem, error)
		GetGroupInfo(ctx context.Context, traceID string, phoneNumber string, groupJID string) (*waDomain.GroupInfoResponse, error)
		ListSubGroups(ctx context.Context, traceID string, phoneNumber string, communityJID string) ([]waDomain.SubGroupItem, error)
		ListCommunityParticipants(ctx context.Context, traceID string, phoneNumber string, communityJID string) ([]waDomain.CommunityParticipantItem, error)
		GetContactInfo(ctx context.Context, traceID string, phoneNumber string, userJID string) (*waDomain.ContactInfoResponse, error)
		GetAvatar(ctx context.Context, traceID string, phoneNumber string, targetJID string, preview bool, existingID string) (*waDomain.AvatarResponse, error)
		MarkRead(ctx context.Context, traceID string, phoneNumber string, chat string, sender string, messageIDs []string) error
		SendChatPresence(ctx context.Context, traceID string, phoneNumber string, chat string, state string, media string) error
		SetWebhookURL(ctx context.Context, traceID string, phoneNumber string, webhook *waDomain.Webhook) error
		DeleteWebhookURL(ctx context.Context, traceID string, phoneNumber string) error
		SendTextMessage(ctx context.Context, traceID string, phoneNumber string, to string, message string, msgCtx *waDomain.MessageContext) (string, error)
		SendImageMessage(ctx context.Context, traceID string, phoneNumber string, to string, imageBytes []byte, mimeType string, caption string, isViewOnce bool, msgCtx *waDomain.MessageContext) (string, error)
		SendAudioMessage(ctx context.Context, traceID string, phoneNumber string, to string, audioBytes []byte, mimeType string, isPTT bool, isViewOnce bool, msgCtx *waDomain.MessageContext) (string, error)
		SendVideoMessage(ctx context.Context, traceID string, phoneNumber string, to string, videoBytes []byte, mimeType string, caption string, isGif bool, isViewOnce bool, msgCtx *waDomain.MessageContext) (string, error)
		SendDocumentMessage(ctx context.Context, traceID string, phoneNumber string, to string, docBytes []byte, mimeType string, fileName string, caption string, msgCtx *waDomain.MessageContext) (string, error)
		SendLocationMessage(ctx context.Context, traceID string, phoneNumber string, to string, latitude float64, longitude float64, name string, address string, msgCtx *waDomain.MessageContext) (string, error)
		SendPollMessage(ctx context.Context, traceID string, phoneNumber string, to string, question string, options []string, selectableCount int, msgCtx *waDomain.MessageContext) (string, error)
		SendStickerMessage(ctx context.Context, traceID string, phoneNumber string, to string, stickerBytes []byte, mimeType string, msgCtx *waDomain.MessageContext) (string, error)
		ReactToMessage(ctx context.Context, traceID string, phoneNumber string, chatJID string, senderJID string, messageID string, emoji string) error
		DeleteMessage(ctx context.Context, traceID string, phoneNumber string, chatJID string, messageID string) error
		EditMessage(ctx context.Context, traceID string, phoneNumber string, chatJID string, messageID string, newText string) error
		// Group & community mutations (Phase E).
		CreateGroup(ctx context.Context, traceID string, phoneNumber string, name string, participantJIDs []string, isCommunity bool, linkedParentJID string, isAnnounce bool, isLocked bool, isJoinApproval bool) (*waDomain.CreateGroupResponse, error)
		LeaveGroup(ctx context.Context, traceID string, phoneNumber string, groupJID string) error
		UpdateGroupParticipants(ctx context.Context, traceID string, phoneNumber string, groupJID string, action string, participantJIDs []string) ([]waDomain.ParticipantResult, error)
		SetGroupAnnounce(ctx context.Context, traceID string, phoneNumber string, groupJID string, announce bool) error
		SetGroupLocked(ctx context.Context, traceID string, phoneNumber string, groupJID string, locked bool) error
		SetGroupName(ctx context.Context, traceID string, phoneNumber string, groupJID string, name string) error
		SetGroupTopic(ctx context.Context, traceID string, phoneNumber string, groupJID string, topic string) error
		SetGroupPhoto(ctx context.Context, traceID string, phoneNumber string, groupJID string, photo []byte) (string, error)
		GetGroupInviteLink(ctx context.Context, traceID string, phoneNumber string, groupJID string, reset bool) (string, error)
		JoinGroupWithLink(ctx context.Context, traceID string, phoneNumber string, code string) (string, error)
		GetGroupInfoFromLink(ctx context.Context, traceID string, phoneNumber string, code string) (*waDomain.GroupInfoResponse, error)
		GetGroupRequestParticipants(ctx context.Context, traceID string, phoneNumber string, groupJID string) ([]waDomain.GroupJoinRequestItem, error)
		UpdateGroupRequestParticipants(ctx context.Context, traceID string, phoneNumber string, groupJID string, participantJIDs []string, approve bool) ([]waDomain.ParticipantResult, error)
		LinkSubGroup(ctx context.Context, traceID string, phoneNumber string, parentJID string, childJID string) error
		UnlinkSubGroup(ctx context.Context, traceID string, phoneNumber string, parentJID string, childJID string) error
		SessionInventory(ctx context.Context) ([]waDomain.SessionInventoryItem, error)
		GetOneSession(ctx context.Context, phone string) (*waDomain.SessionInventoryItem, error)
	}
)

func NewClient(container *sqlstore.Container, cfg *config.Config, repo WhatsAppRepository, logger *customLog.Logger) Client {
	return &client{
		container:  container,
		cfg:        cfg,
		repository: repo,
		logger:     logger,
	}
}

func (c *client) InitClient(traceID string, phoneNumber string, device *store.Device, eventHandler func(string, any)) {
	binary.IndentXML = true
	if clients.Get(phoneNumber) == nil {
		if device == nil {
			// Prefer restoring an existing paired device from the store so a
			// session that was evicted from the in-memory map (but still
			// persisted) reconnects instead of being orphaned and forced to
			// re-pair. Fall back to a fresh device only when no row exists
			// (a never-linked phone).
			if restored := c.findDeviceByPhone(traceID, phoneNumber); restored != nil {
				device = restored
			} else {
				c.logger.Info(traceID, fmt.Sprintf("Creating new device for Phone Number: %s", MaskedPhoneNumber(phoneNumber)), nil)
				device = c.container.NewDevice()
			}
		}
		store.DeviceProps.Os = proto.String(c.cfg.WhatsappDeviceLabel)
		store.DeviceProps.RequireFullSync = proto.Bool(false)

		cli := whatsmeow.NewClient(device, waLog.Stdout("Client-login", c.cfg.WhatsmeowLogLevel, true))
		cli.AddEventHandler(func(evt any) {
			eventHandler(phoneNumber, evt)
		})
		cli.EnableAutoReconnect = true
		cli.AutoTrustIdentity = true

		clients.Set(phoneNumber, cli)
	}
}

// findDeviceByPhone returns the persisted device whose JID user matches the
// given phone number, or nil if none exists in the store. Used to restore a
// paired session on demand when its in-memory client is missing.
func (c *client) findDeviceByPhone(traceID string, phoneNumber string) *store.Device {
	devices, err := c.container.GetAllDevices(context.Background())
	if err != nil {
		c.logger.Error(traceID, "Failed to list devices while restoring client",
			nil,
			customLog.String("phone_number", MaskedPhoneNumber(phoneNumber)),
			customLog.Error(err),
		)
		return nil
	}
	for _, device := range devices {
		if device.ID == nil {
			continue
		}
		if WhatsappDecomposeJID(device.ID.User) == phoneNumber {
			c.logger.Info(traceID, "Restoring existing device from store for "+MaskedPhoneNumber(phoneNumber), nil)
			return device
		}
	}
	return nil
}

func (c *client) Reconnect(traceID string, phoneNumber string) error {
	cli := clients.Get(phoneNumber)
	if cli == nil {
		return errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}
	cli.Disconnect()
	err := cli.Connect()
	if err != nil {
		return c.mapWhatsmeowErr(traceID, phoneNumber, err)
	}
	return nil
}

// mapWhatsmeowErr translates a raw whatsmeow error into the gateway's
// domain error space. The key case is store.ErrDeviceDeleted ("invalid
// use of deleted device"): once WhatsApp has marked the device deleted,
// the in-memory client pointer is permanently dead, so we evict it from
// the Clients map so subsequent calls return ErrClientNotFound (404)
// rather than looping forever on the same deleted state.
func (c *client) mapWhatsmeowErr(traceID string, phoneNumber string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, store.ErrDeviceDeleted) {
		c.logger.Warn(traceID, "WhatsApp session was deleted by server; re-pair required",
			nil,
			customLog.String("phone_number", MaskedPhoneNumber(phoneNumber)),
			customLog.Error(err),
		)
		clients.Delete(phoneNumber)
		return errDomain.NewError(errDomain.ErrConflict, errors.New(constant.ErrClientSessionDeleted))
	}
	// Read-surface sentinels (matched via errors.Is, never substring — ADR 0002).
	// "Absent" resources map to 404; "not allowed to see it" maps to 403.
	if errors.Is(err, whatsmeow.ErrGroupNotFound) || errors.Is(err, whatsmeow.ErrProfilePictureNotSet) {
		return errDomain.NewError(errDomain.ErrNotFound, err)
	}
	if errors.Is(err, whatsmeow.ErrNotInGroup) || errors.Is(err, whatsmeow.ErrProfilePictureUnauthorized) {
		return errDomain.NewError(errDomain.ErrForbidden, err)
	}
	// Group-mutation named sentinels (each wraps an IQError; matched first so the
	// specific human message wins — status is identical to the raw-code fallback).
	if errors.Is(err, whatsmeow.ErrInvalidImageFormat) {
		return errDomain.NewError(errDomain.ErrBadRequest, errors.New("group photo must be a JPEG"))
	}
	if errors.Is(err, whatsmeow.ErrInviteLinkRevoked) {
		return errDomain.NewError(errDomain.ErrGone, err)
	}
	if errors.Is(err, whatsmeow.ErrInviteLinkInvalid) {
		return errDomain.NewError(errDomain.ErrBadRequest, err)
	}
	if errors.Is(err, whatsmeow.ErrGroupInviteLinkUnauthorized) {
		return errDomain.NewError(errDomain.ErrForbidden, err)
	}
	// Raw *whatsmeow.IQError fallback: the mutation calls (CreateGroup,
	// UpdateGroupParticipants, Set*, Link/UnlinkGroup) and reads (GetSubGroups,
	// GetLinkedGroupsParticipants) return a bare IQError instead of wrapping it in
	// ErrGroupNotFound. Map by the server's IQ code. Runs AFTER the sentinel
	// checks so a wrapped ErrGroupNotFound still wins.
	if mapped, ok := mapIQError(err); ok {
		return mapped
	}
	// Recipient problems (e.g. "can't send message to unknown server") are
	// caller input errors, not server faults — surface as 400, not 500.
	if isRecipientError(err) {
		return errDomain.NewError(errDomain.ErrBadRequest, err)
	}
	return errDomain.NewError(errDomain.ErrInternalFailure, err)
}

// mapIQError maps a raw *whatsmeow.IQError to a domain error by its server IQ
// code, returning ok=false when err is not an IQError so callers can fall
// through to other checks. An authenticated caller lacking a group role (401/403)
// is a 403 (never a 401). 410 (gone) → ErrGone; 409/423 (conflict/locked) →
// ErrConflict; 419/429 (resource/rate limit) → ErrTooManyRequests.
func mapIQError(err error) (error, bool) {
	var iq *whatsmeow.IQError
	if !errors.As(err, &iq) {
		return nil, false
	}
	switch iq.Code {
	case 400, 405, 406:
		return errDomain.NewError(errDomain.ErrBadRequest, err), true
	case 401, 403:
		return errDomain.NewError(errDomain.ErrForbidden, err), true
	case 404:
		return errDomain.NewError(errDomain.ErrNotFound, err), true
	case 409, 423:
		return errDomain.NewError(errDomain.ErrConflict, err), true
	case 410:
		return errDomain.NewError(errDomain.ErrGone, err), true
	case 419, 429:
		return errDomain.NewError(errDomain.ErrTooManyRequests, err), true
	default:
		return errDomain.NewError(errDomain.ErrInternalFailure, err), true
	}
}

// isRecipientError reports whether a whatsmeow send error is caused by a bad
// recipient/JID rather than a server-side fault.
func isRecipientError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unknown server") ||
		strings.Contains(msg, "invalid jid") ||
		strings.Contains(msg, "recipient")
}

func (c *client) LoginQRCode(ctx context.Context, traceID string, phoneNumber string) (string, int, error) {
	cli := clients.Get(phoneNumber)
	if cli != nil {
		cli.Disconnect()
		if cli.Store.ID == nil {
			qrChanGenerate, _ := cli.GetQRChannel(ctx)
			err := cli.Connect()
			if err != nil {
				return "", 0, c.mapWhatsmeowErr(traceID, phoneNumber, err)
			}

			qrImage, qrTimeout, err := WhatsappGenerateQRCode(ctx, qrChanGenerate)
			if err != nil {
				return "", 0, err
			}
			return fmt.Sprintf(`data:image/png;base64,%s`, qrImage), qrTimeout, nil
		}

		err := c.Reconnect(traceID, phoneNumber)
		if err != nil {
			return "", 0, err
		}

		return "", 0, errDomain.NewError(errDomain.ErrConflict, errors.New(constant.ErrClientAlreadyLoggedIn))
	}

	return "", 0, errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
}

func (c *client) LoginPairCode(ctx context.Context, traceID string, phoneNumber string) (string, int, error) {
	cli := clients.Get(phoneNumber)
	if cli == nil {
		return "", 0, errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}
	cli.Disconnect()
	if cli.Store.ID == nil {
		err := cli.Connect()
		if err != nil {
			return "", 0, c.mapWhatsmeowErr(traceID, phoneNumber, err)
		}

		pairCode, err := cli.PairPhone(ctx, phoneNumber, true, whatsmeow.PairClientChrome, fmt.Sprintf("Chrome (%s)", WhatsAppGetUserOS()))
		if err != nil {
			return "", 0, err
		}
		return pairCode, 160, nil
	}

	err := c.Reconnect(traceID, phoneNumber)
	if err != nil {
		return "", 0, err
	}

	return "", 0, errDomain.NewError(errDomain.ErrConflict, errors.New(constant.ErrClientAlreadyLoggedIn))
}

func (c *client) LoginStatus(traceID string, phoneNumber string) (bool, error) {
	cli := clients.Get(phoneNumber)
	if cli != nil {
		return cli.IsLoggedIn(), nil
	}
	return false, errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
}

func (c *client) Logout(ctx context.Context, traceID string, phoneNumber string) error {
	cli := clients.Get(phoneNumber)
	if cli != nil {
		err := cli.Logout(ctx)
		if err != nil {
			masked := MaskedPhoneNumber(phoneNumber)
			c.logger.Error(traceID, "Failed to logout client, forcing local cleanup",
				nil,
				customLog.String("phone_number", masked),
				customLog.Error(err),
			)
			cli.Disconnect()
			if delErr := cli.Store.Delete(ctx); delErr != nil {
				c.logger.Error(traceID, "Failed to delete client store",
					nil,
					customLog.String("phone_number", masked),
					customLog.Error(delErr),
				)
			}
			clients.Delete(phoneNumber)
			return errDomain.NewError(errDomain.ErrConflict, errors.New(constant.ErrClientSessionDeleted))
		}
		return nil
	}
	return errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
}

func (c *client) CheckNumber(ctx context.Context, traceID string, phoneNumber string, msisdn string) (waDomain.ContactCheckResponse, error) {
	cli := clients.Get(phoneNumber)
	if cli == nil {
		return waDomain.ContactCheckResponse{}, errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}
	if !cli.IsLoggedIn() {
		return waDomain.ContactCheckResponse{}, errDomain.NewError(errDomain.ErrUnauthorized, errors.New("client not logged in"))
	}

	resp, err := cli.IsOnWhatsApp(ctx, []string{msisdn})
	if err != nil {
		if errors.Is(err, store.ErrDeviceDeleted) {
			return waDomain.ContactCheckResponse{}, c.mapWhatsmeowErr(traceID, phoneNumber, err)
		}
		return waDomain.ContactCheckResponse{}, c.mapWhatsmeowErr(traceID, phoneNumber, fmt.Errorf("failed to check number: %w", err))
	}
	if len(resp) == 0 {
		return waDomain.ContactCheckResponse{}, errDomain.NewError(errDomain.ErrInternalFailure, errors.New("empty IsOnWhatsApp response"))
	}

	out := waDomain.ContactCheckResponse{
		Query:        resp[0].Query,
		JID:          resp[0].JID.String(),
		IsOnWhatsApp: resp[0].IsIn,
	}
	if resp[0].VerifiedName != nil && resp[0].VerifiedName.Details != nil {
		name := resp[0].VerifiedName.Details.GetVerifiedName()
		if name != "" {
			out.VerifiedName = &name
		}
	}
	return out, nil
}

// ListContacts returns the account's locally-synced contacts from the whatsmeow
// store. It performs no network call; the store reflects synced state, so an
// empty or partial map is returned as-is (the usecase adds metadata). Pagination
// happens in the usecase layer.
func (c *client) ListContacts(ctx context.Context, traceID string, phoneNumber string) ([]waDomain.ContactListItem, error) {
	cli := clients.Get(phoneNumber)
	if cli == nil {
		return nil, errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}
	if !cli.IsLoggedIn() {
		return nil, errDomain.NewError(errDomain.ErrUnauthorized, errors.New("client not logged in"))
	}
	if cli.Store == nil || cli.Store.Contacts == nil {
		return nil, errDomain.NewError(errDomain.ErrConflict, errors.New("session store not ready"))
	}

	contacts, err := cli.Store.Contacts.GetAllContacts(ctx)
	if err != nil {
		if errors.Is(err, store.ErrDeviceDeleted) {
			return nil, c.mapWhatsmeowErr(traceID, phoneNumber, err)
		}
		return nil, c.mapWhatsmeowErr(traceID, phoneNumber, fmt.Errorf("failed to list contacts: %w", err))
	}

	items := make([]waDomain.ContactListItem, 0, len(contacts))
	for jid, info := range contacts {
		items = append(items, waDomain.ContactListItem{
			JID:          jid.String(),
			PushName:     info.PushName,
			FullName:     info.FullName,
			FirstName:    info.FirstName,
			BusinessName: info.BusinessName,
		})
	}
	return items, nil
}

// ListGroups returns the account's joined groups via a single whatsmeow IQ
// (GetJoinedGroups). This hits the WhatsApp server, so the usecase layer caches
// it and meters it against the per-account read budget.
func (c *client) ListGroups(ctx context.Context, traceID string, phoneNumber string) ([]waDomain.GroupListItem, error) {
	cli := clients.Get(phoneNumber)
	if cli == nil {
		return nil, errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}
	if !cli.IsLoggedIn() {
		return nil, errDomain.NewError(errDomain.ErrUnauthorized, errors.New("client not logged in"))
	}

	groups, err := cli.GetJoinedGroups(ctx)
	if err != nil {
		return nil, c.mapWhatsmeowErr(traceID, phoneNumber, fmt.Errorf("failed to list groups: %w", err))
	}

	items := make([]waDomain.GroupListItem, 0, len(groups))
	for _, g := range groups {
		if g == nil {
			continue
		}
		count := g.ParticipantCount
		if count == 0 {
			count = len(g.Participants)
		}
		items = append(items, waDomain.GroupListItem{
			JID:              g.JID.String(),
			Name:             g.Name,
			Topic:            g.Topic,
			OwnerJID:         g.OwnerJID.String(),
			ParticipantCount: count,
			IsAnnounce:       g.IsAnnounce,
			IsLocked:         g.IsLocked,
			IsCommunity:      g.IsParent,
		})
	}
	return items, nil
}

// GetGroupInfo returns the full detail of one group (whatsmeow GetGroupInfo, a
// server IQ). Requires a group JID and account membership; absence/permission
// map to 404/403 via mapWhatsmeowErr sentinels.
func (c *client) GetGroupInfo(ctx context.Context, traceID string, phoneNumber string, groupJID string) (*waDomain.GroupInfoResponse, error) {
	cli := clients.Get(phoneNumber)
	if cli == nil {
		return nil, errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}
	if !cli.IsLoggedIn() {
		return nil, errDomain.NewError(errDomain.ErrUnauthorized, errors.New("client not logged in"))
	}

	jid, err := types.ParseJID(groupJID)
	if err != nil || jid.Server != types.GroupServer {
		return nil, errDomain.NewError(errDomain.ErrBadRequest,
			fmt.Errorf("chat must be a group JID (@g.us): %q", groupJID))
	}

	g, err := cli.GetGroupInfo(ctx, jid)
	if err != nil {
		return nil, c.mapWhatsmeowErr(traceID, phoneNumber, err)
	}

	return toGroupInfoResponse(g), nil
}

// toGroupInfoResponse projects a whatsmeow *types.GroupInfo into the domain DTO.
// Shared by GetGroupInfo, CreateGroup, and the invite-link preview so the
// projection lives in one place.
func toGroupInfoResponse(g *types.GroupInfo) *waDomain.GroupInfoResponse {
	if g == nil {
		return nil
	}
	participants := make([]waDomain.GroupParticipantItem, 0, len(g.Participants))
	for _, p := range g.Participants {
		participants = append(participants, waDomain.GroupParticipantItem{
			JID:          p.JID.String(),
			PhoneNumber:  jidStringOrEmpty(p.PhoneNumber),
			LID:          jidStringOrEmpty(p.LID),
			IsAdmin:      p.IsAdmin,
			IsSuperAdmin: p.IsSuperAdmin,
		})
	}
	count := g.ParticipantCount
	if count == 0 {
		count = len(participants)
	}

	return &waDomain.GroupInfoResponse{
		JID:              g.JID.String(),
		Name:             g.Name,
		Topic:            g.Topic,
		OwnerJID:         g.OwnerJID.String(),
		ParticipantCount: count,
		IsAnnounce:       g.IsAnnounce,
		IsLocked:         g.IsLocked,
		IsCommunity:      g.IsParent,
		IsEphemeral:      g.IsEphemeral,
		Participants:     participants,
	}
}

// participantResults maps whatsmeow's per-participant outcome slice into the
// canonical domain result. Error==0 is a success; Error!=0 with an AddRequest is
// a privacy-blocked add converted to an invite (not yet a member); Error!=0
// without one is a hard per-participant failure. A batch mutation returns a
// nil error with these mixed outcomes — never fail the whole call for one item.
func participantResults(ps []types.GroupParticipant) []waDomain.ParticipantResult {
	out := make([]waDomain.ParticipantResult, 0, len(ps))
	for _, p := range ps {
		r := waDomain.ParticipantResult{
			JID: p.JID.String(),
			LID: jidStringOrEmpty(p.LID),
		}
		switch {
		case p.Error == 0:
			r.Status = "ok"
		case p.AddRequest != nil:
			r.Status = "invited"
			r.Code = p.Error
			r.Invite = &waDomain.ParticipantInvite{
				Code:      p.AddRequest.Code,
				ExpiresAt: p.AddRequest.Expiration,
			}
		default:
			r.Status = "failed"
			r.Code = p.Error
		}
		out = append(out, r)
	}
	return out
}

// ListSubGroups returns the groups linked under a community (whatsmeow
// GetSubGroups, a server IQ). Requires a community @g.us JID; a non-community
// group or absence maps to 400/403/404 via the mapWhatsmeowErr IQError branch.
func (c *client) ListSubGroups(ctx context.Context, traceID string, phoneNumber string, communityJID string) ([]waDomain.SubGroupItem, error) {
	cli := clients.Get(phoneNumber)
	if cli == nil {
		return nil, errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}
	if !cli.IsLoggedIn() {
		return nil, errDomain.NewError(errDomain.ErrUnauthorized, errors.New("client not logged in"))
	}

	jid, err := types.ParseJID(communityJID)
	if err != nil || jid.Server != types.GroupServer {
		return nil, errDomain.NewError(errDomain.ErrBadRequest,
			fmt.Errorf("chat must be a community JID (@g.us): %q", communityJID))
	}

	subs, err := cli.GetSubGroups(ctx, jid)
	if err != nil {
		return nil, c.mapWhatsmeowErr(traceID, phoneNumber, err)
	}

	items := make([]waDomain.SubGroupItem, 0, len(subs))
	for _, s := range subs {
		if s == nil {
			continue
		}
		items = append(items, waDomain.SubGroupItem{
			JID:               s.JID.String(),
			Name:              s.Name,
			IsDefaultSubGroup: s.IsDefaultSubGroup,
		})
	}
	return items, nil
}

// ListCommunityParticipants returns every participant across a community's
// linked groups (whatsmeow GetLinkedGroupsParticipants, a server IQ). Requires a
// community @g.us JID; errors map via the mapWhatsmeowErr IQError branch.
func (c *client) ListCommunityParticipants(ctx context.Context, traceID string, phoneNumber string, communityJID string) ([]waDomain.CommunityParticipantItem, error) {
	cli := clients.Get(phoneNumber)
	if cli == nil {
		return nil, errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}
	if !cli.IsLoggedIn() {
		return nil, errDomain.NewError(errDomain.ErrUnauthorized, errors.New("client not logged in"))
	}

	jid, err := types.ParseJID(communityJID)
	if err != nil || jid.Server != types.GroupServer {
		return nil, errDomain.NewError(errDomain.ErrBadRequest,
			fmt.Errorf("chat must be a community JID (@g.us): %q", communityJID))
	}

	members, err := cli.GetLinkedGroupsParticipants(ctx, jid)
	if err != nil {
		return nil, c.mapWhatsmeowErr(traceID, phoneNumber, err)
	}

	items := make([]waDomain.CommunityParticipantItem, 0, len(members))
	for _, j := range members {
		items = append(items, waDomain.CommunityParticipantItem{JID: j.String()})
	}
	return items, nil
}

// loggedInClient returns the account's live client, or a 404 (no client) / 401
// (not logged in) domain error. Shared by the link-based group methods that take
// a raw invite code (no group JID to parse).
func (c *client) loggedInClient(phoneNumber string) (*whatsmeow.Client, error) {
	cli := clients.Get(phoneNumber)
	if cli == nil {
		return nil, errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}
	if !cli.IsLoggedIn() {
		return nil, errDomain.NewError(errDomain.ErrUnauthorized, errors.New("client not logged in"))
	}
	return cli, nil
}

// groupClient returns the account's live client plus the parsed group JID,
// enforcing the same nil→404 / not-logged-in→401 / non-@g.us→400 guards the read
// path uses. Defense in depth: the usecase already required @g.us upstream.
func (c *client) groupClient(phoneNumber string, groupJID string) (*whatsmeow.Client, types.JID, error) {
	cli, err := c.loggedInClient(phoneNumber)
	if err != nil {
		return nil, types.JID{}, err
	}
	jid, err := types.ParseJID(groupJID)
	if err != nil || jid.Server != types.GroupServer {
		return nil, types.JID{}, errDomain.NewError(errDomain.ErrBadRequest,
			fmt.Errorf("chat must be a group JID (@g.us): %q", groupJID))
	}
	return cli, jid, nil
}

// parseUserJIDs parses each participant string into a JID. The usecase already
// normalized/validated these (via resolveChat), so a parse failure here is a 400.
func parseUserJIDs(list []string) ([]types.JID, error) {
	out := make([]types.JID, 0, len(list))
	for _, s := range list {
		jid, err := types.ParseJID(s)
		if err != nil {
			return nil, errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("invalid participant %q", s))
		}
		out = append(out, jid)
	}
	return out, nil
}

// isSelfJID reports whether jid is the account's own number (PN, matched against
// Store.ID for @s.whatsapp.net) or own linked id (matched against Store.LID for
// @lid). Backs the remove/promote/demote self-guard's LID case.
func isSelfJID(cli *whatsmeow.Client, jid types.JID) bool {
	if cli == nil || cli.Store == nil {
		return false
	}
	if id := cli.Store.ID; id != nil && id.User == jid.User && jid.Server == types.DefaultUserServer {
		return true
	}
	if lid := cli.Store.LID; lid.User != "" && lid.User == jid.User && jid.Server == types.HiddenUserServer {
		return true
	}
	return false
}

// CreateGroup creates a group (or community when isCommunity). The account's own
// JID is added implicitly by the server; each participant's add outcome (ok /
// invited / failed) rides back in Results.
func (c *client) CreateGroup(ctx context.Context, traceID string, phoneNumber string, name string, participantJIDs []string, isCommunity bool, linkedParentJID string, isAnnounce bool, isLocked bool, isJoinApproval bool) (*waDomain.CreateGroupResponse, error) {
	cli, err := c.loggedInClient(phoneNumber)
	if err != nil {
		return nil, err
	}
	participants, err := parseUserJIDs(participantJIDs)
	if err != nil {
		return nil, err
	}

	req := whatsmeow.ReqCreateGroup{
		Name:                        name,
		Participants:                participants,
		GroupParent:                 types.GroupParent{IsParent: isCommunity},
		GroupAnnounce:               types.GroupAnnounce{IsAnnounce: isAnnounce},
		GroupLocked:                 types.GroupLocked{IsLocked: isLocked},
		GroupMembershipApprovalMode: types.GroupMembershipApprovalMode{IsJoinApprovalRequired: isJoinApproval},
	}
	if linkedParentJID != "" {
		pjid, err := types.ParseJID(linkedParentJID)
		if err != nil || pjid.Server != types.GroupServer {
			return nil, errDomain.NewError(errDomain.ErrBadRequest,
				fmt.Errorf("linked_parent_jid must be a group JID (@g.us): %q", linkedParentJID))
		}
		req.GroupLinkedParent = types.GroupLinkedParent{LinkedParentJID: pjid}
	}

	info, err := cli.CreateGroup(ctx, req)
	if err != nil {
		return nil, c.mapWhatsmeowErr(traceID, phoneNumber, err)
	}
	return &waDomain.CreateGroupResponse{
		GroupJID:  info.JID.String(),
		GroupInfo: toGroupInfoResponse(info),
		Results:   participantResults(info.Participants),
	}, nil
}

// LeaveGroup removes the account from a group. Allowed for non-admins.
func (c *client) LeaveGroup(ctx context.Context, traceID string, phoneNumber string, groupJID string) error {
	cli, jid, err := c.groupClient(phoneNumber, groupJID)
	if err != nil {
		return err
	}
	if err := cli.LeaveGroup(ctx, jid); err != nil {
		return c.mapWhatsmeowErr(traceID, phoneNumber, err)
	}
	return nil
}

// UpdateGroupParticipants adds/removes/promotes/demotes members. Per-participant
// failures ride back in the result slice (nil error); a self remove/promote/
// demote is rejected as a 400 (use LeaveGroup instead).
func (c *client) UpdateGroupParticipants(ctx context.Context, traceID string, phoneNumber string, groupJID string, action string, participantJIDs []string) ([]waDomain.ParticipantResult, error) {
	cli, jid, err := c.groupClient(phoneNumber, groupJID)
	if err != nil {
		return nil, err
	}
	participants, err := parseUserJIDs(participantJIDs)
	if err != nil {
		return nil, err
	}
	if action != string(whatsmeow.ParticipantChangeAdd) {
		for _, p := range participants {
			if isSelfJID(cli, p) {
				return nil, errDomain.NewError(errDomain.ErrBadRequest,
					errors.New("use POST /group/leave to remove yourself"))
			}
		}
	}
	res, err := cli.UpdateGroupParticipants(ctx, jid, participants, whatsmeow.ParticipantChange(action))
	if err != nil {
		return nil, c.mapWhatsmeowErr(traceID, phoneNumber, err)
	}
	return participantResults(res), nil
}

// SetGroupAnnounce toggles announce mode (only admins can send).
func (c *client) SetGroupAnnounce(ctx context.Context, traceID string, phoneNumber string, groupJID string, announce bool) error {
	cli, jid, err := c.groupClient(phoneNumber, groupJID)
	if err != nil {
		return err
	}
	if err := cli.SetGroupAnnounce(ctx, jid, announce); err != nil {
		return c.mapWhatsmeowErr(traceID, phoneNumber, err)
	}
	return nil
}

// SetGroupLocked toggles locked mode (only admins can edit group info).
func (c *client) SetGroupLocked(ctx context.Context, traceID string, phoneNumber string, groupJID string, locked bool) error {
	cli, jid, err := c.groupClient(phoneNumber, groupJID)
	if err != nil {
		return err
	}
	if err := cli.SetGroupLocked(ctx, jid, locked); err != nil {
		return c.mapWhatsmeowErr(traceID, phoneNumber, err)
	}
	return nil
}

// SetGroupName updates the group name (subject).
func (c *client) SetGroupName(ctx context.Context, traceID string, phoneNumber string, groupJID string, name string) error {
	cli, jid, err := c.groupClient(phoneNumber, groupJID)
	if err != nil {
		return err
	}
	if err := cli.SetGroupName(ctx, jid, name); err != nil {
		return c.mapWhatsmeowErr(traceID, phoneNumber, err)
	}
	return nil
}

// SetGroupTopic updates the group topic (description). Passing empty previous/new
// IDs lets whatsmeow fetch the current topic ID and generate a new one itself.
func (c *client) SetGroupTopic(ctx context.Context, traceID string, phoneNumber string, groupJID string, topic string) error {
	cli, jid, err := c.groupClient(phoneNumber, groupJID)
	if err != nil {
		return err
	}
	if err := cli.SetGroupTopic(ctx, jid, "", "", topic); err != nil {
		return c.mapWhatsmeowErr(traceID, phoneNumber, err)
	}
	return nil
}

// SetGroupPhoto sets (or, with a nil photo, removes) the group picture. Returns
// the new picture ID, or "remove" when the photo was cleared.
func (c *client) SetGroupPhoto(ctx context.Context, traceID string, phoneNumber string, groupJID string, photo []byte) (string, error) {
	cli, jid, err := c.groupClient(phoneNumber, groupJID)
	if err != nil {
		return "", err
	}
	id, err := cli.SetGroupPhoto(ctx, jid, photo)
	if err != nil {
		return "", c.mapWhatsmeowErr(traceID, phoneNumber, err)
	}
	return id, nil
}

// GetGroupInviteLink returns the group's invite link, revoking and regenerating
// it first when reset is true. Requires admin (401/403 → 403).
func (c *client) GetGroupInviteLink(ctx context.Context, traceID string, phoneNumber string, groupJID string, reset bool) (string, error) {
	cli, jid, err := c.groupClient(phoneNumber, groupJID)
	if err != nil {
		return "", err
	}
	link, err := cli.GetGroupInviteLink(ctx, jid, reset)
	if err != nil {
		return "", c.mapWhatsmeowErr(traceID, phoneNumber, err)
	}
	return link, nil
}

// JoinGroupWithLink joins a group by invite code (a full chat.whatsapp.com link
// or a bare code — whatsmeow strips the prefix). Returns the resulting JID (the
// group, or a membership-approval request; whatsmeow does not expose which).
func (c *client) JoinGroupWithLink(ctx context.Context, traceID string, phoneNumber string, code string) (string, error) {
	cli, err := c.loggedInClient(phoneNumber)
	if err != nil {
		return "", err
	}
	jid, err := cli.JoinGroupWithLink(ctx, code)
	if err != nil {
		return "", c.mapWhatsmeowErr(traceID, phoneNumber, err)
	}
	return jid.String(), nil
}

// GetGroupInfoFromLink previews a group from an invite code without joining.
func (c *client) GetGroupInfoFromLink(ctx context.Context, traceID string, phoneNumber string, code string) (*waDomain.GroupInfoResponse, error) {
	cli, err := c.loggedInClient(phoneNumber)
	if err != nil {
		return nil, err
	}
	g, err := cli.GetGroupInfoFromLink(ctx, code)
	if err != nil {
		return nil, c.mapWhatsmeowErr(traceID, phoneNumber, err)
	}
	return toGroupInfoResponse(g), nil
}

// GetGroupRequestParticipants lists a group's pending join requests. Requires admin.
func (c *client) GetGroupRequestParticipants(ctx context.Context, traceID string, phoneNumber string, groupJID string) ([]waDomain.GroupJoinRequestItem, error) {
	cli, jid, err := c.groupClient(phoneNumber, groupJID)
	if err != nil {
		return nil, err
	}
	reqs, err := cli.GetGroupRequestParticipants(ctx, jid)
	if err != nil {
		return nil, c.mapWhatsmeowErr(traceID, phoneNumber, err)
	}
	items := make([]waDomain.GroupJoinRequestItem, 0, len(reqs))
	for _, r := range reqs {
		item := waDomain.GroupJoinRequestItem{JID: r.JID.String()}
		if !r.RequestedAt.IsZero() {
			item.RequestedAt = r.RequestedAt.Format(time.RFC3339)
		}
		items = append(items, item)
	}
	return items, nil
}

// UpdateGroupRequestParticipants approves or rejects pending join requests.
// Per-participant failures ride back in the result slice (nil error).
func (c *client) UpdateGroupRequestParticipants(ctx context.Context, traceID string, phoneNumber string, groupJID string, participantJIDs []string, approve bool) ([]waDomain.ParticipantResult, error) {
	cli, jid, err := c.groupClient(phoneNumber, groupJID)
	if err != nil {
		return nil, err
	}
	participants, err := parseUserJIDs(participantJIDs)
	if err != nil {
		return nil, err
	}
	action := whatsmeow.ParticipantChangeReject
	if approve {
		action = whatsmeow.ParticipantChangeApprove
	}
	res, err := cli.UpdateGroupRequestParticipants(ctx, jid, participants, action)
	if err != nil {
		return nil, c.mapWhatsmeowErr(traceID, phoneNumber, err)
	}
	return participantResults(res), nil
}

// LinkSubGroup links a child group under a parent community. Requires admin on both.
func (c *client) LinkSubGroup(ctx context.Context, traceID string, phoneNumber string, parentJID string, childJID string) error {
	cli, parent, err := c.groupClient(phoneNumber, parentJID)
	if err != nil {
		return err
	}
	child, err := types.ParseJID(childJID)
	if err != nil || child.Server != types.GroupServer {
		return errDomain.NewError(errDomain.ErrBadRequest,
			fmt.Errorf("child_jid must be a group JID (@g.us): %q", childJID))
	}
	if err := cli.LinkGroup(ctx, parent, child); err != nil {
		return c.mapWhatsmeowErr(traceID, phoneNumber, err)
	}
	return nil
}

// UnlinkSubGroup removes a child group from a parent community.
func (c *client) UnlinkSubGroup(ctx context.Context, traceID string, phoneNumber string, parentJID string, childJID string) error {
	cli, parent, err := c.groupClient(phoneNumber, parentJID)
	if err != nil {
		return err
	}
	child, err := types.ParseJID(childJID)
	if err != nil || child.Server != types.GroupServer {
		return errDomain.NewError(errDomain.ErrBadRequest,
			fmt.Errorf("child_jid must be a group JID (@g.us): %q", childJID))
	}
	if err := cli.UnlinkGroup(ctx, parent, child); err != nil {
		return c.mapWhatsmeowErr(traceID, phoneNumber, err)
	}
	return nil
}

// GetContactInfo returns a server-side profile lookup for one user (whatsmeow
// GetUserInfo). An unknown number simply yields a response with the JID only.
func (c *client) GetContactInfo(ctx context.Context, traceID string, phoneNumber string, userJID string) (*waDomain.ContactInfoResponse, error) {
	cli := clients.Get(phoneNumber)
	if cli == nil {
		return nil, errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}
	if !cli.IsLoggedIn() {
		return nil, errDomain.NewError(errDomain.ErrUnauthorized, errors.New("client not logged in"))
	}

	jid, err := types.ParseJID(userJID)
	if err != nil {
		return nil, errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("invalid recipient %q", userJID))
	}

	infos, err := cli.GetUserInfo(ctx, []types.JID{jid})
	if err != nil {
		return nil, c.mapWhatsmeowErr(traceID, phoneNumber, err)
	}

	resp := &waDomain.ContactInfoResponse{JID: jid.String()}
	info, ok := infos[jid]
	if !ok {
		return resp, nil
	}
	resp.Status = info.Status
	resp.PictureID = info.PictureID
	resp.DeviceCount = len(info.Devices)
	resp.LID = jidStringOrEmpty(info.LID)
	if info.VerifiedName != nil && info.VerifiedName.Details != nil {
		resp.VerifiedName = info.VerifiedName.Details.GetVerifiedName()
	}
	return resp, nil
}

// GetAvatar returns a chat's profile picture info (whatsmeow
// GetProfilePictureInfo, a server IQ). A (nil, nil) return means the picture is
// unchanged relative to existingID — the usecase turns that into a 304. Absent
// / hidden pictures map to 404 / 403 via mapWhatsmeowErr sentinels.
//
// ponytail: IsCommunity defaults false, so a community *parent* avatar may need
// a dedicated flag later; regular groups and users work as-is.
func (c *client) GetAvatar(ctx context.Context, traceID string, phoneNumber string, targetJID string, preview bool, existingID string) (*waDomain.AvatarResponse, error) {
	cli := clients.Get(phoneNumber)
	if cli == nil {
		return nil, errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}
	if !cli.IsLoggedIn() {
		return nil, errDomain.NewError(errDomain.ErrUnauthorized, errors.New("client not logged in"))
	}

	jid, err := types.ParseJID(targetJID)
	if err != nil {
		return nil, errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("invalid recipient %q", targetJID))
	}

	info, err := cli.GetProfilePictureInfo(ctx, jid, &whatsmeow.GetProfilePictureParams{
		Preview:    preview,
		ExistingID: existingID,
	})
	if err != nil {
		return nil, c.mapWhatsmeowErr(traceID, phoneNumber, err)
	}
	if info == nil {
		// ExistingID matched the current picture — unchanged.
		return nil, nil
	}

	return &waDomain.AvatarResponse{
		JID:        jid.String(),
		URL:        info.URL,
		ID:         info.ID,
		Type:       info.Type,
		DirectPath: info.DirectPath,
	}, nil
}

// MarkRead marks messages in a chat as read. For a group chat the sender (the
// message author) must be supplied so the receipt is attributed. Exactly one
// receipt type is passed to whatsmeow, avoiding its multi-type panic.
func (c *client) MarkRead(ctx context.Context, traceID string, phoneNumber string, chat string, sender string, messageIDs []string) error {
	cli := clients.Get(phoneNumber)
	if cli == nil {
		return errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}
	if !cli.IsLoggedIn() {
		return errDomain.NewError(errDomain.ErrUnauthorized, errors.New("client not logged in"))
	}
	if len(messageIDs) == 0 {
		return errDomain.NewError(errDomain.ErrBadRequest, errors.New("message_ids is required"))
	}

	chatJID, err := types.ParseJID(chat)
	if err != nil {
		return errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("invalid recipient %q", chat))
	}

	var senderJID types.JID
	if sender != "" {
		senderJID, err = types.ParseJID(sender)
		if err != nil {
			return errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("invalid sender %q", sender))
		}
	}
	if chatJID.Server == types.GroupServer && senderJID.IsEmpty() {
		return errDomain.NewError(errDomain.ErrBadRequest, errors.New("sender is required for group chats"))
	}

	ids := make([]types.MessageID, len(messageIDs))
	for i, id := range messageIDs {
		ids[i] = types.MessageID(id)
	}

	if err := cli.MarkRead(ctx, ids, time.Now(), chatJID, senderJID); err != nil {
		return c.mapWhatsmeowErr(traceID, phoneNumber, err)
	}
	return nil
}

// SendChatPresence sets the typing indicator in a chat. state is the resolved
// whatsmeow ChatPresence ("composing"/"paused") and media is "" or "audio".
func (c *client) SendChatPresence(ctx context.Context, traceID string, phoneNumber string, chat string, state string, media string) error {
	cli := clients.Get(phoneNumber)
	if cli == nil {
		return errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}
	if !cli.IsLoggedIn() {
		return errDomain.NewError(errDomain.ErrUnauthorized, errors.New("client not logged in"))
	}

	chatJID, err := types.ParseJID(chat)
	if err != nil {
		return errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("invalid recipient %q", chat))
	}

	if err := cli.SendChatPresence(ctx, chatJID, types.ChatPresence(state), types.ChatPresenceMedia(media)); err != nil {
		return c.mapWhatsmeowErr(traceID, phoneNumber, err)
	}
	return nil
}

// buildContextInfo turns the caller-supplied reply + mentions metadata into a
// whatsmeow ContextInfo, or nil when there is nothing to attach. Replies are
// storeless (decision #5): the quoted preview is whatever text the caller
// supplies. Fields are expected to already be canonical JIDs (resolved upstream).
func buildContextInfo(msgCtx *waDomain.MessageContext) *waE2E.ContextInfo {
	if msgCtx.IsEmpty() {
		return nil
	}
	ci := &waE2E.ContextInfo{}
	if msgCtx.ReplyToID != "" {
		ci.StanzaID = proto.String(msgCtx.ReplyToID)
		if msgCtx.ReplyToSender != "" {
			ci.Participant = proto.String(msgCtx.ReplyToSender)
		}
		// Quoted preview: caller-supplied text (may be empty).
		ci.QuotedMessage = &waE2E.Message{Conversation: proto.String(msgCtx.ReplyToText)}
	}
	if len(msgCtx.Mentions) > 0 {
		ci.MentionedJID = msgCtx.Mentions
	}
	return ci
}

// jidStringOrEmpty returns the JID string, or "" for the zero JID (so empty
// optional fields are omitted rather than rendered as "@s.whatsapp.net").
func jidStringOrEmpty(j types.JID) string {
	if j.IsEmpty() {
		return ""
	}
	return j.String()
}

func (c *client) GetWebhookURL(ctx context.Context, traceID string, phoneNumber string) (*string, error) {
	cli := clients.Get(phoneNumber)
	if cli == nil {
		return nil, errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}

	webhookURL, err := c.repository.GetWebhook(ctx, cli.Store.ID.String())
	if err != nil {
		c.logger.Error(traceID, fmt.Sprintf("Failed to get webhook URL for %s", MaskedPhoneNumber(phoneNumber)), nil, customLog.Error(err))
		return nil, errDomain.NewError(errDomain.ErrInternalFailure, err)
	}

	if webhookURL == nil {
		return nil, nil
	}

	return &webhookURL.Url, nil
}

func (c *client) SetWebhookURL(ctx context.Context, traceID string, phoneNumber string, webhook *waDomain.Webhook) error {
	cli := clients.Get(phoneNumber)
	if cli == nil {
		return errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}

	err := c.repository.SetWebhook(ctx, cli.Store.ID.String(), webhook.Url, webhook.HmacSecret)
	if err != nil {
		c.logger.Error(traceID, fmt.Sprintf("Failed to set webhook URL for %s", MaskedPhoneNumber(phoneNumber)), nil, customLog.Error(err))
		return errDomain.NewError(errDomain.ErrInternalFailure, err)
	}

	return nil
}

func (c *client) DeleteWebhookURL(ctx context.Context, traceID string, phoneNumber string) error {
	cli := clients.Get(phoneNumber)
	if cli == nil {
		return errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}

	err := c.repository.DeleteWebhook(ctx, cli.Store.ID.String())
	if err != nil {
		c.logger.Error(traceID, fmt.Sprintf("Failed to delete webhook URL for %s", MaskedPhoneNumber(phoneNumber)), nil, customLog.Error(err))
		return errDomain.NewError(errDomain.ErrInternalFailure, err)
	}

	return nil
}

func (c *client) SendTextMessage(ctx context.Context, traceID string, phoneNumber string, to string, message string, msgCtx *waDomain.MessageContext) (string, error) {
	cli := clients.Get(phoneNumber)
	if cli == nil {
		return "", errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}

	if !cli.IsLoggedIn() {
		return "", errDomain.NewError(errDomain.ErrUnauthorized, errors.New("client not logged in"))
	}

	toJID, err := types.ParseJID(to)
	if err != nil {
		return "", errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("invalid JID format: %w", err))
	}

	// Plain text uses Conversation; a reply or mentions require ExtendedTextMessage
	// so a ContextInfo can be attached.
	var msg *waE2E.Message
	if ci := buildContextInfo(msgCtx); ci != nil {
		msg = &waE2E.Message{
			ExtendedTextMessage: &waE2E.ExtendedTextMessage{
				Text:        proto.String(message),
				ContextInfo: ci,
			},
		}
	} else {
		msg = &waE2E.Message{Conversation: proto.String(message)}
	}

	resp, err := cli.SendMessage(ctx, toJID, msg)
	if err != nil {
		if errors.Is(err, store.ErrDeviceDeleted) {
			return "", c.mapWhatsmeowErr(traceID, phoneNumber, err)
		}
		return "", c.mapWhatsmeowErr(traceID, phoneNumber, fmt.Errorf("failed to send message: %w", err))
	}

	return resp.ID, nil
}

func (c *client) SendImageMessage(ctx context.Context, traceID string, phoneNumber string, to string, imageBytes []byte, mimeType string, caption string, isViewOnce bool, msgCtx *waDomain.MessageContext) (string, error) {
	cli := clients.Get(phoneNumber)
	if cli == nil {
		return "", errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}

	if !cli.IsLoggedIn() {
		return "", errDomain.NewError(errDomain.ErrUnauthorized, errors.New("client not logged in"))
	}

	toJID, err := types.ParseJID(to)
	if err != nil {
		return "", errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("invalid JID format: %w", err))
	}

	// Upload image to WhatsApp servers
	uploaded, err := cli.Upload(ctx, imageBytes, whatsmeow.MediaImage)
	if err != nil {
		if errors.Is(err, store.ErrDeviceDeleted) {
			return "", c.mapWhatsmeowErr(traceID, phoneNumber, err)
		}
		return "", errDomain.NewError(errDomain.ErrInternalFailure, fmt.Errorf("failed to upload image: %w", err))
	}

	// Build image message
	imageMsg := &waE2E.ImageMessage{
		URL:           proto.String(uploaded.URL),
		DirectPath:    proto.String(uploaded.DirectPath),
		MediaKey:      uploaded.MediaKey,
		Mimetype:      proto.String(mimeType),
		FileEncSHA256: uploaded.FileEncSHA256,
		FileSHA256:    uploaded.FileSHA256,
		FileLength:    proto.Uint64(uint64(len(imageBytes))),
		ViewOnce:      &isViewOnce,
	}

	if caption != "" {
		imageMsg.Caption = proto.String(caption)
	}

	// Attach reply/mentions on the inner message before any ViewOnce wrap so it survives.
	imageMsg.ContextInfo = buildContextInfo(msgCtx)

	var msg *waE2E.Message
	if isViewOnce {
		msg = &waE2E.Message{
			ViewOnceMessage: &waE2E.FutureProofMessage{
				Message: &waE2E.Message{
					ImageMessage: imageMsg,
				},
			},
		}
	} else {
		msg = &waE2E.Message{
			ImageMessage: imageMsg,
		}
	}

	resp, err := cli.SendMessage(ctx, toJID, msg)
	if err != nil {
		if errors.Is(err, store.ErrDeviceDeleted) {
			return "", c.mapWhatsmeowErr(traceID, phoneNumber, err)
		}
		return "", c.mapWhatsmeowErr(traceID, phoneNumber, fmt.Errorf("failed to send image message: %w", err))
	}

	return resp.ID, nil
}

func (c *client) SendAudioMessage(ctx context.Context, traceID string, phoneNumber string, to string, audioBytes []byte, mimeType string, isPTT bool, isViewOnce bool, msgCtx *waDomain.MessageContext) (string, error) {
	cli := clients.Get(phoneNumber)
	if cli == nil {
		return "", errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}
	if !cli.IsLoggedIn() {
		return "", errDomain.NewError(errDomain.ErrUnauthorized, errors.New("client not logged in"))
	}

	toJID, err := types.ParseJID(to)
	if err != nil {
		return "", errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("invalid JID format: %w", err))
	}

	// Voice notes must be opus ogg; clients render the PTT waveform only for
	// that mimetype. DetectContentType can't see ogg, so trust the caller's
	// extension/mime and default to the opus mimetype for PTT.
	if isPTT && mimeType == "" {
		mimeType = "audio/ogg; codecs=opus"
	} else if mimeType == "" {
		mimeType = "audio/mpeg"
	}

	uploaded, err := cli.Upload(ctx, audioBytes, whatsmeow.MediaAudio)
	if err != nil {
		if errors.Is(err, store.ErrDeviceDeleted) {
			return "", c.mapWhatsmeowErr(traceID, phoneNumber, err)
		}
		return "", errDomain.NewError(errDomain.ErrInternalFailure, fmt.Errorf("failed to upload audio: %w", err))
	}

	audioMsg := &waE2E.AudioMessage{
		URL:           proto.String(uploaded.URL),
		DirectPath:    proto.String(uploaded.DirectPath),
		MediaKey:      uploaded.MediaKey,
		Mimetype:      proto.String(mimeType),
		FileEncSHA256: uploaded.FileEncSHA256,
		FileSHA256:    uploaded.FileSHA256,
		FileLength:    proto.Uint64(uint64(len(audioBytes))),
		PTT:           proto.Bool(isPTT),
		ViewOnce:      proto.Bool(isViewOnce),
	}
	audioMsg.ContextInfo = buildContextInfo(msgCtx)

	msg := &waE2E.Message{AudioMessage: audioMsg}
	if isViewOnce {
		msg = &waE2E.Message{
			ViewOnceMessage: &waE2E.FutureProofMessage{
				Message: &waE2E.Message{AudioMessage: audioMsg},
			},
		}
	}

	resp, err := cli.SendMessage(ctx, toJID, msg)
	if err != nil {
		if errors.Is(err, store.ErrDeviceDeleted) {
			return "", c.mapWhatsmeowErr(traceID, phoneNumber, err)
		}
		return "", c.mapWhatsmeowErr(traceID, phoneNumber, fmt.Errorf("failed to send audio message: %w", err))
	}
	return resp.ID, nil
}

func (c *client) SendVideoMessage(ctx context.Context, traceID string, phoneNumber string, to string, videoBytes []byte, mimeType string, caption string, isGif bool, isViewOnce bool, msgCtx *waDomain.MessageContext) (string, error) {
	cli := clients.Get(phoneNumber)
	if cli == nil {
		return "", errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}
	if !cli.IsLoggedIn() {
		return "", errDomain.NewError(errDomain.ErrUnauthorized, errors.New("client not logged in"))
	}

	toJID, err := types.ParseJID(to)
	if err != nil {
		return "", errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("invalid JID format: %w", err))
	}

	uploaded, err := cli.Upload(ctx, videoBytes, whatsmeow.MediaVideo)
	if err != nil {
		if errors.Is(err, store.ErrDeviceDeleted) {
			return "", c.mapWhatsmeowErr(traceID, phoneNumber, err)
		}
		return "", errDomain.NewError(errDomain.ErrInternalFailure, fmt.Errorf("failed to upload video: %w", err))
	}

	videoMsg := &waE2E.VideoMessage{
		URL:           proto.String(uploaded.URL),
		DirectPath:    proto.String(uploaded.DirectPath),
		MediaKey:      uploaded.MediaKey,
		Mimetype:      proto.String(mimeType),
		FileEncSHA256: uploaded.FileEncSHA256,
		FileSHA256:    uploaded.FileSHA256,
		FileLength:    proto.Uint64(uint64(len(videoBytes))),
		GifPlayback:   proto.Bool(isGif),
		ViewOnce:      proto.Bool(isViewOnce),
	}
	if caption != "" {
		videoMsg.Caption = proto.String(caption)
	}
	videoMsg.ContextInfo = buildContextInfo(msgCtx)

	msg := &waE2E.Message{VideoMessage: videoMsg}
	if isViewOnce {
		msg = &waE2E.Message{
			ViewOnceMessage: &waE2E.FutureProofMessage{
				Message: &waE2E.Message{VideoMessage: videoMsg},
			},
		}
	}

	resp, err := cli.SendMessage(ctx, toJID, msg)
	if err != nil {
		if errors.Is(err, store.ErrDeviceDeleted) {
			return "", c.mapWhatsmeowErr(traceID, phoneNumber, err)
		}
		return "", c.mapWhatsmeowErr(traceID, phoneNumber, fmt.Errorf("failed to send video message: %w", err))
	}
	return resp.ID, nil
}

func (c *client) SendDocumentMessage(ctx context.Context, traceID string, phoneNumber string, to string, docBytes []byte, mimeType string, fileName string, caption string, msgCtx *waDomain.MessageContext) (string, error) {
	cli := clients.Get(phoneNumber)
	if cli == nil {
		return "", errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}
	if !cli.IsLoggedIn() {
		return "", errDomain.NewError(errDomain.ErrUnauthorized, errors.New("client not logged in"))
	}

	toJID, err := types.ParseJID(to)
	if err != nil {
		return "", errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("invalid JID format: %w", err))
	}

	uploaded, err := cli.Upload(ctx, docBytes, whatsmeow.MediaDocument)
	if err != nil {
		if errors.Is(err, store.ErrDeviceDeleted) {
			return "", c.mapWhatsmeowErr(traceID, phoneNumber, err)
		}
		return "", errDomain.NewError(errDomain.ErrInternalFailure, fmt.Errorf("failed to upload document: %w", err))
	}

	docMsg := &waE2E.DocumentMessage{
		URL:           proto.String(uploaded.URL),
		DirectPath:    proto.String(uploaded.DirectPath),
		MediaKey:      uploaded.MediaKey,
		Mimetype:      proto.String(mimeType),
		Title:         proto.String(fileName),
		FileName:      proto.String(fileName),
		FileEncSHA256: uploaded.FileEncSHA256,
		FileSHA256:    uploaded.FileSHA256,
		FileLength:    proto.Uint64(uint64(len(docBytes))),
	}
	if caption != "" {
		docMsg.Caption = proto.String(caption)
	}
	docMsg.ContextInfo = buildContextInfo(msgCtx)

	// Captions on documents render reliably only when wrapped in a
	// DocumentWithCaptionMessage (FutureProofMessage); a bare DocumentMessage
	// caption is dropped by many clients.
	var msg *waE2E.Message
	if caption != "" {
		msg = &waE2E.Message{
			DocumentWithCaptionMessage: &waE2E.FutureProofMessage{
				Message: &waE2E.Message{DocumentMessage: docMsg},
			},
		}
	} else {
		msg = &waE2E.Message{DocumentMessage: docMsg}
	}

	resp, err := cli.SendMessage(ctx, toJID, msg)
	if err != nil {
		if errors.Is(err, store.ErrDeviceDeleted) {
			return "", c.mapWhatsmeowErr(traceID, phoneNumber, err)
		}
		return "", c.mapWhatsmeowErr(traceID, phoneNumber, fmt.Errorf("failed to send document message: %w", err))
	}
	return resp.ID, nil
}

func (c *client) ReactToMessage(ctx context.Context, traceID string, phoneNumber string, chatJID string, senderJID string, messageID string, emoji string) error {
	cli := clients.Get(phoneNumber)
	if cli == nil {
		return errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}

	if !cli.IsLoggedIn() {
		return errDomain.NewError(errDomain.ErrUnauthorized, errors.New("client not logged in"))
	}

	toJID, err := types.ParseJID(chatJID)
	if err != nil {
		return errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("invalid JID format: %w", err))
	}

	// Resolve the original message's sender. An empty sender means we are
	// reacting to our own outgoing message (FromMe=true). BuildReaction +
	// BuildMessageKey set FromMe/Participant from this, so reactions now
	// attribute correctly in both DMs and groups and on your own messages.
	var sender types.JID
	if senderJID != "" {
		sender, err = types.ParseJID(senderJID)
		if err != nil {
			return errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("invalid sender JID format: %w", err))
		}
	}

	msg := cli.BuildReaction(toJID, sender, messageID, emoji)

	_, err = cli.SendMessage(ctx, toJID, msg)
	if err != nil {
		if errors.Is(err, store.ErrDeviceDeleted) {
			return c.mapWhatsmeowErr(traceID, phoneNumber, err)
		}
		return c.mapWhatsmeowErr(traceID, phoneNumber, fmt.Errorf("failed to send reaction: %w", err))
	}

	return nil
}

func (c *client) DeleteMessage(ctx context.Context, traceID string, phoneNumber string, chatJID string, messageID string) error {
	cli := clients.Get(phoneNumber)
	if cli == nil {
		return errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}

	if !cli.IsLoggedIn() {
		return errDomain.NewError(errDomain.ErrUnauthorized, errors.New("client not logged in"))
	}

	toJID, err := types.ParseJID(chatJID)
	if err != nil {
		return errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("invalid JID format: %w", err))
	}

	// Build revoke message
	_, err = cli.SendMessage(ctx, toJID, cli.BuildRevoke(toJID, types.EmptyJID, messageID))
	if err != nil {
		if errors.Is(err, store.ErrDeviceDeleted) {
			return c.mapWhatsmeowErr(traceID, phoneNumber, err)
		}
		return errDomain.NewError(errDomain.ErrInternalFailure, fmt.Errorf("failed to delete message: %w", err))
	}

	return nil
}

func (c *client) EditMessage(ctx context.Context, traceID string, phoneNumber string, chatJID string, messageID string, newText string) error {
	cli := clients.Get(phoneNumber)
	if cli == nil {
		return errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}

	if !cli.IsLoggedIn() {
		return errDomain.NewError(errDomain.ErrUnauthorized, errors.New("client not logged in"))
	}

	toJID, err := types.ParseJID(chatJID)
	if err != nil {
		return errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("invalid JID format: %w", err))
	}

	// Build edit message
	editMsg := &waE2E.Message{
		Conversation: proto.String(newText),
	}

	_, err = cli.SendMessage(ctx, toJID, cli.BuildEdit(toJID, messageID, editMsg))
	if err != nil {
		if errors.Is(err, store.ErrDeviceDeleted) {
			return c.mapWhatsmeowErr(traceID, phoneNumber, err)
		}
		return errDomain.NewError(errDomain.ErrInternalFailure, fmt.Errorf("failed to edit message: %w", err))
	}

	return nil
}

func (c *client) SendLocationMessage(ctx context.Context, traceID string, phoneNumber string, to string, latitude float64, longitude float64, name string, address string, msgCtx *waDomain.MessageContext) (string, error) {
	cli := clients.Get(phoneNumber)
	if cli == nil {
		return "", errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}
	if !cli.IsLoggedIn() {
		return "", errDomain.NewError(errDomain.ErrUnauthorized, errors.New("client not logged in"))
	}

	toJID, err := types.ParseJID(to)
	if err != nil {
		return "", errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("invalid JID format: %w", err))
	}

	locationMsg := &waE2E.LocationMessage{
		DegreesLatitude:  &latitude,
		DegreesLongitude: &longitude,
	}
	if name != "" {
		locationMsg.Name = &name
	}
	if address != "" {
		locationMsg.Address = &address
	}
	locationMsg.ContextInfo = buildContextInfo(msgCtx)

	resp, err := cli.SendMessage(ctx, toJID, &waE2E.Message{LocationMessage: locationMsg})
	if err != nil {
		if errors.Is(err, store.ErrDeviceDeleted) {
			return "", c.mapWhatsmeowErr(traceID, phoneNumber, err)
		}
		return "", c.mapWhatsmeowErr(traceID, phoneNumber, fmt.Errorf("failed to send location message: %w", err))
	}
	return resp.ID, nil
}

func (c *client) SendPollMessage(ctx context.Context, traceID string, phoneNumber string, to string, question string, options []string, selectableCount int, msgCtx *waDomain.MessageContext) (string, error) {
	cli := clients.Get(phoneNumber)
	if cli == nil {
		return "", errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}
	if !cli.IsLoggedIn() {
		return "", errDomain.NewError(errDomain.ErrUnauthorized, errors.New("client not logged in"))
	}

	toJID, err := types.ParseJID(to)
	if err != nil {
		return "", errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("invalid JID format: %w", err))
	}

	// Use BuildPollCreation so the message carries the 32-byte MessageSecret
	// required to decrypt votes. Hand-building PollCreationMessage produces a
	// poll that can never be voted on (votes arrive undecryptable).
	if selectableCount <= 0 {
		selectableCount = 1
	}

	msg := cli.BuildPollCreation(question, options, selectableCount)

	// Attach reply/mentions to the poll body only. Never touch
	// msg.MessageContextInfo — it carries the poll's MessageSecret used to
	// decrypt votes; overwriting it makes the poll unvotable.
	if msg.PollCreationMessage != nil {
		msg.PollCreationMessage.ContextInfo = buildContextInfo(msgCtx)
	}

	resp, err := cli.SendMessage(ctx, toJID, msg)
	if err != nil {
		if errors.Is(err, store.ErrDeviceDeleted) {
			return "", c.mapWhatsmeowErr(traceID, phoneNumber, err)
		}
		return "", c.mapWhatsmeowErr(traceID, phoneNumber, fmt.Errorf("failed to send poll message: %w", err))
	}
	return resp.ID, nil
}

func (c *client) SendStickerMessage(ctx context.Context, traceID string, phoneNumber string, to string, stickerBytes []byte, mimeType string, msgCtx *waDomain.MessageContext) (string, error) {
	cli := clients.Get(phoneNumber)
	if cli == nil {
		return "", errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}
	if !cli.IsLoggedIn() {
		return "", errDomain.NewError(errDomain.ErrUnauthorized, errors.New("client not logged in"))
	}

	toJID, err := types.ParseJID(to)
	if err != nil {
		return "", errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("invalid JID format: %w", err))
	}

	uploaded, err := cli.Upload(ctx, stickerBytes, whatsmeow.MediaImage)
	if err != nil {
		if errors.Is(err, store.ErrDeviceDeleted) {
			return "", c.mapWhatsmeowErr(traceID, phoneNumber, err)
		}
		return "", errDomain.NewError(errDomain.ErrInternalFailure, fmt.Errorf("failed to upload sticker: %w", err))
	}

	fileLen := uint64(len(stickerBytes))
	resp, err := cli.SendMessage(ctx, toJID, &waE2E.Message{
		StickerMessage: &waE2E.StickerMessage{
			URL:           &uploaded.URL,
			DirectPath:    &uploaded.DirectPath,
			MediaKey:      uploaded.MediaKey,
			Mimetype:      &mimeType,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    &fileLen,
			ContextInfo:   buildContextInfo(msgCtx),
		},
	})
	if err != nil {
		if errors.Is(err, store.ErrDeviceDeleted) {
			return "", c.mapWhatsmeowErr(traceID, phoneNumber, err)
		}
		return "", c.mapWhatsmeowErr(traceID, phoneNumber, fmt.Errorf("failed to send sticker message: %w", err))
	}
	return resp.ID, nil
}
