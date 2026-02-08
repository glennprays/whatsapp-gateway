package database

import (
	"database/sql"

	"github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
)

// ProvideDatabase initializes database connection
func ProvideDatabase(cfg *config.Config, logger *log.Logger) (*sql.DB, error) {
	return NewConnection(logger, cfg.WhatsappDatastoreType, cfg.WhatsappDatastoreUri)
}
