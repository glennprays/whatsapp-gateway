package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/glennprays/whatsapp-gateway/config"
	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/constant"
	customLog "github.com/glennprays/log"
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
		ReactToMessage(ctx context.Context, traceID string, phoneNumber string, chatJID string, messageID string, emoji string) error
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
	if Clients[phoneNumber] == nil {
		if device == nil {
			c.logger.Info(traceID, fmt.Sprintf("Creating new device for Phone Number: %s", MaskedPhoneNumber(phoneNumber)), nil)
			device = c.container.NewDevice()
		}
		store.DeviceProps.Os = proto.String(c.cfg.WhatsappDeviceLabel)
		store.DeviceProps.RequireFullSync = proto.Bool(false)

		client := whatsmeow.NewClient(device, waLog.Stdout("Client-login", c.cfg.WhatsmeowLogLevel, true))
		client.AddEventHandler(func(evt any) {
			eventHandler(phoneNumber, evt)
		})
		client.EnableAutoReconnect = true
		client.AutoTrustIdentity = true

		Clients[phoneNumber] = client
	}
}

func (c *client) Reconnect(traceID string, phoneNumber string) error {
	client := Clients[phoneNumber]
	if client == nil {
		return errors.New(constant.ErrClientNotFound)
	}
	client.Disconnect()
	err := client.Connect()
	if err != nil {
		return err
	}
	return nil
}

