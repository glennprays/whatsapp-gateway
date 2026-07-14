package whatsapp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	customLog "github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/constant"
	"github.com/glennprays/whatsapp-gateway/internal/utils"
	"github.com/glennprays/whatsapp-gateway/pkg/cipherx"
	"github.com/google/uuid"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type (
	manager struct {
		Client       Client
		EventHandler Handler
		Cipher       *cipherx.Cipher
		Logger       *customLog.Logger
		Queue        domainQueue.MessageQueue
	}
	Manager interface {
		RegisterClient(ctx context.Context, traceID string, phoneNumber string)
		LoginQRCode(ctx context.Context, traceID string, phoneNumber string) (string, int, error)
		LoginPairCode(ctx context.Context, traceID string, phoneNumber string) (string, int, error)
		LoginStatus(ctx context.Context, traceID string, phoneNumber string) (bool, error)
		Logout(ctx context.Context, traceID string, phoneNumber string) error
		Reconnect(ctx context.Context, traceID string, phoneNumber string) error
		GetWebhookURL(ctx context.Context, traceID string, phoneNumber string) (*string, error)
		SetWebhookURL(ctx context.Context, traceID string, phoneNumber string, webhook *waDomain.Webhook) error
		DeleteWebhookURL(ctx context.Context, traceID string, phoneNumber string) error
		SendTextMessage(ctx context.Context, traceID string, phoneNumber string, to string, message string, msgCtx *waDomain.MessageContext) (string, error)
		SendImageMessage(ctx context.Context, traceID string, phoneNumber string, to string, imageBytes []byte, mimeType string, caption string, isViewOnce bool, msgCtx *waDomain.MessageContext) (string, error)
		SendAudioMessage(ctx context.Context, traceID string, phoneNumber string, to string, audioBytes []byte, mimeType string, isPTT bool, isViewOnce bool, msgCtx *waDomain.MessageContext) (string, error)
		SendVideoMessage(ctx context.Context, traceID string, phoneNumber string, to string, videoBytes []byte, mimeType string, caption string, isGif bool, isViewOnce bool, msgCtx *waDomain.MessageContext) (string, error)
		SendDocumentMessage(ctx context.Context, traceID string, phoneNumber string, to string, docBytes []byte, mimeType string, fileName string, caption string, msgCtx *waDomain.MessageContext) (string, error)
		SendLocationMessage(ctx context.Context, traceID string, phoneNumber string, to string, latitude float64, longitude float64, name string, address string, msgCtx *waDomain.MessageContext) (string, error)
		SendPollMessage(ctx context.Context, traceID string, phoneNumber string, to string, question string, options []string, selectableCount int, msgCtx *waDomain.MessageContext) (string, error)
		SendStickerMessage(ctx context.Context, traceID string, phoneNumber string, to string, stickerBytes []byte, mimeType string) (string, error)
		ReactToMessage(ctx context.Context, traceID string, phoneNumber string, chatJID string, senderJID string, messageID string, emoji string) error
		DeleteMessage(ctx context.Context, traceID string, phoneNumber string, chatJID string, messageID string) error
		EditMessage(ctx context.Context, traceID string, phoneNumber string, chatJID string, messageID string, newText string) error
		GetIncomingMessages(ctx context.Context, traceID string, phoneNumber string, limit int) ([]*IncomingMessage, error)
		GetJIDFromPhoneNumber(phoneNumber string) (string, error)
		CheckNumber(ctx context.Context, traceID string, phoneNumber string, msisdn string) (waDomain.ContactCheckResponse, error)
		ListContacts(ctx context.Context, traceID string, phoneNumber string) ([]waDomain.ContactListItem, error)
		ListGroups(ctx context.Context, traceID string, phoneNumber string) ([]waDomain.GroupListItem, error)
		GetGroupInfo(ctx context.Context, traceID string, phoneNumber string, groupJID string) (*waDomain.GroupInfoResponse, error)
		GetContactInfo(ctx context.Context, traceID string, phoneNumber string, userJID string) (*waDomain.ContactInfoResponse, error)
		GetAvatar(ctx context.Context, traceID string, phoneNumber string, targetJID string, preview bool, existingID string) (*waDomain.AvatarResponse, error)
		MarkRead(ctx context.Context, traceID string, phoneNumber string, chat string, sender string, messageIDs []string) error
		SendChatPresence(ctx context.Context, traceID string, phoneNumber string, chat string, state string, media string) error
		GetClientStore() *ClientStore
	}
)

