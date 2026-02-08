package main

import (
	"log"

	"github.com/glennprays/whatsapp-gateway/internal/infrastructure"
)

func main() {
	if err := infrastructure.Run(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}
