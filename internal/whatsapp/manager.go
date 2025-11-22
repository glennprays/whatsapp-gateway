package whatsapp

import (
	"context"
	"database/sql"
	"errors"

	"github.com/glennprays/whatsapp-gateway/config"
	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
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
	}
)

var Clients map[string]*whatsmeow.Client

func init() {
	Clients = make(map[string]*whatsmeow.Client)
}

func NewManager(config *config.Config, dbType string, db *sql.DB, cp *cipherx.Cipher) Manager {
	ctx := context.Background()

	evtHandler := NewHandler()

	dbLog := waLog.Stdout("Database", config.WhatsmeowLogLevel, true)
	container := sqlstore.NewWithDB(db, dbType, dbLog)
	if err := container.Upgrade(ctx); err != nil {
		log.Fatalf("Failed to upgrade database schema: %v", err)
	}
	repository := NewWhatsappRepository(db)

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