var clients = NewClientStore()

func NewManager(config *config.Config, dbType string, db *sql.DB, cp *cipherx.Cipher, logger *customLog.Logger, queue domainQueue.MessageQueue, mediaDownloader MediaDownloader) (Manager, error) {
	ctx := context.Background()
	startupTraceID := uuid.New().String()

	dbLog := waLog.Stdout("Database", config.WhatsmeowLogLevel, true)
	container := sqlstore.NewWithDB(db, dbType, dbLog)
	if err := container.Upgrade(ctx); err != nil {
		logger.Error(startupTraceID, "Failed to upgrade database schema", nil, customLog.Error(err))
		return nil, fmt.Errorf("database schema upgrade: %w", err)
	}
	repository := NewWhatsappRepository(db)

	// Create webhook sender
	webhookSender := NewWebhookSender(cp, logger)

	// Create event handler with repository, sender, queue, and media downloader
	evtHandler := NewHandler(repository, webhookSender, queue, logger, mediaDownloader, config.IncomingMessageBufferSize)

	client := NewClient(container, config, repository, logger)

	err := runMigrations(db)
	if err != nil {
		logger.Error(startupTraceID, "Failed to run database migrations", nil, customLog.Error(err))
		return nil, fmt.Errorf("database migrations: %w", err)
	}

	devices, err := container.GetAllDevices(ctx)
	if err != nil {
		logger.Error(startupTraceID, "Failed to get devices from database", nil, customLog.Error(err))
		return nil, fmt.Errorf("get all devices: %w", err)
	}

	for _, device := range devices {
		phoneNumber := WhatsappDecomposeJID(device.ID.User)

		maskedPhoneNumber := MaskedPhoneNumber(phoneNumber)

		logger.Info(startupTraceID, "Restoring WhatsApp Client for "+maskedPhoneNumber, nil)
		client.InitClient(startupTraceID, phoneNumber, device, evtHandler.HandleEvent)

		if err := client.Reconnect(startupTraceID, phoneNumber); err != nil {
			logger.Error(startupTraceID, "Failed to reconnect WhatsApp client",
				nil,
				customLog.String("phone_number", maskedPhoneNumber),
				customLog.String("device_id", device.ID.String()),
				customLog.Error(err),
			)
		}
	}

	return &manager{
		Client:       client,
		EventHandler: evtHandler,
		Cipher:       cp,
		Logger:       logger,
		Queue:        queue,
	}, nil
}

func (m *manager) LoginQRCode(ctx context.Context, traceID string, phoneNumber string) (string, int, error) {
	qr, timeout, err := m.Client.LoginQRCode(ctx, traceID, phoneNumber)
	if err != nil {
		m.Logger.Error(traceID, "Failed to generate QR code for "+MaskedPhoneNumber(phoneNumber), nil, customLog.Error(err))
		return "", 0, err
	}

	m.Logger.Info(traceID, "Generated QR code for "+MaskedPhoneNumber(phoneNumber), nil)
	return qr, timeout, nil
}

func (m *manager) RegisterClient(ctx context.Context, traceID string, phoneNumber string) {
	if clients.Get(phoneNumber) == nil {
		m.Logger.Info(traceID, "Registering WhatsApp client for "+MaskedPhoneNumber(phoneNumber), nil)
		m.Client.InitClient(traceID, phoneNumber, nil, m.EventHandler.HandleEvent)
	} else {
		m.Logger.Info(traceID, "WhatsApp client for "+MaskedPhoneNumber(phoneNumber)+" already exists, skipping registration", nil)
	}
}

