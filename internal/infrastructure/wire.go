//go:build wireinject
// +build wireinject

package infrastructure

import (
	"database/sql"

	"github.com/glennprays/log"
	"github.com/glennprays/whatsapp-gateway/config"
	"github.com/glennprays/whatsapp-gateway/internal/database"
	"github.com/glennprays/whatsapp-gateway/internal/handler"
	auth_handler "github.com/glennprays/whatsapp-gateway/internal/handler/auth"
	whatsapp_handler "github.com/glennprays/whatsapp-gateway/internal/handler/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	"github.com/glennprays/whatsapp-gateway/internal/router"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
	"github.com/glennprays/whatsapp-gateway/pkg/auth"
	"github.com/glennprays/whatsapp-gateway/pkg/cipherx"
	"github.com/gofiber/fiber/v2"
	"github.com/google/wire"
)

// ProvideConfig loads application configuration
func ProvideConfig() (*config.Config, error) {
	return config.Load()
}

// ProvideLogger initializes logger based on configuration
func ProvideLogger(cfg *config.Config) (*log.Logger, error) {
	// Map string level to log.Level
	var level log.Level
	switch cfg.LogLevel {
	case "debug":
		level = log.DebugLevel
	case "info":
		level = log.InfoLevel
	case "warn":
		level = log.WarnLevel
	case "error":
		level = log.ErrorLevel
	case "fatal":
		level = log.FatalLevel
	default:
		level = log.InfoLevel
	}

	// Map string output to log.OutputType
	var output log.OutputType
	if cfg.LogOutput == "file" {
		output = log.OutputFile
	} else {
		output = log.OutputStdout
	}

	logConfig := log.Config{
		Service:      "whatsapp-gateway",
		Env:          cfg.Env.String(),
		Level:        level,
		Output:       output,
		FilePath:     cfg.LogFilePath,
		EnableCaller: cfg.EnableCaller,
	}

	return log.New(logConfig)
}

// ProvideDatabase initializes database connection
func ProvideDatabase(cfg *config.Config, logger *log.Logger) (*sql.DB, error) {
	return database.NewConnection(logger, cfg.WhatsappDatastoreType, cfg.WhatsappDatastoreUri)
}

// ProvideCipher initializes encryption cipher
func ProvideCipher(cfg *config.Config) *cipherx.Cipher {
	return cipherx.NewCipher(cfg.WhatsappWebhookHmacEncryptionMasterKey)
}

// ProvideWhatsappManager initializes WhatsApp manager
func ProvideWhatsappManager(cfg *config.Config, db *sql.DB, cipher *cipherx.Cipher, logger *log.Logger) whatsapp.Manager {
	return whatsapp.NewManager(cfg, cfg.WhatsappDatastoreType, db, cipher, logger)
}

// ProvideJWTManager initializes JWT manager
func ProvideJWTManager(cfg *config.Config) *auth.JWTManager {
	return auth.NewJWTManager(cfg.JwtSecret, cfg.JwtIssuer, cfg.GetJwtDuration())
}

// ProvideAuthHandler initializes authentication handler
func ProvideAuthHandler(cfg *config.Config, jwtManager *auth.JWTManager, whatsappManager whatsapp.Manager, logger *log.Logger) *auth_handler.AuthHandler {
	return auth_handler.NewAuthHandler(cfg, jwtManager, whatsappManager, logger)
}

// ProvideWhatsappAuthHandler initializes WhatsApp authentication handler
func ProvideWhatsappAuthHandler(whatsappManager whatsapp.Manager, logger *log.Logger) *whatsapp_handler.WhatsappAuthHandler {
	return whatsapp_handler.NewWhatsappAuthHandler(whatsappManager, logger)
}

// ProvideWhatsappWebhookHandler initializes WhatsApp webhook handler
func ProvideWhatsappWebhookHandler(whatsappManager whatsapp.Manager, logger *log.Logger) *whatsapp_handler.WhatsappWebhookHandler {
	return whatsapp_handler.NewWhatsappWebhookHandler(whatsappManager, logger)
}

// ProvideMainHandler initializes main handler
func ProvideMainHandler(
	authHandler *auth_handler.AuthHandler,
	whatsappAuthHandler *whatsapp_handler.WhatsappAuthHandler,
	whatsappWebhookHandler *whatsapp_handler.WhatsappWebhookHandler,
) *handler.Handler {
	return handler.NewHandler(authHandler, whatsappAuthHandler, whatsappWebhookHandler)
}

// ProvideAuthMiddleware initializes authentication middleware
func ProvideAuthMiddleware(jwtManager *auth.JWTManager) *middleware.AuthMiddleware {
	return middleware.NewAuthMiddleware(jwtManager)
}

// ProvideTraceIDMiddleware initializes trace ID middleware
func ProvideTraceIDMiddleware(logger *log.Logger) fiber.Handler {
	return middleware.NewTraceIDMiddleware(logger)
}

// ProvideRouter sets up the router
func ProvideRouter(cfg *config.Config, traceIDMw fiber.Handler, authMiddleware *middleware.AuthMiddleware, mainHandler *handler.Handler, logger *log.Logger) *fiber.App {
	return router.SetupRouter(cfg, traceIDMw, authMiddleware, mainHandler, logger)
}

// App holds the application components
type App struct {
	FiberApp *fiber.App
	Config   *config.Config
	Logger   *log.Logger
}

// InitializeApp wires up all dependencies and returns App with cleanup function
func InitializeApp() (*App, func(), error) {
	wire.Build(
		ProvideConfig,
		ProvideLogger,
		ProvideDatabase,
		ProvideCipher,
		ProvideWhatsappManager,
		ProvideJWTManager,
		ProvideAuthHandler,
		ProvideWhatsappAuthHandler,
		ProvideWhatsappWebhookHandler,
		ProvideMainHandler,
		ProvideAuthMiddleware,
		ProvideTraceIDMiddleware,
		ProvideRouter,
		wire.Struct(new(App), "FiberApp", "Config", "Logger"),
	)
	return nil, nil, nil
}

// Cleanup syncs the logger before shutdown
func (a *App) Cleanup() {
	if err := a.Logger.Sync(); err != nil {
		// Ignore sync errors on stdout/stderr (common on Linux)
	}
}