func (c *client) LoginQRCode(ctx context.Context, traceID string, phoneNumber string) (string, int, error) {
	if Clients[phoneNumber] != nil {
		client := Clients[phoneNumber]

		client.Disconnect()
		if client.Store.ID == nil {
			qrChanGenerate, _ := client.GetQRChannel(context.Background())
			err := client.Connect()
			if err != nil {
				return "", 0, err
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
	client := Clients[phoneNumber]
	if client == nil {
		return "", 0, errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}
	client.Disconnect()
	if client.Store.ID == nil {
		err := client.Connect()
		if err != nil {
			return "", 0, err
		}

		pairCode, err := client.PairPhone(ctx, phoneNumber, true, whatsmeow.PairClientChrome, fmt.Sprintf("Chrome (%s)", WhatsAppGetUserOS()))
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
	if Clients[phoneNumber] != nil {
		client := Clients[phoneNumber]
		return client.IsLoggedIn(), nil
	}
	return false, errDomain.NewError(errDomain.ErrNotFound, errors.New("client not found"))
}

func (c *client) Logout(ctx context.Context, traceID string, phoneNumber string) error {
	if Clients[phoneNumber] != nil {
		client := Clients[phoneNumber]
		err := client.Logout(ctx)
		if err != nil {
			c.logger.Error(traceID, fmt.Sprintf("Failed to logout client %s", MaskedPhoneNumber(phoneNumber)), nil, customLog.Error(err))
			client.Disconnect()
			if err := client.Store.Delete(ctx); err != nil {
				c.logger.Error(traceID, fmt.Sprintf("Failed to delete client store %s", MaskedPhoneNumber(phoneNumber)), nil, customLog.Error(err))
			}
			return nil
		}
		return nil
	}
	return errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
}

func (c *client) GetWebhookURL(ctx context.Context, traceID string, phoneNumber string) (*string, error) {
	client := Clients[phoneNumber]
	if client == nil {
		return nil, errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}

	webhookURL, err := c.repository.GetWebhook(ctx, client.Store.ID.String())
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
	client := Clients[phoneNumber]
	if client == nil {
		return errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}

	err := c.repository.SetWebhook(ctx, client.Store.ID.String(), webhook.Url, webhook.HmacSecret)
	if err != nil {
		c.logger.Error(traceID, fmt.Sprintf("Failed to set webhook URL for %s", MaskedPhoneNumber(phoneNumber)), nil, customLog.Error(err))
		return errDomain.NewError(errDomain.ErrInternalFailure, err)
	}

	return nil
}

func (c *client) DeleteWebhookURL(ctx context.Context, traceID string, phoneNumber string) error {
	client := Clients[phoneNumber]
	if client == nil {
		return errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}

	err := c.repository.DeleteWebhook(ctx, client.Store.ID.String())
	if err != nil {
		c.logger.Error(traceID, fmt.Sprintf("Failed to delete webhook URL for %s", MaskedPhoneNumber(phoneNumber)), nil, customLog.Error(err))
		return errDomain.NewError(errDomain.ErrInternalFailure, err)
	}

	return nil
}

func (c *client) SendTextMessage(ctx context.Context, traceID string, phoneNumber string, to string, message string) (string, error) {
	client := Clients[phoneNumber]
	if client == nil {
		return "", errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}

	if !client.IsLoggedIn() {
		return "", errDomain.NewError(errDomain.ErrUnauthorized, errors.New("client not logged in"))
	}

	toJID, err := types.ParseJID(to)
	if err != nil {
		return "", errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("invalid JID format: %w", err))
	}

	msg := &waE2E.Message{
		Conversation: proto.String(message),
	}

	resp, err := client.SendMessage(ctx, toJID, msg)
	if err != nil {
		return "", errDomain.NewError(errDomain.ErrInternalFailure, fmt.Errorf("failed to send message: %w", err))
	}

	return resp.ID, nil
}

func (c *client) SendImageMessage(ctx context.Context, traceID string, phoneNumber string, to string, imageBytes []byte, mimeType string, caption string, isViewOnce bool) (string, error) {
	client := Clients[phoneNumber]
	if client == nil {
		return "", errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}

	if !client.IsLoggedIn() {
		return "", errDomain.NewError(errDomain.ErrUnauthorized, errors.New("client not logged in"))
	}

	toJID, err := types.ParseJID(to)
	if err != nil {
		return "", errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("invalid JID format: %w", err))
	}

	// Upload image to WhatsApp servers
	uploaded, err := client.Upload(ctx, imageBytes, whatsmeow.MediaImage)
	if err != nil {
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

	resp, err := client.SendMessage(ctx, toJID, msg)
	if err != nil {
		return "", errDomain.NewError(errDomain.ErrInternalFailure, fmt.Errorf("failed to send image message: %w", err))
	}

	return resp.ID, nil
}

func (c *client) ReactToMessage(ctx context.Context, traceID string, phoneNumber string, chatJID string, messageID string, emoji string) error {
	client := Clients[phoneNumber]
	if client == nil {
		return errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}

	if !client.IsLoggedIn() {
		return errDomain.NewError(errDomain.ErrUnauthorized, errors.New("client not logged in"))
	}

	toJID, err := types.ParseJID(chatJID)
	if err != nil {
		return errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("invalid JID format: %w", err))
	}

	// Build reaction message
	msg := &waE2E.Message{
		ReactionMessage: &waE2E.ReactionMessage{
			Key: &waE2E.MessageKey{
				RemoteJID: proto.String(chatJID),
				FromMe:    proto.Bool(false),
				ID:        proto.String(messageID),
			},
			Text:              proto.String(emoji),
			SenderTimestampMS: proto.Int64(time.Now().UnixMilli()),
		},
	}

	_, err = client.SendMessage(ctx, toJID, msg)
	if err != nil {
		return errDomain.NewError(errDomain.ErrInternalFailure, fmt.Errorf("failed to send reaction: %w", err))
	}

	return nil
}

func (c *client) DeleteMessage(ctx context.Context, traceID string, phoneNumber string, chatJID string, messageID string) error {
	client := Clients[phoneNumber]
	if client == nil {
		return errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}

	if !client.IsLoggedIn() {
		return errDomain.NewError(errDomain.ErrUnauthorized, errors.New("client not logged in"))
	}

	toJID, err := types.ParseJID(chatJID)
	if err != nil {
		return errDomain.NewError(errDomain.ErrBadRequest, fmt.Errorf("invalid JID format: %w", err))
	}

	// Build revoke message
	_, err = client.SendMessage(ctx, toJID, client.BuildRevoke(toJID, types.EmptyJID, messageID))
	if err != nil {
		return errDomain.NewError(errDomain.ErrInternalFailure, fmt.Errorf("failed to delete message: %w", err))
	}

	return nil
}

func (c *client) EditMessage(ctx context.Context, traceID string, phoneNumber string, chatJID string, messageID string, newText string) error {
	client := Clients[phoneNumber]
	if client == nil {
		return errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}

	if !client.IsLoggedIn() {
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

	_, err = client.SendMessage(ctx, toJID, client.BuildEdit(toJID, messageID, editMsg))
	if err != nil {
		return errDomain.NewError(errDomain.ErrInternalFailure, fmt.Errorf("failed to edit message: %w", err))
	}

	return nil
}