func (m *manager) LoginStatus(ctx context.Context, traceID string, phoneNumber string) (bool, error) {
	return m.Client.LoginStatus(traceID, phoneNumber)
}

func (m *manager) Logout(ctx context.Context, traceID string, phoneNumber string) error {
	return m.Client.Logout(ctx, traceID, phoneNumber)
}

func (m *manager) Reconnect(ctx context.Context, traceID string, phoneNumber string) error {
	return m.Client.Reconnect(traceID, phoneNumber)
}

func (m *manager) LoginPairCode(ctx context.Context, traceID string, phoneNumber string) (string, int, error) {
	return m.Client.LoginPairCode(ctx, traceID, phoneNumber)
}

func (m *manager) GetWebhookURL(ctx context.Context, traceID string, phoneNumber string) (*string, error) {
	return m.Client.GetWebhookURL(ctx, traceID, phoneNumber)
}

func (m *manager) SetWebhookURL(ctx context.Context, traceID string, phoneNumber string, webhook *waDomain.Webhook) error {
	masked := MaskedPhoneNumber(phoneNumber)
	loginStatus, err := m.Client.LoginStatus(traceID, phoneNumber)
	if err != nil {
		m.Logger.Error(traceID, "Failed to get login status", nil,
			customLog.String("phone_number", masked),
			customLog.Error(err),
		)
		return errDomain.NewError(errDomain.ErrInternalFailure, err)
	}

	if !loginStatus {
		m.Logger.Error(traceID, "Cannot set webhook URL: client not logged in", nil,
			customLog.String("phone_number", masked),
		)
		return errDomain.NewError(errDomain.ErrConflict, errDomain.NewError(errDomain.ErrUnauthorized, errors.New(constant.ErrClientNotLoggedIn)))
	}

	err = utils.ValidateURL(webhook.Url)
	if err != nil {
		m.Logger.Error(traceID, "Invalid webhook URL", nil,
			customLog.String("phone_number", masked),
			customLog.Error(err),
		)
		return errDomain.NewError(errDomain.ErrBadRequest, err)
	}

	encryptedHmacSecret, err := m.Cipher.Encrypt(webhook.HmacSecret)
	if err != nil {
		m.Logger.Error(traceID, "Failed to encrypt HMAC secret", nil,
			customLog.String("phone_number", masked),
			customLog.Error(err),
		)
		return err
	}
	webhook.HmacSecret = encryptedHmacSecret
	return m.Client.SetWebhookURL(ctx, traceID, phoneNumber, webhook)
}

func (m *manager) DeleteWebhookURL(ctx context.Context, traceID string, phoneNumber string) error {
	masked := MaskedPhoneNumber(phoneNumber)
	loginStatus, err := m.Client.LoginStatus(traceID, phoneNumber)
	if err != nil {
		m.Logger.Error(traceID, "Failed to get login status", nil,
			customLog.String("phone_number", masked),
			customLog.Error(err),
		)
		return errDomain.NewError(errDomain.ErrInternalFailure, err)
	}

	if !loginStatus {
		m.Logger.Error(traceID, "Cannot delete webhook URL: client not logged in", nil,
			customLog.String("phone_number", masked),
		)
		return errDomain.NewError(errDomain.ErrConflict, errDomain.NewError(errDomain.ErrUnauthorized, errors.New(constant.ErrClientNotLoggedIn)))
	}

	return m.Client.DeleteWebhookURL(ctx, traceID, phoneNumber)
}

func (m *manager) SendTextMessage(ctx context.Context, traceID string, phoneNumber string, to string, message string, msgCtx *waDomain.MessageContext) (string, error) {
	masked := MaskedPhoneNumber(phoneNumber)
	m.Logger.Info(traceID, "Sending text message", nil,
		customLog.String("phone_number", masked),
		customLog.String("to", to),
	)

	messageID, err := m.Client.SendTextMessage(ctx, traceID, phoneNumber, to, message, msgCtx)
	if err != nil {
		m.Logger.Error(traceID, "Failed to send text message", nil,
			customLog.String("phone_number", masked),
			customLog.String("to", to),
			customLog.Error(err),
		)
		return "", err
	}

	m.Logger.Info(traceID, "Successfully sent text message", nil,
		customLog.String("phone_number", masked),
		customLog.String("to", to),
		customLog.String("message_id", messageID),
	)

	return messageID, nil
}

