package main

import (
	"log"

	"github.com/glennprays/whatsapp-gateway/config"
	"github.com/glennprays/whatsapp-gateway/internal/infrastructure"
)

// version is the running build version, injected at build time via
// -ldflags "-X main.version=...". Defaults to "dev" for local `go run`.
var version = "dev"

func main() {
	config.AppVersion = version
	if err := infrastructure.Run(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}
