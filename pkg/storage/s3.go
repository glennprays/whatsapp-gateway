package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/set"

	"github.com/glennprays/log"
	domainStorage "github.com/glennprays/whatsapp-gateway/domain/storage"
)

// S3Storage implements the Storage interface using S3/S3-compatible storage
type S3Storage struct {
	client     *minio.Client
	config     *Config
	logger     *log.Logger
	bucket     string
	healthy    bool
	mu         struct {
		sync.RWMutex
		policies map[string]string // prefix -> policy JSON
	}
}

// NewS3Storage creates a new S3 storage client
func NewS3Storage(cfg *Config, logger *log.Logger) (domainStorage.Storage, error) {
	if cfg.Endpoint == "" {
		return nil, errors.New("S3 endpoint is required")
	}
	if cfg.AccessKeyID == "" {
		return nil, errors.New("S3 access key ID is required")
	}
	if cfg.SecretAccessKey == "" {
		return nil, errors.New("S3 secret access key is required")
	}
	if cfg.Bucket == "" {
		return nil, errors.New("S3 bucket name is required")
	}

	// Initialize minio client
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 client: %w", err)
	}

	s := &S3Storage{
		client:  client,
		config:  cfg,
		logger:  logger,
		bucket:  cfg.Bucket,
		healthy: false,
	}

	// Initialize policies map
	s.mu.policies = make(map[string]string)

	// Create bucket if it doesn't exist
	ctx := context.Background()
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		err = client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{
			Region: cfg.Region,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
		logger.Info("STORAGE-INIT", fmt.Sprintf("Created bucket: %s", cfg.Bucket), nil)
	}

	// Set healthy after successful bucket check
	s.healthy = true

	return s, nil
}

// UploadFile uploads a file to S3 and returns the object key
func (s *S3Storage) UploadFile(ctx context.Context, bucket, key string, reader io.Reader, size int64, contentType string) (string, error) {
	if !s.healthy {
		return "", errors.New("storage client is not healthy")
	}

	traceID := fmt.Sprintf("STORAGE-UPLOAD:%s", key)

	// Use configured bucket if not specified
	if bucket == "" {
		bucket = s.bucket
	}

	// Upload the file
	_, err := s.client.PutObject(ctx, bucket, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		s.logger.Error(traceID, fmt.Sprintf("Failed to upload file (bucket=%s, key=%s)", bucket, key), nil, log.Error(err))
		return "", fmt.Errorf("failed to upload file: %w", err)
	}

	s.logger.Debug(traceID, fmt.Sprintf("File uploaded successfully (bucket=%s, key=%s, size=%d)", bucket, key, size), nil)

	return key, nil
}

// GetPublicURL returns a publicly accessible URL for the object
func (s *S3Storage) GetPublicURL(bucket, key string) string {
	if bucket == "" {
		bucket = s.bucket
	}

	scheme := "https"
	if !s.config.UseSSL {
		scheme = "http"
	}

	return fmt.Sprintf("%s://%s/%s/%s", scheme, s.config.Endpoint, bucket, key)
}

// GetPresignedURL returns a time-limited signed URL for the object
func (s *S3Storage) GetPresignedURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	if !s.healthy {
		return "", errors.New("storage client is not healthy")
	}

	if bucket == "" {
		bucket = s.bucket
	}

	if expiry <= 0 {
		expiry = s.config.PresignedURLExpiry
	}

	presignedURL, err := s.client.PresignedGetObject(ctx, bucket, key, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return presignedURL.String(), nil
}

// CreateFolder creates a folder (prefix) in the bucket
func (s *S3Storage) CreateFolder(ctx context.Context, bucket, prefix string) error {
	if !s.healthy {
		return errors.New("storage client is not healthy")
	}

	traceID := fmt.Sprintf("STORAGE-CREATE-FOLDER:%s", prefix)

	if bucket == "" {
		bucket = s.bucket
	}

	// Ensure prefix ends with /
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	// Create a placeholder object with the prefix
	_, err := s.client.PutObject(ctx, bucket, prefix, strings.NewReader(""), 0, minio.PutObjectOptions{})
	if err != nil {
		s.logger.Error(traceID, fmt.Sprintf("Failed to create folder (bucket=%s, prefix=%s)", bucket, prefix), nil, log.Error(err))
		return fmt.Errorf("failed to create folder: %w", err)
	}

	s.logger.Debug(traceID, fmt.Sprintf("Folder created successfully (bucket=%s, prefix=%s)", bucket, prefix), nil)

	return nil
}