func (m *manager) SendImageMessage(ctx context.Context, traceID string, phoneNumber string, to string, imageBytes []byte, mimeType string, caption string, isViewOnce bool, msgCtx *waDomain.MessageContext) (string, error) {
	masked := MaskedPhoneNumber(phoneNumber)
	m.Logger.Info(traceID, "Sending image message", nil,
		customLog.String("phone_number", masked),
		customLog.String("to", to),
	)

	messageID, err := m.Client.SendImageMessage(ctx, traceID, phoneNumber, to, imageBytes, mimeType, caption, isViewOnce, msgCtx)
	if err != nil {
		m.Logger.Error(traceID, "Failed to send image message", nil,
			customLog.String("phone_number", masked),
			customLog.String("to", to),
			customLog.Error(err),
		)
		return "", err
	}

	m.Logger.Info(traceID, "Successfully sent image message", nil,
		customLog.String("phone_number", masked),
		customLog.String("to", to),
		customLog.String("message_id", messageID),
	)

	return messageID, nil
}

func (m *manager) SendAudioMessage(ctx context.Context, traceID string, phoneNumber string, to string, audioBytes []byte, mimeType string, isPTT bool, isViewOnce bool, msgCtx *waDomain.MessageContext) (string, error) {
	masked := MaskedPhoneNumber(phoneNumber)
	m.Logger.Info(traceID, "Sending audio message", nil,
		customLog.String("phone_number", masked),
		customLog.String("to", to),
		customLog.Bool("ptt", isPTT),
	)

	messageID, err := m.Client.SendAudioMessage(ctx, traceID, phoneNumber, to, audioBytes, mimeType, isPTT, isViewOnce, msgCtx)
	if err != nil {
		m.Logger.Error(traceID, "Failed to send audio message", nil,
			customLog.String("phone_number", masked),
			customLog.String("to", to),
			customLog.Error(err),
		)
		return "", err
	}

	m.Logger.Info(traceID, "Successfully sent audio message", nil,
		customLog.String("phone_number", masked),
		customLog.String("to", to),
		customLog.String("message_id", messageID),
	)

	return messageID, nil
}

func (m *manager) SendVideoMessage(ctx context.Context, traceID string, phoneNumber string, to string, videoBytes []byte, mimeType string, caption string, isGif bool, isViewOnce bool, msgCtx *waDomain.MessageContext) (string, error) {
	masked := MaskedPhoneNumber(phoneNumber)
	m.Logger.Info(traceID, "Sending video message", nil,
		customLog.String("phone_number", masked),
		customLog.String("to", to),
		customLog.Bool("gif", isGif),
	)

	messageID, err := m.Client.SendVideoMessage(ctx, traceID, phoneNumber, to, videoBytes, mimeType, caption, isGif, isViewOnce, msgCtx)
	if err != nil {
		m.Logger.Error(traceID, "Failed to send video message", nil,
			customLog.String("phone_number", masked),
			customLog.String("to", to),
			customLog.Error(err),
		)
		return "", err
	}

	m.Logger.Info(traceID, "Successfully sent video message", nil,
		customLog.String("phone_number", masked),
		customLog.String("to", to),
		customLog.String("message_id", messageID),
	)

	return messageID, nil
}

