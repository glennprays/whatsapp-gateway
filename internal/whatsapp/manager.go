package whatsapp

import (
	"context"
	"database/sql"

	"github.com/glennprays/whatsapp-gateway/config"
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
		LoginStatus(ctx context.Context, phoneNumber string) (bool, error)
	}
)

var (
	Clients   map[string]*whatsmeow.Client
	DB        *sql.DB
	container *sqlstore.Container
	cfg       *config.Config
)

func init() {
	Clients = make(map[string]*whatsmeow.Client)
}

func NewManager(config *config.Config, dbType string, db *sql.DB) Manager {
	ctx := context.Background()
	DB = db
	cfg = config

	dbLog := waLog.Stdout("Database", config.WhatsmeowLogLevel, true)
	container = sqlstore.NewWithDB(db, dbType, dbLog)
	if err := container.Upgrade(ctx); err != nil {
		log.Fatalf("Failed to upgrade database schema: %v", err)
	}

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
