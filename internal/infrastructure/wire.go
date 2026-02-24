//go:build wireinject
// +build wireinject

package infrastructure

import (
	"github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	"github.com/glennprays/whatsapp-gateway/internal/database"
	"github.com/glennprays/whatsapp-gateway/internal/handler"
	auth_handler "github.com/glennprays/whatsapp-gateway/internal/handler/auth"
	storage_handler "github.com/glennprays/whatsapp-gateway/internal/handler/storage"
	whatsapp_handler "github.com/glennprays/whatsapp-gateway/internal/handler/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	"github.com/glennprays/whatsapp-gateway/internal/queue"
	"github.com/glennprays/whatsapp-gateway/internal/router"
	auth_usecase "github.com/glennprays/whatsapp-gateway/internal/usecase/auth"
	whatsapp_usecase "github.com/glennprays/whatsapp-gateway/internal/usecase/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
	"github.com/glennprays/whatsapp-gateway/pkg/auth"
	"github.com/glennprays/whatsapp-gateway/pkg/cipherx"
	pkgQueue "github.com/glennprays/whatsapp-gateway/pkg/queue"
	"github.com/glennprays/whatsapp-gateway/pkg/ratelimiter"
	"github.com/glennprays/whatsapp-gateway/pkg/storage"
	"github.com/gofiber/fiber/v2"
	"github.com/google/wire"
)

// App holds the application components
type App struct {
	FiberApp *fiber.App
	Config   *config.Config
	Logger   *log.Logger
	Workers  *pkgQueue.WorkerManager
}

// InitializeApp wires up all dependencies and returns App with cleanup function
func InitializeApp() (*App, func(), error) {
	wire.Build(
		// Configuration & Logging
		config.ProvideConfig,
		config.ProvideLogger,

		// Database
		database.ProvideDatabase,
		database.ProvideRedis,

		// ratelimiter
		ratelimiter.ProvideRateLimiter,

		// Infrastructure
		cipherx.ProvideCipher,
		pkgQueue.ProvideMessageQueue,
		queue.ProvideJobRepository,
		storage.ProvideStorage,

		// WhatsApp Domain
		whatsapp.ProvideWhatsappManager,
		whatsapp.ProvideWhatsAppRepository,
		whatsapp.ProvideWebhookSender,

		// Authentication
		auth.ProvideJWTManager,

		// Usecases
		auth_usecase.ProvideAuthUsecase,
		whatsapp_usecase.ProvideWhatsappAuthUsecase,
		whatsapp_usecase.ProvideWhatsappWebhookUsecase,
		whatsapp_usecase.ProvideWhatsappMessageUsecase,

		// Handlers
		auth_handler.ProvideAuthHandler,
		whatsapp_handler.ProvideWhatsappAuthHandler,
		whatsapp_handler.ProvideWhatsappWebhookHandler,
		whatsapp_handler.ProvideWhatsappMessageHandler,
		storage_handler.ProvideStorageHandler,
		handler.ProvideMainHandler,

		// Middleware
		middleware.ProvideAuthMiddleware,
		middleware.ProvideTraceIDMiddleware,

		// Infrastructure Setup
		ProvideQueueWorkers,

		// Router
		router.ProvideRouter,

		// App struct
		wire.Struct(new(App), "FiberApp", "Config", "Logger", "Workers"),
	)
	return nil, nil, nil
}

// Cleanup syncs the logger before shutdown
func (a *App) Cleanup() {
	if err := a.Logger.Sync(); err != nil {
		// Ignore sync errors on stdout/stderr (common on Linux)
	}
}
