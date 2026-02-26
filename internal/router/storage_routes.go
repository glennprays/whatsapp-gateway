package router

import (
	"github.com/gofiber/fiber/v2"
	"github.com/glennprays/whatsapp-gateway/internal/handler/storage"
)

// RegisterStorageRoutes registers storage routes for serving files
func RegisterStorageRoutes(app *fiber.App, storageHandler *storage.StorageHandler, basePath string) {
	// File serving route - serves files from storage
	// basePath is configurable (default: /storage)
	app.Get(basePath+"/*", storageHandler.GetFile)
}
