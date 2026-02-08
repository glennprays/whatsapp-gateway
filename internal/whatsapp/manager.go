package whatsapp

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	customLog "github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/constant"
	"github.com/glennprays/whatsapp-gateway/internal/utils"
	"github.com/glennprays/whatsapp-gateway/pkg/cipherx"
	"go.mau.fi/whatsmeow"
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
		SendTextMessage(ctx context.Context, traceID string, phoneNumber string, to string, message string) (string, error)
		SendImageMessage(ctx context.Context, traceID string, phoneNumber string, to string, imageBytes []byte, mimeType string, caption string, isViewOnce bool) (string, error)
		ReactToMessage(ctx context.Context, traceID string, phoneNumber string, chatJID string, messageID string, emoji string) error
		DeleteMessage(ctx context.Context, traceID string, phoneNumber string, chatJID string, messageID string) error
		EditMessage(ctx context.Context, traceID string, phoneNumber string, chatJID string, messageID string, newText string) error
	}
)

var Clients map[string]*whatsmeow.Client

func init() {
	Clients = make(map[string]*whatsmeow.Client)
}

func NewManager(config *config.Config, dbType string, db *sql.DB, cp *cipherx.Cipher, logger *customLog.Logger, queue domainQueue.MessageQueue) Manager {
	ctx := context.Background()
	startupTraceID := uuid.New().String()

	dbLog := waLog.Stdout("Database", config.WhatsmeowLogLevel, true)
	container := sqlstore.NewWithDB(db, dbType, dbLog)
	if err := container.Upgrade(ctx); err != nil {
		logger.Error(startupTraceID, "Failed to upgrade database schema", nil, customLog.Error(err))
		panic(err)
	}
	repository := NewWhatsappRepository(db)

	// Create webhook sender
	webhookSender := NewWebhookSender(cp)

	// Create event handler with repository, sender, and queue
	evtHandler := NewHandler(repository, webhookSender, queue, logger)

	client := NewClient(container, config, repository, logger)

	err := runMigrations(db)
	if err != nil {
		logger.Error(startupTraceID, "Failed to run database migrations", nil, customLog.Error(err))
		panic(err)
	}

	devices, err := container.GetAllDevices(ctx)
	if err != nil {
		logger.Error(startupTraceID, "Failed to get devices from database", nil, customLog.Error(err))
		panic(err)
	}

	for _, device := range devices {
		phoneNumber := WhatsappDecomposeJID(device.ID.User)

		maskedPhoneNumber := MaskedPhoneNumber(phoneNumber)

		logger.Info(startupTraceID, "Restoring WhatsApp Client for "+maskedPhoneNumber, nil)
		client.InitClient(startupTraceID, phoneNumber, device, evtHandler.HandleEvent)

		if err := client.Reconnect(startupTraceID, phoneNumber); err != nil {
			logger.Error(startupTraceID, "Failed to reconnect WhatsApp client for "+maskedPhoneNumber, nil, customLog.Error(err))
		}
	}

	return &manager{
		Client:       client,
		EventHandler: evtHandler,
		Cipher:       cp,
		Logger:       logger,
		Queue:        queue,
	}
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
	if Clients[phoneNumber] == nil {
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
	loginStatus, err := m.Client.LoginStatus(traceID, phoneNumber)
	if err != nil {
		m.Logger.Error(traceID, "Failed to get login status for "+MaskedPhoneNumber(phoneNumber), nil, customLog.Error(err))
		return errDomain.NewError(errDomain.ErrInternalFailure, err)
	}

	if !loginStatus {
		m.Logger.Error(traceID, "Cannot set webhook URL for "+MaskedPhoneNumber(phoneNumber)+": client not logged in", nil)
		return errDomain.NewError(errDomain.ErrConflict, errDomain.NewError(errDomain.ErrUnauthorized, errors.New(constant.ErrClientNotLoggedIn)))
	}

	err = utils.ValidateURL(webhook.Url)
	if err != nil {
		m.Logger.Error(traceID, "Invalid webhook URL for "+MaskedPhoneNumber(phoneNumber), nil, customLog.Error(err))
		return errDomain.NewError(errDomain.ErrBadRequest, err)
	}

	encryptedHmacSecret, err := m.Cipher.Encrypt(webhook.HmacSecret)
	if err != nil {
		m.Logger.Error(traceID, "Failed to encrypt HMAC secret for "+MaskedPhoneNumber(phoneNumber), nil, customLog.Error(err))
		return err
	}
	webhook.HmacSecret = encryptedHmacSecret
	return m.Client.SetWebhookURL(ctx, traceID, phoneNumber, webhook)
}

func (m *manager) DeleteWebhookURL(ctx context.Context, traceID string, phoneNumber string) error {
	loginStatus, err := m.Client.LoginStatus(traceID, phoneNumber)
	if err != nil {
		m.Logger.Error(traceID, "Failed to get login status for "+MaskedPhoneNumber(phoneNumber), nil, customLog.Error(err))
		return errDomain.NewError(errDomain.ErrInternalFailure, err)
	}

	if !loginStatus {
		m.Logger.Error(traceID, "Cannot delete webhook URL for "+MaskedPhoneNumber(phoneNumber)+": client not logged in", nil)
		return errDomain.NewError(errDomain.ErrConflict, errDomain.NewError(errDomain.ErrUnauthorized, errors.New(constant.ErrClientNotLoggedIn)))
	}

	return m.Client.DeleteWebhookURL(ctx, traceID, phoneNumber)
}

func (m *manager) SendTextMessage(ctx context.Context, traceID string, phoneNumber string, to string, message string) (string, error) {
	m.Logger.Info(
		traceID,
		"Sending text message",
		[]customLog.Field{
			customLog.String("phone_number", MaskedPhoneNumber(phoneNumber)),
			customLog.String("to", to),
		},
	)

	messageID, err := m.Client.SendTextMessage(ctx, traceID, phoneNumber, to, message)
	if err != nil {
		m.Logger.Error(traceID, "Failed to send text message for "+MaskedPhoneNumber(phoneNumber), nil, customLog.Error(err))
		return "", err
	}

	m.Logger.Info(
		traceID,
		"Successfully sent text message",
		[]customLog.Field{
			customLog.String("phone_number", MaskedPhoneNumber(phoneNumber)),
			customLog.String("message_id", messageID),
		},
	)

	return messageID, nil
}

func (m *manager) SendImageMessage(ctx context.Context, traceID string, phoneNumber string, to string, imageBytes []byte, mimeType string, caption string, isViewOnce bool) (string, error) {
	m.Logger.Info(
		traceID,
		"Sending image message",
		[]customLog.Field{
			customLog.String("phone_number", MaskedPhoneNumber(phoneNumber)),
			customLog.String("to", to),
		},
	)

	messageID, err := m.Client.SendImageMessage(ctx, traceID, phoneNumber, to, imageBytes, mimeType, caption, isViewOnce)
	if err != nil {
		m.Logger.Error(traceID, "Failed to send image message for "+MaskedPhoneNumber(phoneNumber), nil, customLog.Error(err))
		return "", err
	}

	m.Logger.Info(
		traceID,
		"Successfully sent image message",
		[]customLog.Field{
			customLog.String("phone_number", MaskedPhoneNumber(phoneNumber)),
			customLog.String("message_id", messageID),
		},
	)

	return messageID, nil
}

func (m *manager) ReactToMessage(ctx context.Context, traceID string, phoneNumber string, chatJID string, messageID string, emoji string) error {
	m.Logger.Info(
		traceID,
		"Reacting to message",
		[]customLog.Field{
			customLog.String("phone_number", MaskedPhoneNumber(phoneNumber)),
			customLog.String("message_id", messageID),
		},
	)

	err := m.Client.ReactToMessage(ctx, traceID, phoneNumber, chatJID, messageID, emoji)
	if err != nil {
		m.Logger.Error(traceID, "Failed to react to message for "+MaskedPhoneNumber(phoneNumber), nil, customLog.Error(err))
		return err
	}

	m.Logger.Info(
		traceID,
		"Successfully reacted to message",
		[]customLog.Field{
			customLog.String("phone_number", MaskedPhoneNumber(phoneNumber)),
			customLog.String("message_id", messageID),
		},
	)

	return nil
}

func (m *manager) DeleteMessage(ctx context.Context, traceID string, phoneNumber string, chatJID string, messageID string) error {
	m.Logger.Info(
		traceID,
		"Deleting message",
		[]customLog.Field{
			customLog.String("phone_number", MaskedPhoneNumber(phoneNumber)),
			customLog.String("message_id", messageID),
		},
	)

	err := m.Client.DeleteMessage(ctx, traceID, phoneNumber, chatJID, messageID)
	if err != nil {
		m.Logger.Error(traceID, "Failed to delete message for "+MaskedPhoneNumber(phoneNumber), nil, customLog.Error(err))
		return err
	}

	m.Logger.Info(
		traceID,
		"Successfully deleted message",
		[]customLog.Field{
			customLog.String("phone_number", MaskedPhoneNumber(phoneNumber)),
			customLog.String("message_id", messageID),
		},
	)

	return nil
}

func (m *manager) EditMessage(ctx context.Context, traceID string, phoneNumber string, chatJID string, messageID string, newText string) error {
	m.Logger.Info(
		traceID,
		"Editing message",
		[]customLog.Field{
			customLog.String("phone_number", MaskedPhoneNumber(phoneNumber)),
			customLog.String("message_id", messageID),
		},
	)

	err := m.Client.EditMessage(ctx, traceID, phoneNumber, chatJID, messageID, newText)
	if err != nil {
		m.Logger.Error(traceID, "Failed to edit message for "+MaskedPhoneNumber(phoneNumber), nil, customLog.Error(err))
		return err
	}

	m.Logger.Info(
		traceID,
		"Successfully edited message",
		[]customLog.Field{
			customLog.String("phone_number", MaskedPhoneNumber(phoneNumber)),
			customLog.String("message_id", messageID),
		},
	)

	return nil
}