func (m *manager) SendDocumentMessage(ctx context.Context, traceID string, phoneNumber string, to string, docBytes []byte, mimeType string, fileName string, caption string, msgCtx *waDomain.MessageContext) (string, error) {
	masked := MaskedPhoneNumber(phoneNumber)
	m.Logger.Info(traceID, "Sending document message", nil,
		customLog.String("phone_number", masked),
		customLog.String("to", to),
		customLog.String("file_name", fileName),
	)

	messageID, err := m.Client.SendDocumentMessage(ctx, traceID, phoneNumber, to, docBytes, mimeType, fileName, caption, msgCtx)
	if err != nil {
		m.Logger.Error(traceID, "Failed to send document message", nil,
			customLog.String("phone_number", masked),
			customLog.String("to", to),
			customLog.Error(err),
		)
		return "", err
	}

	m.Logger.Info(traceID, "Successfully sent document message", nil,
		customLog.String("phone_number", masked),
		customLog.String("to", to),
		customLog.String("message_id", messageID),
	)

	return messageID, nil
}

func (m *manager) SendLocationMessage(ctx context.Context, traceID string, phoneNumber string, to string, latitude float64, longitude float64, name string, address string, msgCtx *waDomain.MessageContext) (string, error) {
	masked := MaskedPhoneNumber(phoneNumber)
	m.Logger.Info(traceID, "Sending location message", nil,
		customLog.String("phone_number", masked),
		customLog.String("to", to),
	)

	messageID, err := m.Client.SendLocationMessage(ctx, traceID, phoneNumber, to, latitude, longitude, name, address, msgCtx)
	if err != nil {
		m.Logger.Error(traceID, "Failed to send location message", nil,
			customLog.String("phone_number", masked),
			customLog.String("to", to),
			customLog.Error(err),
		)
		return "", err
	}

	m.Logger.Info(traceID, "Successfully sent location message", nil,
		customLog.String("phone_number", masked),
		customLog.String("to", to),
		customLog.String("message_id", messageID),
	)
	return messageID, nil
}

func (m *manager) SendPollMessage(ctx context.Context, traceID string, phoneNumber string, to string, question string, options []string, selectableCount int, msgCtx *waDomain.MessageContext) (string, error) {
	masked := MaskedPhoneNumber(phoneNumber)
	m.Logger.Info(traceID, "Sending poll message", nil,
		customLog.String("phone_number", masked),
		customLog.String("to", to),
	)

	messageID, err := m.Client.SendPollMessage(ctx, traceID, phoneNumber, to, question, options, selectableCount, msgCtx)
	if err != nil {
		m.Logger.Error(traceID, "Failed to send poll message", nil,
			customLog.String("phone_number", masked),
			customLog.String("to", to),
			customLog.Error(err),
		)
		return "", err
	}

	m.Logger.Info(traceID, "Successfully sent poll message", nil,
		customLog.String("phone_number", masked),
		customLog.String("to", to),
		customLog.String("message_id", messageID),
	)
	return messageID, nil
}

func (m *manager) SendStickerMessage(ctx context.Context, traceID string, phoneNumber string, to string, stickerBytes []byte, mimeType string) (string, error) {
	masked := MaskedPhoneNumber(phoneNumber)
	m.Logger.Info(traceID, "Sending sticker message", nil,
		customLog.String("phone_number", masked),
		customLog.String("to", to),
	)

	messageID, err := m.Client.SendStickerMessage(ctx, traceID, phoneNumber, to, stickerBytes, mimeType)
	if err != nil {
		m.Logger.Error(traceID, "Failed to send sticker message", nil,
			customLog.String("phone_number", masked),
			customLog.String("to", to),
			customLog.Error(err),
		)
		return "", err
	}

	m.Logger.Info(traceID, "Successfully sent sticker message", nil,
		customLog.String("phone_number", masked),
		customLog.String("to", to),
		customLog.String("message_id", messageID),
	)
	return messageID, nil
}

