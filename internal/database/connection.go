package database

import (
	"database/sql"

	log "github.com/sirupsen/logrus"
)

func NewConnection(driverName string, dataSouceName string) (*sql.DB, error) {
	db, err := sql.Open(driverName, dataSouceName)
	if err != nil {
		log.Warnf("Error Open Database: %v", err)
	}

	if err = db.Ping(); err != nil {
		log.Warnf("Error Ping Database: %v", err)
	}
	log.Info("Database connection established successfully")
	return db, nil
}
