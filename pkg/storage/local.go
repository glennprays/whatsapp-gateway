package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/glennprays/log"
	domainStorage "github.com/glennprays/whatsapp-gateway/domain/storage"
)

// LocalStorage implements the Storage interface using local filesystem
type LocalStorage struct {
	config  *Config
	logger  *log.Logger
	baseURL string
	healthy bool
}

// NewLocalStorage creates a new local filesystem storage client
func NewLocalStorage(cfg *Config, logger *log.Logger) (domainStorage.Storage, error) {
	if cfg.LocalPath == "" {
		return nil, errors.New("local storage path is required")
	}

	// Create the base directory if it doesn't exist
	if err := os.MkdirAll(cfg.LocalPath, 0750); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Check if directory is writable
	testFile := filepath.Join(cfg.LocalPath, ".write_test")
	if err := os.WriteFile(testFile, []byte("test"), 0640); err != nil {
		return nil, fmt.Errorf("storage directory is not writable: %w", err)
	}
	os.Remove(testFile)

	// Determine base URL for public access
	baseURL := cfg.BaseURL
	if baseURL == "" {
		// Default to empty string - caller should construct full URL
		baseURL = ""
	}

	s := &LocalStorage{
		config:  cfg,
		logger:  logger,
		baseURL: baseURL,
		healthy: true,
	}

	logger.Info("STORAGE-INIT", fmt.Sprintf("Local storage initialized at: %s", cfg.LocalPath), nil)

	return s, nil
}

// UploadFile uploads a file to local filesystem and returns the object key
func (s *LocalStorage) UploadFile(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) (string, error) {
	if !s.healthy {
		return "", errors.New("storage client is not healthy")
	}

	traceID := fmt.Sprintf("STORAGE-UPLOAD:%s", key)

	// Determine full file path
	fullPath := s.getFullPath(bucket, key)

	// Create directory structure
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		s.logger.Error(traceID, fmt.Sprintf("Failed to create directory (path=%s)", dir), nil, log.Error(err))
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Create the file
	file, err := os.Create(fullPath)
	if err != nil {
		s.logger.Error(traceID, fmt.Sprintf("Failed to create file (path=%s)", fullPath), nil, log.Error(err))
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy content
	copied, err := io.Copy(file, reader)
	if err != nil {
		s.logger.Error(traceID, fmt.Sprintf("Failed to write file (path=%s)", fullPath), nil, log.Error(err))
		// Remove partially written file
		os.Remove(fullPath)
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// Set file permissions
	if err := file.Chmod(0640); err != nil {
		s.logger.Warn(traceID, fmt.Sprintf("Failed to set file permissions (path=%s)", fullPath), nil, log.Error(err))
	}

	s.logger.Debug(traceID, fmt.Sprintf("File uploaded successfully (path=%s, size=%d)", fullPath, copied), nil)

	return key, nil
}

// GetPublicURL returns a publicly accessible URL for the object
func (s *LocalStorage) GetPublicURL(bucket, key string) string {
	if s.baseURL != "" {
		// Use configured base URL
		return fmt.Sprintf("%s/%s", strings.TrimSuffix(s.baseURL, "/"), key)
	}
	// Return just the key - caller can construct full URL
	return key
}

// GetPresignedURL returns a time-limited signed URL for the object
// For local storage, this just returns the public URL since we rely on
// the reverse proxy or web server for access control
func (s *LocalStorage) GetPresignedURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	// Local storage doesn't support presigned URLs in the S3 sense
	// Access is controlled by file permissions and web server configuration
	return s.GetPublicURL(bucket, key), nil
}

// CreateFolder creates a folder (prefix) in the storage
func (s *LocalStorage) CreateFolder(ctx context.Context, bucket, prefix string) error {
	if !s.healthy {
		return errors.New("storage client is not healthy")
	}

	traceID := fmt.Sprintf("STORAGE-CREATE-FOLDER:%s", prefix)

	// Ensure prefix ends with /
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	fullPath := s.getFullPath(bucket, prefix)

	// Create directory
	if err := os.MkdirAll(fullPath, 0750); err != nil {
		s.logger.Error(traceID, fmt.Sprintf("Failed to create folder (path=%s)", fullPath), nil, log.Error(err))
		return fmt.Errorf("failed to create folder: %w", err)
	}

	s.logger.Debug(traceID, fmt.Sprintf("Folder created successfully (path=%s)", fullPath), nil)

	return nil
}

// SetFolderAccess sets access control for a folder/prefix via file permissions
func (s *LocalStorage) SetFolderAccess(ctx context.Context, bucket, prefix string, isPublic bool) error {
	if !s.healthy {
		return errors.New("storage client is not healthy")
	}

	traceID := fmt.Sprintf("STORAGE-SET-ACCESS:%s", prefix)

	// Ensure prefix ends with /
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	fullPath := s.getFullPath(bucket, prefix)

	// Check if folder exists
	_, err := os.Stat(fullPath)
	if err != nil {
		s.logger.Error(traceID, fmt.Sprintf("Failed to get folder info (path=%s)", fullPath), nil, log.Error(err))
		return fmt.Errorf("failed to get folder info: %w", err)
	}

	var mode os.FileMode
	if isPublic {
		// Make readable by all: rwxr-xr-x (0755)
		mode = 0755
	} else {
		// Make private: rwx------ (0700)
		mode = 0700
	}

	// Apply permissions recursively
	err = filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		// Set permissions
		var targetMode os.FileMode
		if info.IsDir() {
			targetMode = mode
		} else {
			// For files, remove execute bit: rw-r--r-- (0644) or rw------- (0600)
			targetMode = mode & 0666
		}

		if err := os.Chmod(path, targetMode); err != nil {
			// Some systems don't support chmod
			if errors.Is(err, syscall.ENOTSUP) || errors.Is(err, syscall.EPERM) {
				s.logger.Debug(traceID, fmt.Sprintf("Chmod not supported, skipping (path=%s)", path), nil)
				return nil
			}
			return err
		}

		return nil
	})

	if err != nil {
		s.logger.Error(traceID, fmt.Sprintf("Failed to set folder permissions (path=%s)", fullPath), nil, log.Error(err))
		return fmt.Errorf("failed to set folder permissions: %w", err)
	}

	s.logger.Info(traceID, fmt.Sprintf("Folder permissions updated (path=%s, public=%v)", fullPath, isPublic), nil)

	return nil
}

