package queue

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type JobRepository struct {
	db *sql.DB
}

type MessageJob struct {
	JobID        string
	PhoneNumber  string
	Status       string
	MessageID    *string
	ErrorMessage *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	CompletedAt  *time.Time
}

func NewJobRepository(db *sql.DB) *JobRepository {
	return &JobRepository{db: db}
}

func (r *JobRepository) Create(ctx context.Context, jobID, status, phoneNumber string) error {
	query := `
		INSERT INTO message_jobs (job_id, phone_number, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`

	now := time.Now()
	_, err := r.db.ExecContext(ctx, query, jobID, phoneNumber, status, now, now)
	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	return nil
}

func (r *JobRepository) UpdateStatus(ctx context.Context, jobID, status, messageID, errorMessage string) error {
	now := time.Now()

	var completedAt *time.Time
	if status == "completed" || status == "failed" {
		completedAt = &now
	}

	query := `
		UPDATE message_jobs
		SET status = ?, message_id = ?, error_message = ?, updated_at = ?, completed_at = ?
		WHERE job_id = ?
	`

	var msgID *string
	if messageID != "" {
		msgID = &messageID
	}

	var errMsg *string
	if errorMessage != "" {
		errMsg = &errorMessage
	}

	_, err := r.db.ExecContext(ctx, query, status, msgID, errMsg, now, completedAt, jobID)
	if err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	return nil
}

func (r *JobRepository) Get(ctx context.Context, jobID string) (*MessageJob, error) {
	query := `
		SELECT job_id, phone_number, status, message_id, error_message, created_at, updated_at, completed_at
		FROM message_jobs
		WHERE job_id = ?
	`

	var job MessageJob
	err := r.db.QueryRowContext(ctx, query, jobID).Scan(
		&job.JobID,
		&job.PhoneNumber,
		&job.Status,
		&job.MessageID,
		&job.ErrorMessage,
		&job.CreatedAt,
		&job.UpdatedAt,
		&job.CompletedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	return &job, nil
}