// SetFolderAccess sets access control for a folder/prefix via bucket policy
func (s *S3Storage) SetFolderAccess(ctx context.Context, bucket, prefix string, isPublic bool) error {
	if !s.healthy {
		return errors.New("storage client is not healthy")
	}

	traceID := fmt.Sprintf("STORAGE-SET-ACCESS:%s", prefix)

	if bucket == "" {
		bucket = s.bucket
	}

	// Ensure prefix ends with /
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	if isPublic {
		// Add read permission for this prefix
		if err := s.setPrefixPolicy(ctx, bucket, prefix, true); err != nil {
			s.logger.Error(traceID, fmt.Sprintf("Failed to set folder public (bucket=%s, prefix=%s)", bucket, prefix), nil, log.Error(err))
			return fmt.Errorf("failed to set folder public: %w", err)
		}
		s.logger.Info(traceID, fmt.Sprintf("Folder set to public (bucket=%s, prefix=%s)", bucket, prefix), nil)
	} else {
		// Remove read permission for this prefix
		if err := s.setPrefixPolicy(ctx, bucket, prefix, false); err != nil {
			s.logger.Error(traceID, fmt.Sprintf("Failed to set folder private (bucket=%s, prefix=%s)", bucket, prefix), nil, log.Error(err))
			return fmt.Errorf("failed to set folder private: %w", err)
		}
		s.logger.Info(traceID, fmt.Sprintf("Folder set to private (bucket=%s, prefix=%s)", bucket, prefix), nil)
	}

	return nil
}

// setPrefixPolicy adds or removes a prefix from the bucket policy
func (s *S3Storage) setPrefixPolicy(ctx context.Context, bucket, prefix string, add bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get existing policy
	policyStr, err := s.client.GetBucketPolicy(ctx, bucket)
	if err != nil {
		// No policy exists, create new one
		var errResp minio.ErrorResponse
		if errors.As(err, &errResp) && errResp.Code == "NoSuchBucketPolicy" {
			policyStr = s.getDefaultBucketPolicy(bucket)
		} else {
			return err
		}
	}

	// Parse policy
	var policy map[string]interface{}
	if err := json.Unmarshal([]byte(policyStr), &policy); err != nil {
		return fmt.Errorf("failed to parse bucket policy: %w", err)
	}

	// Get or create Statement array
	statements, ok := policy["Statement"].([]interface{})
	if !ok {
		statements = []interface{}{}
	}

	// Find or create public read statement
	var publicReadStmt map[string]interface{}
	var stmtIndex int = -1

	for i, stmt := range statements {
		s, ok := stmt.(map[string]interface{})
		if !ok {
			continue
		}
		sid, ok := s["Sid"].(string)
		if ok && sid == "PublicReadGetObject" {
			publicReadStmt = s
			stmtIndex = i
			break
		}
	}

	if add {
		// Add prefix to public read statement
		if publicReadStmt == nil {
			// Create new statement
			publicReadStmt = map[string]interface{}{
				"Sid":    "PublicReadGetObject",
				"Effect": "Allow",
				"Principal": map[string]string{
					"AWS": "*",
				},
				"Action":   []string{"s3:GetObject"},
				"Resource": []string{},
			}
			statements = append(statements, publicReadStmt)
		}

		// Add prefix to resources
		resources, ok := publicReadStmt["Resource"].([]interface{})
		if !ok {
			resources = []interface{}{}
		}

		// Ensure we use a set to avoid duplicates
		resourceSet := set.NewStringSet()
		for _, r := range resources {
			if s, ok := r.(string); ok {
				resourceSet.Add(s)
			}
		}

		resourcePattern := fmt.Sprintf("arn:aws:s3:::%s%s*", bucket, prefix)
		if !resourceSet.Contains(resourcePattern) {
			resourceSet.Add(resourcePattern)
			publicReadStmt["Resource"] = resourceSet.ToSlice()
		}

		// Update statement
		if stmtIndex >= 0 {
			statements[stmtIndex] = publicReadStmt
		}
	} else {
		// Remove prefix from public read statement
		if publicReadStmt != nil {
			resources, ok := publicReadStmt["Resource"].([]interface{})
			if ok {
				var newResources []interface{}
				for _, r := range resources {
					rs, ok := r.(string)
					if !ok {
						continue
					}
					// Keep resources that don't match this prefix
					if !strings.HasPrefix(rs, fmt.Sprintf("arn:aws:s3:::%s%s", bucket, prefix)) {
						newResources = append(newResources, rs)
					}
				}

				if len(newResources) > 0 {
					publicReadStmt["Resource"] = newResources
					if stmtIndex >= 0 {
						statements[stmtIndex] = publicReadStmt
					}
				} else {
					// Remove the entire statement if no resources left
					if stmtIndex >= 0 {
						statements = append(statements[:stmtIndex], statements[stmtIndex+1:]...)
					}
				}
			}
		}
	}

	// Update policy
	if len(statements) == 0 {
		// No statements left, delete the policy by setting an empty policy
		emptyPolicy := map[string]interface{}{
			"Version":   "2012-10-17",
			"Statement": []interface{}{},
		}
		policyBytes, err := json.Marshal(emptyPolicy)
		if err != nil {
			return fmt.Errorf("failed to marshal empty bucket policy: %w", err)
		}

		err = s.client.SetBucketPolicy(ctx, bucket, string(policyBytes))
		if err != nil {
			return fmt.Errorf("failed to clear bucket policy: %w", err)
		}
	} else {
		policy["Statement"] = statements
		policyBytes, err := json.Marshal(policy)
		if err != nil {
			return fmt.Errorf("failed to marshal bucket policy: %w", err)
		}

		err = s.client.SetBucketPolicy(ctx, bucket, string(policyBytes))
		if err != nil {
			return fmt.Errorf("failed to set bucket policy: %w", err)
		}
	}

	return nil
}

