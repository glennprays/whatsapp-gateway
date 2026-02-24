package storage

import "time"

// Config holds storage client configuration
type Config struct {
	// Provider selects the storage backend ("s3" or "local")
	Provider string

	// S3 Configuration
	Endpoint            string
	AccessKeyID         string
	SecretAccessKey     string
	Region              string
	Bucket              string
	UseSSL              bool
	PresignedURLExpiry  time.Duration

	// Local Configuration
	LocalPath string

	// BaseURL is the base URL for public access (for local provider)
	// This is used when constructing public URLs for local files
	BaseURL string

	// Lifecycle Configuration
	RetentionDays          int
	AutoDeleteEnabled       bool
	AutoDeleteIntervalHours int
}
