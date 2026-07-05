package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
		SetWebhookURL(ctx context.Context, traceID string, phoneNumber string, webhook *waDomain.Webhook) error
		DeleteWebhookURL(ctx context.Context, traceID string, phoneNumber string) error
		SendTextMessage(ctx context.Context, traceID string, phoneNumber string, to string, message string) (string, error)
		SendImageMessage(ctx context.Context, traceID string, phoneNumber string, to string, imageBytes []byte, mimeType string, caption string, isViewOnce bool) (string, error)
	SendAudioMessage(ctx context.Context, traceID string, phoneNumber string, to string, audioBytes []byte, mimeType string, isPTT bool, isViewOnce bool) (string, error)
	SendVideoMessage(ctx context.Context, traceID string, phoneNumber string, to string, videoBytes []byte, mimeType string, caption string, isGif bool, isViewOnce bool) (string, error)
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

func (c *client) SendTextMessage(ctx context.Context, traceID string, phoneNumber string, to string, message string) (string, error) {
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

	msg := &waE2E.Message{
		Conversation: proto.String(message),
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

func (c *client) SendImageMessage(ctx context.Context, traceID string, phoneNumber string, to string, imageBytes []byte, mimeType string, caption string, isViewOnce bool) (string, error) {
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

func (c *client) SendAudioMessage(ctx context.Context, traceID string, phoneNumber string, to string, audioBytes []byte, mimeType string, isPTT bool, isViewOnce bool) (string, error) {
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

func (c *client) SendVideoMessage(ctx context.Context, traceID string, phoneNumber string, to string, videoBytes []byte, mimeType string, caption string, isGif bool, isViewOnce bool) (string, error) {
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