func (m *manager) ReactToMessage(ctx context.Context, traceID string, phoneNumber string, chatJID string, senderJID string, messageID string, emoji string) error {
	masked := MaskedPhoneNumber(phoneNumber)
	m.Logger.Info(traceID, "Reacting to message", nil,
		customLog.String("phone_number", masked),
		customLog.String("chat_jid", chatJID),
		customLog.String("message_id", messageID),
	)

	err := m.Client.ReactToMessage(ctx, traceID, phoneNumber, chatJID, senderJID, messageID, emoji)
	if err != nil {
		m.Logger.Error(traceID, "Failed to react to message", nil,
			customLog.String("phone_number", masked),
			customLog.String("chat_jid", chatJID),
			customLog.String("message_id", messageID),
			customLog.Error(err),
		)
		return err
	}

	m.Logger.Info(traceID, "Successfully reacted to message", nil,
		customLog.String("phone_number", masked),
		customLog.String("chat_jid", chatJID),
		customLog.String("message_id", messageID),
	)

	return nil
}

func (m *manager) DeleteMessage(ctx context.Context, traceID string, phoneNumber string, chatJID string, messageID string) error {
	masked := MaskedPhoneNumber(phoneNumber)
	m.Logger.Info(traceID, "Deleting message", nil,
		customLog.String("phone_number", masked),
		customLog.String("chat_jid", chatJID),
		customLog.String("message_id", messageID),
	)

	err := m.Client.DeleteMessage(ctx, traceID, phoneNumber, chatJID, messageID)
	if err != nil {
		m.Logger.Error(traceID, "Failed to delete message", nil,
			customLog.String("phone_number", masked),
			customLog.String("chat_jid", chatJID),
			customLog.String("message_id", messageID),
			customLog.Error(err),
		)
		return err
	}

	m.Logger.Info(traceID, "Successfully deleted message", nil,
		customLog.String("phone_number", masked),
		customLog.String("chat_jid", chatJID),
		customLog.String("message_id", messageID),
	)

	return nil
}

func (m *manager) EditMessage(ctx context.Context, traceID string, phoneNumber string, chatJID string, messageID string, newText string) error {
	masked := MaskedPhoneNumber(phoneNumber)
	m.Logger.Info(traceID, "Editing message", nil,
		customLog.String("phone_number", masked),
		customLog.String("chat_jid", chatJID),
		customLog.String("message_id", messageID),
	)

	err := m.Client.EditMessage(ctx, traceID, phoneNumber, chatJID, messageID, newText)
	if err != nil {
		m.Logger.Error(traceID, "Failed to edit message", nil,
			customLog.String("phone_number", masked),
			customLog.String("chat_jid", chatJID),
			customLog.String("message_id", messageID),
			customLog.Error(err),
		)
		return err
	}

	m.Logger.Info(traceID, "Successfully edited message", nil,
		customLog.String("phone_number", masked),
		customLog.String("chat_jid", chatJID),
		customLog.String("message_id", messageID),
	)

	return nil
}

func (m *manager) GetJIDFromPhoneNumber(phoneNumber string) (string, error) {
	client := clients.Get(phoneNumber)
	if client == nil {
		return "", errDomain.NewError(errDomain.ErrNotFound, errors.New(constant.ErrClientNotFound))
	}

	return client.Store.ID.String(), nil
}

func (m *manager) CheckNumber(ctx context.Context, traceID string, phoneNumber string, msisdn string) (waDomain.ContactCheckResponse, error) {
	masked := MaskedPhoneNumber(phoneNumber)
	m.Logger.Info(traceID, "Checking if number is on WhatsApp", nil,
		customLog.String("phone_number", masked),
	)
	resp, err := m.Client.CheckNumber(ctx, traceID, phoneNumber, msisdn)
	if err != nil {
		m.Logger.Error(traceID, "Failed to check number", nil,
			customLog.String("phone_number", masked),
			customLog.Error(err),
		)
		return waDomain.ContactCheckResponse{}, err
	}
	return resp, nil
}

func (m *manager) ListContacts(ctx context.Context, traceID string, phoneNumber string) ([]waDomain.ContactListItem, error) {
	items, err := m.Client.ListContacts(ctx, traceID, phoneNumber)
	if err != nil {
		m.Logger.Error(traceID, "Failed to list contacts", nil,
			customLog.String("phone_number", MaskedPhoneNumber(phoneNumber)),
			customLog.Error(err),
		)
		return nil, err
	}
	return items, nil
}

