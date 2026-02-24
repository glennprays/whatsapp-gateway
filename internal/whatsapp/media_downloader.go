package whatsapp

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"time"

	customLog "github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	domainStorage "github.com/glennprays/whatsapp-gateway/domain/storage"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/binary/proto"
	"github.com/google/uuid"
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

	// Determine file extension
	ext := md.getExtension(mediaType, mediaMessage)

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

	// Get public URL
	storageURL := md.storage.GetPublicURL("", key)
	md.logger.Debug(traceID, fmt.Sprintf("Media stored successfully: %s", storageURL), nil)

	return storageURL, nil
}

func (md *mediaDownloader) generateStorageKey(phoneNumber string, ext string) string {
	// Format: {prefix}/{phone_number}/{year}/{month}/{uuid}.{ext}
	now := time.Now()
	return filepath.Join(
		md.config.WebhookMediaStoragePrefix,
		phoneNumber,
		fmt.Sprintf("%d", now.Year()),
		fmt.Sprintf("%02d", now.Month()),
		fmt.Sprintf("%s%s", uuid.New().String(), ext),
	)
}

func (md *mediaDownloader) getExtension(mediaType string, mediaMessage DownloadableMessage) string {
	// Try to get extension from mime type if available
	var mimeType string
	switch m := mediaMessage.(type) {
	case *proto.ImageMessage:
		if m.Mimetype != nil {
			mimeType = *m.Mimetype
		}
	case *proto.VideoMessage:
		if m.Mimetype != nil {
			mimeType = *m.Mimetype
		}
	case *proto.AudioMessage:
		if m.Mimetype != nil {
			mimeType = *m.Mimetype
		}
	case *proto.DocumentMessage:
		if m.Mimetype != nil {
			mimeType = *m.Mimetype
		}
	}

	// Map mime types to extensions
	switch mimeType {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "video/mp4":
		return ".mp4"
	case "video/3gpp":
		return ".3gp"
	case "audio/mpeg", "audio/mp3":
		return ".mp3"
	case "audio/aac":
		return ".aac"
	case "audio/ogg":
		return ".ogg"
	case "application/pdf":
		return ".pdf"
	case "text/plain":
		return ".txt"
	default:
		// Default extensions by media type
		switch mediaType {
		case "image":
			return ".jpg"
		case "video":
			return ".mp4"
		case "audio":
			return ".mp3"
		case "document":
			return ".pdf"
		case "sticker":
			return ".webp"
		default:
			return ".bin"
		}
	}
}

func (md *mediaDownloader) getContentType(mediaMessage DownloadableMessage, mediaType string) string {
	switch m := mediaMessage.(type) {
	case *proto.ImageMessage:
		if m.Mimetype != nil {
			return *m.Mimetype
		}
	case *proto.VideoMessage:
		if m.Mimetype != nil {
			return *m.Mimetype
		}
	case *proto.AudioMessage:
		if m.Mimetype != nil {
			return *m.Mimetype
		}
	case *proto.DocumentMessage:
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
