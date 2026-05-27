package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	"github.com/google/uuid"
)

func NewConnection(logger *log.Logger, cfg *config.Config, driverName string, dataSouceName string) (*sql.DB, error) {
	dbTraceID := fmt.Sprintf("DB-INIT:%s", uuid.New().String())
	db, err := sql.Open(driverName, dataSouceName)
	if err != nil {
		logger.Warn(dbTraceID, "Error Open Database", log.Error(err))
		return nil, err
	}

	db.SetMaxOpenConns(cfg.DBMaxOpenConns)
	db.SetMaxIdleConns(cfg.DBMaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(cfg.DBConnMaxLifeMins) * time.Minute)

	if err = db.Ping(); err != nil {
		logger.Warn(dbTraceID, "Error Ping Database", log.Error(err))
		return nil, err
	}
	logger.Info(dbTraceID, "Database connection established successfully", nil)
	return db, nil
}
