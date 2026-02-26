package whatsapp

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	customLog "github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	domainStorage "github.com/glennprays/whatsapp-gateway/domain/storage"
	"github.com/google/uuid"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
)

// DownloadableMessage interface from whatsmeow
type DownloadableMessage interface {
	GetDirectPath() string
	GetMediaKey() []byte
	GetFileSHA256() []byte
	GetFileEncSHA256() []byte
	GetURL() string
}

type (
	mediaDownloader struct {
		storage       domainStorage.Storage
		config        *config.Config
		logger        *customLog.Logger
		getClientFunc func(phoneNumber string) *whatsmeow.Client
	}

	MediaDownloader interface {
		DownloadAndStoreMedia(
			ctx context.Context,
			traceID string,
			phoneNumber string,
			mediaMessage DownloadableMessage,
			mediaType string,
		) (string, error)
	}
)

func NewMediaDownloader(
	storage domainStorage.Storage,
	cfg *config.Config,
	logger *customLog.Logger,
	getClientFunc func(phoneNumber string) *whatsmeow.Client,
) MediaDownloader {
	return &mediaDownloader{
		storage:       storage,
		config:        cfg,
		logger:        logger,
		getClientFunc: getClientFunc,
	}
}

func (md *mediaDownloader) DownloadAndStoreMedia(
	ctx context.Context,
	traceID string,
	phoneNumber string,
	mediaMessage DownloadableMessage,
	mediaType string,
) (string, error) {
	// Check if feature is enabled
	if !md.config.WebhookMediaDownloadEnabled {
		return "", nil
	}

	// Get WhatsApp client
	client := md.getClientFunc(phoneNumber)
	if client == nil {
		md.logger.Error(traceID, "No WhatsApp client found for phone number", nil)
		return "", fmt.Errorf("client not found")
	}

	// Create context with timeout
	downloadCtx, cancel := context.WithTimeout(ctx, time.Duration(md.config.WebhookMediaDownloadTimeoutSeconds)*time.Second)
	defer cancel()

	// Download media from WhatsApp
	md.logger.Debug(traceID, fmt.Sprintf("Downloading media type: %s", mediaType), nil)
	mediaBytes, err := client.Download(downloadCtx, mediaMessage)
	if err != nil {
		md.logger.Error(traceID, "Failed to download media from WhatsApp", nil, customLog.Error(err))
		return "", fmt.Errorf("failed to download media: %w", err)
	}

	// Validate file size
	maxSize := md.config.WebhookMediaMaxFileSizeBytes
	if int64(len(mediaBytes)) > maxSize {
		md.logger.Error(traceID, fmt.Sprintf("Media file size %d bytes exceeds limit %d", len(mediaBytes), maxSize), nil)
		return "", fmt.Errorf("media file size %d bytes exceeds limit %d", len(mediaBytes), maxSize)
	}

	// Determine file extension
	ext, err := md.getExtension(mediaType, mediaMessage)
	if err != nil {
		md.logger.Error(traceID, "Failed to determine file extension", nil, customLog.Error(err))
		return "", fmt.Errorf("failed to determine file extension: %w", err)
	}

	// Generate storage key
	key := md.generateStorageKey(phoneNumber, ext)

	// Determine content type
	contentType := md.getContentType(mediaMessage, mediaType)

	// Upload to storage
	md.logger.Debug(traceID, fmt.Sprintf("Uploading media to storage: %s", key), nil)
	_, err = md.storage.UploadFile(
		ctx,
		traceID,
		"", // default bucket
		key,
		bytes.NewReader(mediaBytes),
		int64(len(mediaBytes)),
		contentType,
	)
	if err != nil {
		md.logger.Error(traceID, "Failed to upload media to storage", nil, customLog.Error(err))
		return "", fmt.Errorf("failed to upload to storage: %w", err)
	}

	// Get URL based on storage provider
	var storageURL string
	if md.config.StorageProvider == "s3" {
		// Always use presigned URLs for S3
		presignedURL, err := md.storage.GetPresignedURL(ctx, traceID, "", key, 0)
		if err != nil {
			md.logger.Error(traceID, "Failed to generate presigned URL", nil, customLog.Error(err))
			return "", fmt.Errorf("failed to generate presigned URL: %w", err)
		}
		storageURL = presignedURL
	} else {
		// Local storage - use public URL
		storageURL = md.storage.GetPublicURL("", key)
	}

	md.logger.Debug(traceID, fmt.Sprintf("Media stored successfully: %s", storageURL), nil)

	return storageURL, nil
}

