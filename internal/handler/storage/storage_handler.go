package storage

import (
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/glennprays/whatsapp-gateway/domain/storage"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	"github.com/gofiber/fiber/v2"
)

// StorageHandler handles file serving operations
type StorageHandler struct {
	storage storage.Storage
}

// NewStorageHandler creates a new storage handler
func NewStorageHandler(storage storage.Storage) *StorageHandler {
	return &StorageHandler{
		storage: storage,
	}
}

// GetFile serves a file from storage
func (h *StorageHandler) GetFile(c *fiber.Ctx) error {
	traceID := middleware.GetTraceID(c)
	ctx := c.Context()

	// Get file key from path parameter
	key := c.Params("*") // Wildcard to capture full path
	if key == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "File key is required",
		})
	}

	// Get file from storage using storage abstraction
	bucket := "" // Use default bucket
	reader, fileInfo, err := h.storage.GetFile(ctx, traceID, bucket, key)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "File not found",
		})
	}
	defer reader.Close()

	// Set proper response headers from FileInfo
	c.Set("Content-Type", fileInfo.ContentType)
	c.Set("Content-Length", strconv.FormatInt(fileInfo.Size, 10))
	c.Set("Content-Disposition", formatContentDisposition(key))
	c.Set("Cache-Control", "public, max-age=31536000") // 1 year cache
	c.Set("Accept-Ranges", "bytes")
	c.Set("Last-Modified", fileInfo.LastModified.UTC().Format(http.TimeFormat))
	c.Set("ETag", fileInfo.ETag)

	// Stream file to response
	_, err = io.Copy(c, reader)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to stream file",
		})
	}

	return nil
}

// formatContentDisposition returns a safe Content-Disposition header value.
func formatContentDisposition(filename string) string {
	base := filepath.Base(filename)
	return mime.FormatMediaType("inline", map[string]string{"filename": base})
}
