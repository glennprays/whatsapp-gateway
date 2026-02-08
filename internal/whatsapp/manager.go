package whatsapp

import (
	"context"
	"database/sql"
	"errors"

	customLog "github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/constant"
	"github.com/glennprays/whatsapp-gateway/internal/utils"
	"github.com/glennprays/whatsapp-gateway/pkg/cipherx"
	log "github.com/sirupsen/logrus"
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
		RegisterClient(ctx context.Context, phoneNumber string)
		LoginQRCode(ctx context.Context, phoneNumber string) (string, int, error)
		LoginPairCode(ctx context.Context, phoneNumber string) (string, int, error)
		LoginStatus(ctx context.Context, phoneNumber string) (bool, error)
		Logout(ctx context.Context, phoneNumber string) error
		Reconnect(ctx context.Context, phoneNumber string) error
		GetWebhookURL(ctx context.Context, phoneNumber string) (*string, error)
		SetWebhookURL(ctx context.Context, phoneNumber string, webhook *waDomain.Webhook) error
		DeleteWebhookURL(ctx context.Context, phoneNumber string) error
		SendTextMessage(ctx context.Context, phoneNumber string, to string, message string) (string, error)
		SendImageMessage(ctx context.Context, phoneNumber string, to string, imageBytes []byte, mimeType string, caption string, isViewOnce bool) (string, error)
		ReactToMessage(ctx context.Context, phoneNumber string, chatJID string, messageID string, emoji string) error
		DeleteMessage(ctx context.Context, phoneNumber string, chatJID string, messageID string) error
		EditMessage(ctx context.Context, phoneNumber string, chatJID string, messageID string, newText string) error
	}
)

var Clients map[string]*whatsmeow.Client

func init() {
	Clients = make(map[string]*whatsmeow.Client)
}

