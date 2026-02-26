package router

import (
	"github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	domainStorage "github.com/glennprays/whatsapp-gateway/domain/storage"
	domainQueue "github.com/glennprays/whatsapp-gateway/domain/queue"
	"github.com/glennprays/whatsapp-gateway/internal/handler"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
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
) *fiber.App {
	return SetupRouter(cfg, traceIDMw, authMiddleware, mainHandler, logger, queue, storage)
}
