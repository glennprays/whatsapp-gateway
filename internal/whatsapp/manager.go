package whatsapp

import (
	"context"
	"database/sql"

	"github.com/glennprays/whatsapp-gateway/config"
	errDomain "github.com/glennprays/whatsapp-gateway/domain/error"
	waDomain "github.com/glennprays/whatsapp-gateway/domain/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/utils"
	"github.com/glennprays/whatsapp-gateway/pkg/cipherx"
	log "github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

type (
	manager struct{}
	Manager interface {
		RegisterClient(ctx context.Context, phoneNumber string)
		LoginQRCode(ctx context.Context, phoneNumber string) (string, int, error)
		LoginPairCode(ctx context.Context, phoneNumber string) (string, int, error)
		LoginStatus(ctx context.Context, phoneNumber string) (bool, error)
		Logout(ctx context.Context, phoneNumber string) error
		Reconnect(ctx context.Context, phoneNumber string) error
		GetWebhookURL(ctx context.Context, phoneNumber string) (*string, error)
		SetWebhookURL(ctx context.Context, phoneNumber string, webhook *waDomain.Webhook) error
	}
)

var (
	Clients    map[string]*whatsmeow.Client
	DB         *sql.DB
	container  *sqlstore.Container
	cfg        *config.Config
	repository WhatsAppRepository
	cipher     *cipherx.Cipher
)

func init() {
	Clients = make(map[string]*whatsmeow.Client)
}

func NewManager(config *config.Config, dbType string, db *sql.DB, cp *cipherx.Cipher) Manager {
	ctx := context.Background()
	DB = db
	cfg = config
	cipher = cp

	dbLog := waLog.Stdout("Database", config.WhatsmeowLogLevel, true)
	container = sqlstore.NewWithDB(db, dbType, dbLog)
	if err := container.Upgrade(ctx); err != nil {
		log.Fatalf("Failed to upgrade database schema: %v", err)
	}

	err := runMigrations(db)
	if err != nil {
		log.Fatalf("Failed to run database migrations: %v", err)
	}

	repository = NewWhatsappRepository(db)

	devices, err := container.GetAllDevices(ctx)
	if err != nil {
		log.Fatalf("Failed to get devices from database: %v", err)
	}

	for _, device := range devices {
		phoneNumber := WhatsappDecomposeJID(device.ID.User)

		maskedPhoneNumber := MaskedPhoneNumber(phoneNumber)

		log.Infof("Restoring WhatsApp Client for %s", maskedPhoneNumber)
		InitClient(phoneNumber, device)

		if err := Reconnect(phoneNumber); err != nil {
			log.Errorf("Failed to reconnect WhatsApp client for %s: %v", maskedPhoneNumber, err)
		}
	}

	return &manager{}
}

func (m *manager) LoginQRCode(ctx context.Context, phoneNumber string) (string, int, error) {
	qr, timeout, err := LoginQRCode(ctx, phoneNumber)
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
		InitClient(phoneNumber, nil)
	} else {
		log.Warnf("WhatsApp client for %s already exists, skipping registration", MaskedPhoneNumber(phoneNumber))
	}
}

func (m *manager) LoginStatus(ctx context.Context, phoneNumber string) (bool, error) {
	return LoginStatus(phoneNumber)
}

func (m *manager) Logout(ctx context.Context, phoneNumber string) error {
	return Logout(ctx, phoneNumber)
}

func (m *manager) Reconnect(ctx context.Context, phoneNumber string) error {
	return Reconnect(phoneNumber)
}

func (m *manager) LoginPairCode(ctx context.Context, phoneNumber string) (string, int, error) {
	return LoginPairCode(ctx, phoneNumber)
}

func (m *manager) GetWebhookURL(ctx context.Context, phoneNumber string) (*string, error) {
	return GetWebhookURL(ctx, phoneNumber)
}

func (m *manager) SetWebhookURL(ctx context.Context, phoneNumber string, webhook *waDomain.Webhook) error {
	err := utils.ValidateURL(webhook.Url)
	if err != nil {
		log.Errorf("Invalid webhook URL for %s: %v", MaskedPhoneNumber(phoneNumber), err)
		return errDomain.NewError(errDomain.ErrBadRequest, err)
	}

	encryptedHmacSecret, err := cipher.Encrypt(webhook.HmacSecret)
	if err != nil {
		log.Errorf("Failed to encrypt HMAC secret for %s: %v", MaskedPhoneNumber(phoneNumber), err)
		return err
	}
	webhook.HmacSecret = encryptedHmacSecret
	return SetWebhookURL(ctx, phoneNumber, webhook)
}
