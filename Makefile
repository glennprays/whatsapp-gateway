.PHONY:
.SILENT:

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

run:
	go run -ldflags "-X main.version=$(VERSION)" cmd/api/main.go | jq -R 'fromjson? // { type: "raw", message: . }'

generate:
	@echo "Generating Wire dependencies..."
	@wire gen ./internal/infrastructure/...
