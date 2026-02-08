package queue

import (
	"database/sql"
)

// ProvideJobRepository initializes job tracking repository
func ProvideJobRepository(db *sql.DB) *JobRepository {
	return NewJobRepository(db)
}