// getDefaultBucketPolicy returns a default bucket policy structure
func (s *S3Storage) getDefaultBucketPolicy(bucket string) string {
	policy := map[string]interface{}{
		"Version": "2012-10-17",
		"Statement": []map[string]interface{}{},
	}

	policyBytes, _ := json.Marshal(policy)
	return string(policyBytes)
}

// DeleteFile deletes an object from storage
func (s *S3Storage) DeleteFile(ctx context.Context, bucket, key string) error {
	if !s.healthy {
		return errors.New("storage client is not healthy")
	}

	traceID := fmt.Sprintf("STORAGE-DELETE:%s", key)

	if bucket == "" {
		bucket = s.bucket
	}

	err := s.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		s.logger.Error(traceID, fmt.Sprintf("Failed to delete file (bucket=%s, key=%s)", bucket, key), nil, log.Error(err))
		return fmt.Errorf("failed to delete file: %w", err)
	}

	s.logger.Debug(traceID, fmt.Sprintf("File deleted successfully (bucket=%s, key=%s)", bucket, key), nil)

	return nil
}

// ListFiles lists objects under a prefix
func (s *S3Storage) ListFiles(ctx context.Context, bucket, prefix string, limit int) ([]domainStorage.FileInfo, error) {
	if !s.healthy {
		return nil, errors.New("storage client is not healthy")
	}

	traceID := fmt.Sprintf("STORAGE-LIST:%s", prefix)

	if bucket == "" {
		bucket = s.bucket
	}

	if limit <= 0 {
		limit = 100
	}

	var files []domainStorage.FileInfo

	// List objects
	objectsCh := s.client.ListObjects(ctx, bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: false,
	})

	for object := range objectsCh {
		if object.Err != nil {
			s.logger.Error(traceID, fmt.Sprintf("Error listing files (bucket=%s, prefix=%s)", bucket, prefix), nil, log.Error(object.Err))
			continue
		}

		// Skip prefix placeholders (objects ending with /)
		if strings.HasSuffix(object.Key, "/") && object.Size == 0 {
			continue
		}

		fileInfo := domainStorage.FileInfo{
			Key:          object.Key,
			Size:         object.Size,
			LastModified: object.LastModified,
			ETag:         object.ETag,
			ContentType:  object.ContentType,
		}

		files = append(files, fileInfo)

		if limit > 0 && len(files) >= limit {
			break
		}
	}

	s.logger.Debug(traceID, fmt.Sprintf("Files listed successfully (bucket=%s, prefix=%s, count=%d)", bucket, prefix, len(files)), nil)

	return files, nil
}

// IsHealthy checks if the storage service is accessible
func (s *S3Storage) IsHealthy() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.healthy {
		return false
	}

	// Perform a quick health check by checking if bucket exists
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		s.logger.Warn("STORAGE-HEALTH", fmt.Sprintf("Health check failed (bucket=%s)", s.bucket), nil, log.Error(err))
		s.healthy = false
		return false
	}

	return true
}