// DeleteFile deletes an object from storage
func (s *LocalStorage) DeleteFile(ctx context.Context, bucket, key string) error {
	if !s.healthy {
		return errors.New("storage client is not healthy")
	}

	traceID := fmt.Sprintf("STORAGE-DELETE:%s", key)

	fullPath := s.getFullPath(bucket, key)

	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", key)
	}

	// Delete the file
	if err := os.Remove(fullPath); err != nil {
		s.logger.Error(traceID, fmt.Sprintf("Failed to delete file (path=%s)", fullPath), nil, log.Error(err))
		return fmt.Errorf("failed to delete file: %w", err)
	}

	s.logger.Debug(traceID, fmt.Sprintf("File deleted successfully (path=%s)", fullPath), nil)

	// Try to clean up empty directories (best effort)
	s.cleanupEmptyDirs(fullPath)

	return nil
}

// ListFiles lists objects under a prefix
func (s *LocalStorage) ListFiles(ctx context.Context, bucket, prefix string, limit int) ([]domainStorage.FileInfo, error) {
	if !s.healthy {
		return nil, errors.New("storage client is not healthy")
	}

	traceID := fmt.Sprintf("STORAGE-LIST:%s", prefix)

	basePath := s.getFullPath(bucket, prefix)

	if limit <= 0 {
		limit = 100
	}

	var files []domainStorage.FileInfo

	// Walk the directory
	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip errors on individual files
			return nil
		}

		// Skip the base directory itself
		if path == basePath {
			return nil
		}

		// Skip directories (they're represented as prefixes in object storage)
		if info.IsDir() {
			return nil
		}

		// Get relative path as the key
		relPath, err := filepath.Rel(s.config.LocalPath, path)
		if err != nil {
			return nil
		}
		key := filepath.ToSlash(relPath)

		fileInfo := domainStorage.FileInfo{
			Key:          key,
			Size:         info.Size(),
			LastModified: info.ModTime(),
			ETag:         fmt.Sprintf("%d", info.ModTime().UnixNano()), // Use mtime as ETag
		}

		files = append(files, fileInfo)

		if limit > 0 && len(files) >= limit {
			return io.EOF // Stop walking
		}

		return nil
	})

	if err != nil && err != io.EOF {
		s.logger.Error(traceID, fmt.Sprintf("Error listing files (path=%s)", basePath), nil, log.Error(err))
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	s.logger.Debug(traceID, fmt.Sprintf("Files listed successfully (path=%s, count=%d)", basePath, len(files)), nil)

	return files, nil
}

// IsHealthy checks if the storage service is accessible
func (s *LocalStorage) IsHealthy() bool {
	if !s.healthy {
		return false
	}

	// Perform a quick health check by checking if directory is writable
	testFile := filepath.Join(s.config.LocalPath, ".health_check")
	if err := os.WriteFile(testFile, []byte("health"), 0640); err != nil {
		s.logger.Warn("STORAGE-HEALTH", fmt.Sprintf("Health check failed (path=%s)", s.config.LocalPath), nil, log.Error(err))
		s.healthy = false
		return false
	}
	os.Remove(testFile)

	return true
}

// getFullPath returns the full filesystem path for a given key
func (s *LocalStorage) getFullPath(bucket, key string) string {
	// For local storage, we ignore the bucket parameter and use the configured path
	// This allows us to map multiple "buckets" to subdirectories if needed
	if bucket != "" {
		return filepath.Join(s.config.LocalPath, bucket, key)
	}
	return filepath.Join(s.config.LocalPath, key)
}

// cleanupEmptyDirs removes empty parent directories (best effort)
func (s *LocalStorage) cleanupEmptyDirs(path string) {
	dir := filepath.Dir(path)
	baseDir := s.config.LocalPath

	for dir != baseDir && dir != "." {
		// Check if directory is empty
		entries, err := os.ReadDir(dir)
		if err != nil {
			break
		}
		if len(entries) == 0 {
			os.Remove(dir)
			dir = filepath.Dir(dir)
		} else {
			break
		}
	}
}
