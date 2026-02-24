package storage

import (
	"fmt"
	"time"

	"github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	domainStorage "github.com/glennprays/whatsapp-gateway/domain/storage"
)

// ProvideStorage initializes the appropriate storage client based on configuration
func ProvideStorage(cfg *config.Config, logger *log.Logger) (domainStorage.Storage, error) {
	storageCfg := Config{
		Provider:            cfg.StorageProvider,
		Endpoint:            cfg.StorageS3Endpoint,
		AccessKeyID:         cfg.StorageS3AccessKeyID,
		SecretAccessKey:     cfg.StorageS3SecretAccessKey,
		Region:              cfg.StorageS3Region,
		Bucket:              cfg.StorageS3Bucket,
		UseSSL:              cfg.StorageS3UseSSL,
		PresignedURLExpiry:  time.Duration(cfg.StorageS3PresignedURLExpiry) * time.Second,
		LocalPath:           cfg.StorageLocalPath,
		BaseURL:             cfg.StorageBaseURL,
		RetentionDays:       cfg.WebhookMediaRetentionDays,
		AutoDeleteEnabled:   cfg.StorageAutoDeleteEnabled,
		AutoDeleteIntervalHours: cfg.StorageAutoDeleteIntervalHours,
	}

	switch cfg.StorageProvider {
	case ProviderS3:
		logger.Info("STORAGE-PROVIDER", "Initializing S3 storage", nil)
		return NewS3Storage(&storageCfg, logger)
	case ProviderLocal:
		logger.Info("STORAGE-PROVIDER", "Initializing local storage", nil)
		return NewLocalStorage(&storageCfg, logger)
	default:
		logger.Warn("STORAGE-PROVIDER", fmt.Sprintf("Unknown storage provider: %s, defaulting to local", cfg.StorageProvider), nil)
		return NewLocalStorage(&storageCfg, logger)
	}
}
