package whatsapp

import (
	"context"
	"database/sql"

	log "github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
)

type (
	manager struct{}
	Manager interface{}
)

var (
	Clients   map[string]*whatsmeow.Client
	DB        *sql.DB
	container *sqlstore.Container
)

func NewManager(dbType string, db *sql.DB) Manager {
	ctx := context.Background()
	DB = db

	container = sqlstore.NewWithDB(db, dbType, nil)
	if err := container.Upgrade(ctx); err != nil {
		log.Fatalf("Failed to upgrade database schema: %v", err)
	}

	devices, err := container.GetAllDevices(ctx)
	if err != nil {
		log.Fatalf("Failed to get devices from database: %v", err)
	}

	for _, device := range devices {
		jid := WhatsappDecomposeJID(device.ID.User)

		// Mask JID for Logging Information
		maskedJID := MaskedJID(jid)

		// Print Restore Log
		log.Infof("Restoring WhatsApp Client for %s", maskedJID)
		InitClient(jid, device)

		if err := Reconnect(jid); err != nil {
			log.Errorf("Failed to reconnect WhatsApp client for %s: %v", maskedJID, err)
		}
	}

	return &manager{}
}
