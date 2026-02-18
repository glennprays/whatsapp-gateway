package database

import (
	"database/sql"
	"fmt"

	"github.com/glennprays/log"
	"github.com/google/uuid"
)

func NewConnection(logger *log.Logger, driverName string, dataSouceName string) (*sql.DB, error) {
	dbTraceID := fmt.Sprintf("DB-INIT:%s", uuid.New().String())
	db, err := sql.Open(driverName, dataSouceName)
	if err != nil {
		logger.Warn(dbTraceID, "Error Open Database", log.Error(err))
		return nil, err
	}

	if err = db.Ping(); err != nil {
		logger.Warn(dbTraceID, "Error Ping Database", log.Error(err))
		return nil, err
	}
	logger.Info(dbTraceID, "Database connection established successfully", nil)
	return db, nil
}
