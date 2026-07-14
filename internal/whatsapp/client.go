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
		SendDocumentMessage(ctx context.Context, traceID string, phoneNumber string, to string, docBytes []byte, mimeType string, fileName string, caption string) (string, error)
		SendLocationMessage(ctx context.Context, traceID string, phoneNumber string, to string, latitude float64, longitude float64, name string, address string) (string, error)
		SendPollMessage(ctx context.Context, traceID string, phoneNumber string, to string, question string, options []string, selectableCount int) (string, error)
		SendStickerMessage(ctx context.Context, traceID string, phoneNumber string, to string, stickerBytes []byte, mimeType string) (string, error)
		ReactToMessage(ctx context.Context, traceID string, phoneNumber string, chatJID string, senderJID string, messageID string, emoji string) error
		DeleteMessage(ctx context.Context, traceID string, phoneNumber string, chatJID string, messageID string) error
		EditMessage(ctx context.Context, traceID string, phoneNumber string, chatJID string, messageID string, newText string) error
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
	// Recipient problems (e.g. "can't send message to unknown server") are
	// caller input errors, not server faults — surface as 400, not 500.
	if isRecipientError(err) {
		return errDomain.NewError(errDomain.ErrBadRequest, err)
	}
	return errDomain.NewError(errDomain.ErrInternalFailure, err)
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
	}, nil
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

func (c *client) SendDocumentMessage(ctx context.Context, traceID string, phoneNumber string, to string, docBytes []byte, mimeType string, fileName string, caption string) (string, error) {
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

func (c *client) SendLocationMessage(ctx context.Context, traceID string, phoneNumber string, to string, latitude float64, longitude float64, name string, address string) (string, error) {
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

	resp, err := cli.SendMessage(ctx, toJID, &waE2E.Message{LocationMessage: locationMsg})
	if err != nil {
		if errors.Is(err, store.ErrDeviceDeleted) {
			return "", c.mapWhatsmeowErr(traceID, phoneNumber, err)
		}
		return "", c.mapWhatsmeowErr(traceID, phoneNumber, fmt.Errorf("failed to send location message: %w", err))
	}
	return resp.ID, nil
}

func (c *client) SendPollMessage(ctx context.Context, traceID string, phoneNumber string, to string, question string, options []string, selectableCount int) (string, error) {
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

	resp, err := cli.SendMessage(ctx, toJID, msg)
	if err != nil {
		if errors.Is(err, store.ErrDeviceDeleted) {
			return "", c.mapWhatsmeowErr(traceID, phoneNumber, err)
		}
		return "", c.mapWhatsmeowErr(traceID, phoneNumber, fmt.Errorf("failed to send poll message: %w", err))
	}
	return resp.ID, nil
}

func (c *client) SendStickerMessage(ctx context.Context, traceID string, phoneNumber string, to string, stickerBytes []byte, mimeType string) (string, error) {
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
