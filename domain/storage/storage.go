package storage

import (
	"context"
	"io"
	"time"
)

// Storage represents an object storage abstraction
type Storage interface {
	// UploadFile uploads a file and returns the object key
	UploadFile(ctx context.Context, traceID string, bucket, key string, reader io.Reader, size int64, contentType string) (string, error)

	// GetFile reads a file and returns its content with metadata
	GetFile(ctx context.Context, traceID string, bucket, key string) (io.ReadCloser, *FileInfo, error)

	// GetPublicURL returns a publicly accessible URL for the object
	GetPublicURL(bucket, key string) string

	// GetPresignedURL returns a time-limited signed URL for the object
	GetPresignedURL(ctx context.Context, traceID string, bucket, key string, expiry time.Duration) (string, error)

	// CreateFolder creates a folder (prefix) in the bucket
	CreateFolder(ctx context.Context, traceID string, bucket, prefix string) error

	// SetFolderAccess sets access control for a folder/prefix
	SetFolderAccess(ctx context.Context, traceID string, bucket, prefix string, isPublic bool) error

	// DeleteFile deletes an object from storage
	DeleteFile(ctx context.Context, traceID string, bucket, key string) error

	// ListFiles lists objects under a prefix
	ListFiles(ctx context.Context, traceID string, bucket, prefix string, limit int) ([]FileInfo, error)

	// IsHealthy checks if the storage service is accessible
	IsHealthy() bool
}

// FileInfo represents metadata about a stored object
type FileInfo struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
	ContentType  string
}