func (m *manager) ListGroups(ctx context.Context, traceID string, phoneNumber string) ([]waDomain.GroupListItem, error) {
	items, err := m.Client.ListGroups(ctx, traceID, phoneNumber)
	if err != nil {
		m.Logger.Error(traceID, "Failed to list groups", nil,
			customLog.String("phone_number", MaskedPhoneNumber(phoneNumber)),
			customLog.Error(err),
		)
		return nil, err
	}
	return items, nil
}

func (m *manager) GetGroupInfo(ctx context.Context, traceID string, phoneNumber string, groupJID string) (*waDomain.GroupInfoResponse, error) {
	info, err := m.Client.GetGroupInfo(ctx, traceID, phoneNumber, groupJID)
	if err != nil {
		m.Logger.Error(traceID, "Failed to get group info", nil,
			customLog.String("phone_number", MaskedPhoneNumber(phoneNumber)),
			customLog.Error(err),
		)
		return nil, err
	}
	return info, nil
}

func (m *manager) GetContactInfo(ctx context.Context, traceID string, phoneNumber string, userJID string) (*waDomain.ContactInfoResponse, error) {
	info, err := m.Client.GetContactInfo(ctx, traceID, phoneNumber, userJID)
	if err != nil {
		m.Logger.Error(traceID, "Failed to get contact info", nil,
			customLog.String("phone_number", MaskedPhoneNumber(phoneNumber)),
			customLog.Error(err),
		)
		return nil, err
	}
	return info, nil
}

func (m *manager) GetAvatar(ctx context.Context, traceID string, phoneNumber string, targetJID string, preview bool, existingID string) (*waDomain.AvatarResponse, error) {
	info, err := m.Client.GetAvatar(ctx, traceID, phoneNumber, targetJID, preview, existingID)
	if err != nil {
		m.Logger.Error(traceID, "Failed to get avatar", nil,
			customLog.String("phone_number", MaskedPhoneNumber(phoneNumber)),
			customLog.Error(err),
		)
		return nil, err
	}
	return info, nil
}

func (m *manager) MarkRead(ctx context.Context, traceID string, phoneNumber string, chat string, sender string, messageIDs []string) error {
	if err := m.Client.MarkRead(ctx, traceID, phoneNumber, chat, sender, messageIDs); err != nil {
		m.Logger.Error(traceID, "Failed to mark messages read", nil,
			customLog.String("phone_number", MaskedPhoneNumber(phoneNumber)),
			customLog.Error(err),
		)
		return err
	}
	return nil
}

func (m *manager) SendChatPresence(ctx context.Context, traceID string, phoneNumber string, chat string, state string, media string) error {
	if err := m.Client.SendChatPresence(ctx, traceID, phoneNumber, chat, state, media); err != nil {
		m.Logger.Error(traceID, "Failed to send chat presence", nil,
			customLog.String("phone_number", MaskedPhoneNumber(phoneNumber)),
			customLog.Error(err),
		)
		return err
	}
	return nil
}

func (m *manager) GetClientStore() *ClientStore {
	return clients
}

func (m *manager) GetIncomingMessages(ctx context.Context, traceID string, phoneNumber string, limit int) ([]*IncomingMessage, error) {
	masked := MaskedPhoneNumber(phoneNumber)
	loginStatus, err := m.Client.LoginStatus(traceID, phoneNumber)
	if err != nil {
		m.Logger.Error(traceID, "Failed to get login status", nil,
			customLog.String("phone_number", masked),
			customLog.Error(err),
		)
		return nil, errDomain.NewError(errDomain.ErrInternalFailure, err)
	}

	if !loginStatus {
		m.Logger.Error(traceID, "Cannot read incoming messages: client not logged in", nil,
			customLog.String("phone_number", masked),
		)
		return nil, errDomain.NewError(errDomain.ErrConflict, errDomain.NewError(errDomain.ErrUnauthorized, errors.New(constant.ErrClientNotLoggedIn)))
	}

	return m.EventHandler.GetIncomingMessages(phoneNumber, limit), nil
}