func NewManager(config *config.Config, dbType string, db *sql.DB, cp *cipherx.Cipher, logger *customLog.Logger, queue domainQueue.MessageQueue) Manager {
	ctx := context.Background()

	dbLog := waLog.Stdout("Database", config.WhatsmeowLogLevel, true)
	container := sqlstore.NewWithDB(db, dbType, dbLog)
	if err := container.Upgrade(ctx); err != nil {
		log.Fatalf("Failed to upgrade database schema: %v", err)
	}
	repository := NewWhatsappRepository(db)

	// Create webhook sender
	webhookSender := NewWebhookSender(cp)

	// Create event handler with repository, sender, and queue
	evtHandler := NewHandler(repository, webhookSender, queue)

	client := NewClient(container, config, repository)

	err := runMigrations(db)
	if err != nil {
		log.Fatalf("Failed to run database migrations: %v", err)
	}

	devices, err := container.GetAllDevices(ctx)
	if err != nil {
		log.Fatalf("Failed to get devices from database: %v", err)
	}

	for _, device := range devices {
		phoneNumber := WhatsappDecomposeJID(device.ID.User)

		maskedPhoneNumber := MaskedPhoneNumber(phoneNumber)

		log.Infof("Restoring WhatsApp Client for %s", maskedPhoneNumber)
		client.InitClient(phoneNumber, device, evtHandler.HandleEvent)

		if err := client.Reconnect(phoneNumber); err != nil {
			log.Errorf("Failed to reconnect WhatsApp client for %s: %v", maskedPhoneNumber, err)
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

func (m *manager) LoginQRCode(ctx context.Context, phoneNumber string) (string, int, error) {
	qr, timeout, err := m.Client.LoginQRCode(ctx, phoneNumber)
	if err != nil {
		log.Errorf("Failed to generate QR code for %s: %v", MaskedPhoneNumber(phoneNumber), err)
		return "", 0, err
	}

	log.Infof("Generated QR code for %s with timeout %d seconds", MaskedPhoneNumber(phoneNumber), timeout)
	return qr, timeout, nil
}

func (m *manager) RegisterClient(ctx context.Context, phoneNumber string) {
	if Clients[phoneNumber] == nil {
		log.Infof("Registering WhatsApp client for %s", MaskedPhoneNumber(phoneNumber))
		m.Client.InitClient(phoneNumber, nil, m.EventHandler.HandleEvent)
	} else {
		log.Warnf("WhatsApp client for %s already exists, skipping registration", MaskedPhoneNumber(phoneNumber))
	}
}

func (m *manager) LoginStatus(ctx context.Context, phoneNumber string) (bool, error) {
	return m.Client.LoginStatus(phoneNumber)
}

func (m *manager) Logout(ctx context.Context, phoneNumber string) error {
	return m.Client.Logout(ctx, phoneNumber)
}

func (m *manager) Reconnect(ctx context.Context, phoneNumber string) error {
	return m.Client.Reconnect(phoneNumber)
}

func (m *manager) LoginPairCode(ctx context.Context, phoneNumber string) (string, int, error) {
	return m.Client.LoginPairCode(ctx, phoneNumber)
}

func (m *manager) GetWebhookURL(ctx context.Context, phoneNumber string) (*string, error) {
	return m.Client.GetWebhookURL(ctx, phoneNumber)
}

func (m *manager) SetWebhookURL(ctx context.Context, phoneNumber string, webhook *waDomain.Webhook) error {
	loginStatus, err := m.Client.LoginStatus(phoneNumber)
	if err != nil {
		log.Errorf("Failed to get login status for %s: %v", MaskedPhoneNumber(phoneNumber), err)
		return errDomain.NewError(errDomain.ErrInternalFailure, err)
	}

	if !loginStatus {
		log.Errorf("Cannot set webhook URL for %s: client not logged in", MaskedPhoneNumber(phoneNumber))
		return errDomain.NewError(errDomain.ErrConflict, errDomain.NewError(errDomain.ErrUnauthorized, errors.New(constant.ErrClientNotLoggedIn)))
	}

	err = utils.ValidateURL(webhook.Url)
	if err != nil {
		log.Errorf("Invalid webhook URL for %s: %v", MaskedPhoneNumber(phoneNumber), err)
		return errDomain.NewError(errDomain.ErrBadRequest, err)
	}

	encryptedHmacSecret, err := m.Cipher.Encrypt(webhook.HmacSecret)
	if err != nil {
		log.Errorf("Failed to encrypt HMAC secret for %s: %v", MaskedPhoneNumber(phoneNumber), err)
		return err
	}
	webhook.HmacSecret = encryptedHmacSecret
	return m.Client.SetWebhookURL(ctx, phoneNumber, webhook)
}

func (m *manager) DeleteWebhookURL(ctx context.Context, phoneNumber string) error {
	loginStatus, err := m.Client.LoginStatus(phoneNumber)
	if err != nil {
		log.Errorf("Failed to get login status for %s: %v", MaskedPhoneNumber(phoneNumber), err)
		return errDomain.NewError(errDomain.ErrInternalFailure, err)
	}

	if !loginStatus {
		log.Errorf("Cannot delete webhook URL for %s: client not logged in", MaskedPhoneNumber(phoneNumber))
		return errDomain.NewError(errDomain.ErrConflict, errDomain.NewError(errDomain.ErrUnauthorized, errors.New(constant.ErrClientNotLoggedIn)))
	}

	return m.Client.DeleteWebhookURL(ctx, phoneNumber)
}

func (m *manager) SendTextMessage(ctx context.Context, phoneNumber string, to string, message string) (string, error) {
	m.Logger.Info(
		"",
		"Sending text message",
		[]customLog.Field{
			customLog.String("phone_number", MaskedPhoneNumber(phoneNumber)),
			customLog.String("to", to),
		},
	)

	messageID, err := m.Client.SendTextMessage(ctx, phoneNumber, to, message)
	if err != nil {
		log.Errorf("Failed to send text message for %s: %v", MaskedPhoneNumber(phoneNumber), err)
		return "", err
	}

	m.Logger.Info(
		"",
		"Successfully sent text message",
		[]customLog.Field{
			customLog.String("phone_number", MaskedPhoneNumber(phoneNumber)),
			customLog.String("message_id", messageID),
		},
	)

	return messageID, nil
}

func (m *manager) SendImageMessage(ctx context.Context, phoneNumber string, to string, imageBytes []byte, mimeType string, caption string, isViewOnce bool) (string, error) {
	m.Logger.Info(
		"",
		"Sending image message",
		[]customLog.Field{
			customLog.String("phone_number", MaskedPhoneNumber(phoneNumber)),
			customLog.String("to", to),
		},
	)

	messageID, err := m.Client.SendImageMessage(ctx, phoneNumber, to, imageBytes, mimeType, caption, isViewOnce)
	if err != nil {
		log.Errorf("Failed to send image message for %s: %v", MaskedPhoneNumber(phoneNumber), err)
		return "", err
	}

	m.Logger.Info(
		"",
		"Successfully sent image message",
		[]customLog.Field{
			customLog.String("phone_number", MaskedPhoneNumber(phoneNumber)),
			customLog.String("message_id", messageID),
		},
	)

	return messageID, nil
}

func (m *manager) ReactToMessage(ctx context.Context, phoneNumber string, chatJID string, messageID string, emoji string) error {
	m.Logger.Info(
		"",
		"Reacting to message",
		[]customLog.Field{
			customLog.String("phone_number", MaskedPhoneNumber(phoneNumber)),
			customLog.String("message_id", messageID),
		},
	)

	err := m.Client.ReactToMessage(ctx, phoneNumber, chatJID, messageID, emoji)
	if err != nil {
		log.Errorf("Failed to react to message for %s: %v", MaskedPhoneNumber(phoneNumber), err)
		return err
	}

	m.Logger.Info(
		"",
		"Successfully reacted to message",
		[]customLog.Field{
			customLog.String("phone_number", MaskedPhoneNumber(phoneNumber)),
			customLog.String("message_id", messageID),
		},
	)

	return nil
}

func (m *manager) DeleteMessage(ctx context.Context, phoneNumber string, chatJID string, messageID string) error {
	m.Logger.Info(
		"",
		"Deleting message",
		[]customLog.Field{
			customLog.String("phone_number", MaskedPhoneNumber(phoneNumber)),
			customLog.String("message_id", messageID),
		},
	)

	err := m.Client.DeleteMessage(ctx, phoneNumber, chatJID, messageID)
	if err != nil {
		log.Errorf("Failed to delete message for %s: %v", MaskedPhoneNumber(phoneNumber), err)
		return err
	}

	m.Logger.Info(
		"",
		"Successfully deleted message",
		[]customLog.Field{
			customLog.String("phone_number", MaskedPhoneNumber(phoneNumber)),
			customLog.String("message_id", messageID),
		},
	)

	return nil
}

func (m *manager) EditMessage(ctx context.Context, phoneNumber string, chatJID string, messageID string, newText string) error {
	m.Logger.Info(
		"",
		"Editing message",
		[]customLog.Field{
			customLog.String("phone_number", MaskedPhoneNumber(phoneNumber)),
			customLog.String("message_id", messageID),
		},
	)

	err := m.Client.EditMessage(ctx, phoneNumber, chatJID, messageID, newText)
	if err != nil {
		log.Errorf("Failed to edit message for %s: %v", MaskedPhoneNumber(phoneNumber), err)
		return err
	}

	m.Logger.Info(
		"",
		"Successfully edited message",
		[]customLog.Field{
			customLog.String("phone_number", MaskedPhoneNumber(phoneNumber)),
			customLog.String("message_id", messageID),
		},
	)

	return nil
}