func (md *mediaDownloader) generateStorageKey(phoneNumber string, ext string) string {
	// Sanitize phone number to prevent path traversal attacks
	// Only allow digits and plus sign, replace anything else with underscore
	safePhone := strings.Map(func(r rune) rune {
		if (r >= '0' && r <= '9') || r == '+' {
			return r
		}
		return '_'
	}, strings.TrimSpace(phoneNumber))

	// Also sanitize extension to prevent any path characters
	safeExt := strings.Map(func(r rune) rune {
		if r == '.' {
			return r
		}
		if r >= 'a' && r <= 'z' {
			return r
		}
		if r >= 'A' && r <= 'Z' {
			return r
		}
		if r >= '0' && r <= '9' {
			return r
		}
		return '_'
	}, ext)

	// Format: {prefix}/{phone_number}/{year}/{month}/{uuid}.{ext}
	now := time.Now()
	return filepath.Join(
		md.config.WebhookMediaStoragePrefix,
		safePhone,
		fmt.Sprintf("%d", now.Year()),
		fmt.Sprintf("%02d", now.Month()),
		fmt.Sprintf("%s%s", uuid.New().String(), safeExt),
	)
}

func (md *mediaDownloader) getExtension(mediaType string, mediaMessage DownloadableMessage) (string, error) {
	// Try to get extension from mime type if available
	var mimeType string
	switch m := mediaMessage.(type) {
	case *waE2E.ImageMessage:
		if m.Mimetype != nil {
			mimeType = *m.Mimetype
		}
	case *waE2E.VideoMessage:
		if m.Mimetype != nil {
			mimeType = *m.Mimetype
		}
	case *waE2E.AudioMessage:
		if m.Mimetype != nil {
			mimeType = *m.Mimetype
		}
	case *waE2E.DocumentMessage:
		if m.Mimetype != nil {
			mimeType = *m.Mimetype
		}
	}

	// Validate MIME type against allowed list
	if mimeType != "" {
		allowedMimeTypes := map[string]bool{
			"image/jpeg":     true,
			"image/jpg":      true,
			"image/png":      true,
			"image/gif":      true,
			"image/webp":     true,
			"video/mp4":      true,
			"video/3gpp":     true,
			"audio/mpeg":     true,
			"audio/mp3":      true,
			"audio/aac":      true,
			"audio/ogg":      true,
			"application/pdf": true,
			"text/plain":     true,
		}

		if !allowedMimeTypes[mimeType] {
			return "", fmt.Errorf("unsupported MIME type: %s", mimeType)
		}
	}

	// Map mime types to extensions
	switch mimeType {
	case "image/jpeg", "image/jpg":
		return ".jpg", nil
	case "image/png":
		return ".png", nil
	case "image/gif":
		return ".gif", nil
	case "image/webp":
		return ".webp", nil
	case "video/mp4":
		return ".mp4", nil
	case "video/3gpp":
		return ".3gp", nil
	case "audio/mpeg", "audio/mp3":
		return ".mp3", nil
	case "audio/aac":
		return ".aac", nil
	case "audio/ogg":
		return ".ogg", nil
	case "application/pdf":
		return ".pdf", nil
	case "text/plain":
		return ".txt", nil
	default:
		// Default extensions by media type
		switch mediaType {
		case "image":
			return ".jpg", nil
		case "video":
			return ".mp4", nil
		case "audio":
			return ".mp3", nil
		case "document":
			return ".pdf", nil
		case "sticker":
			return ".webp", nil
		default:
			return ".bin", nil
		}
	}
}

func (md *mediaDownloader) getContentType(mediaMessage DownloadableMessage, mediaType string) string {
	switch m := mediaMessage.(type) {
	case *waE2E.ImageMessage:
		if m.Mimetype != nil {
			return *m.Mimetype
		}
	case *waE2E.VideoMessage:
		if m.Mimetype != nil {
			return *m.Mimetype
		}
	case *waE2E.AudioMessage:
		if m.Mimetype != nil {
			return *m.Mimetype
		}
	case *waE2E.DocumentMessage:
		if m.Mimetype != nil {
			return *m.Mimetype
		}
	}
	// Default content types
	switch mediaType {
	case "image":
		return "image/jpeg"
	case "video":
		return "video/mp4"
	case "audio":
		return "audio/mpeg"
	case "document":
		return "application/pdf"
	case "sticker":
		return "image/webp"
	default:
		return "application/octet-stream"
	}
}
