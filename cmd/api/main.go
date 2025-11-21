package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/glennprays/whatsapp-gateway/config"
	"github.com/glennprays/whatsapp-gateway/docs"
	"github.com/glennprays/whatsapp-gateway/internal/database"
	"github.com/glennprays/whatsapp-gateway/internal/handler"
	auth_handler "github.com/glennprays/whatsapp-gateway/internal/handler/auth"
	whatsapp_handler "github.com/glennprays/whatsapp-gateway/internal/handler/whatsapp"
	"github.com/glennprays/whatsapp-gateway/internal/middleware"
	"github.com/glennprays/whatsapp-gateway/internal/router"
	"github.com/glennprays/whatsapp-gateway/internal/whatsapp"
	"github.com/glennprays/whatsapp-gateway/pkg/auth"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.Println("Starting WhatsApp Gateway...")

	log.Println("loading configuration...")
	cfg := config.LoadConfig()

	log.Println("initializing Swagger...")
	docs.NewSwagger(cfg)

	log.Println("initializing database connection...")
	db, err := database.NewConnection(cfg.WhatsappDatastoreType, cfg.WhatsappDatastoreUri)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	if db.Ping() != nil {
		log.Fatalf("failed to ping database: %v", err)
	}
	log.Println("initializing WhatsApp manager...")
	whatsappManager := whatsapp.NewManager(cfg, cfg.WhatsappDatastoreType, db)

	log.Println("initializing JWT manager...")
	jwtManager := auth.NewJWTManager(cfg.JwtSecret, cfg.JwtIssuer, cfg.JwtDuration)

	log.Println("initializing handlers...")
	authHandler := auth_handler.NewAuthHandler(
		cfg,
		jwtManager,
		whatsappManager,
	)
	whatsappAuthHandler := whatsapp_handler.NewWhatsappAuthHandler(whatsappManager)
	whatsappWebhookHandler := whatsapp_handler.NewWhatsappWebhookHandler(whatsappManager)

	log.Println("initializing main handler...")
	mainHandler := handler.NewHandler(
		authHandler,
		whatsappAuthHandler,
		whatsappWebhookHandler,
	)

	log.Println("initializing Auth middleware...")
	authMiddleware := middleware.NewAuthMiddleware(jwtManager)

	log.Println("setting up router...")
	routerEngine := router.SetupRouter(
		cfg,
		authMiddleware,
		mainHandler,
	)

	log.Println("setting up HTTP server...")
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      routerEngine,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("starting server on port %s...", cfg.Port)

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server ListenAndServe: %v", err)
		}
		log.Println("HTTP server stopped.")
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutdown signal received, initiating graceful shutdown...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("server exiting gracefully...")
}
