package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/glennprays/whatsapp-gateway/config"
	"github.com/glennprays/whatsapp-gateway/internal/infrastructure"
)

// version is the running build version, injected at build time via
// -ldflags "-X main.version=...". Defaults to "dev" for local `go run`.
var version = "dev"

func main() {
	// The runtime image is FROM scratch: no shell, curl, or wget for the Docker
	// healthcheck to exec. The container instead runs `/main health`, which
	// probes the local HTTP server and exits 0/1 per Docker's convention.
	if len(os.Args) > 1 && os.Args[1] == "health" {
		os.Exit(healthProbe())
	}

	config.AppVersion = version
	if err := infrastructure.Run(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}

// healthProbe GETs the local /health endpoint (honoring PORT/BASE_PATH) and
// exits 0 on any 2xx/3xx, 1 otherwise. Bounded so a wedged server fails the
// check instead of hanging it.
func healthProbe() int {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	url := fmt.Sprintf("http://localhost:%s%s/health", port, "/"+strings.Trim(os.Getenv("BASE_PATH"), "/"))
	if strings.HasSuffix(url, "//health") {
		url = fmt.Sprintf("http://localhost:%s/health", port)
	}

	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "health probe failed: %v\n", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		fmt.Fprintf(os.Stderr, "health probe got status %d\n", resp.StatusCode)
		return 1
	}
	return 0
}
