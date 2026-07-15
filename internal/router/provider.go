package router

import (
	"database/sql"

	"github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
	domainStorage "github.com/glennprays/whatsapp-gateway/domain/storage"
	"github.com/glennprays/whatsapp-gateway/internal/handler"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
	"github.com/gofiber/fiber/v2"
)

// ProvideRouter sets up the router
func ProvideRouter(
	cfg *config.Config,
	traceIDMw fiber.Handler,
	authMiddleware *middleware.AuthMiddleware,
	mainHandler *handler.Handler,
	logger *log.Logger,
	queue domainQueue.MessageQueue,
	storage domainStorage.Storage,
	db *sql.DB,
	manager whatsapp.Manager,
) *fiber.App {
	return SetupRouter(cfg, traceIDMw, authMiddleware, mainHandler, logger, queue, storage, db, manager)
}
