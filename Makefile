.PHONY:
.SILENT:

run:
	go run cmd/api/main.go | jq

generate:
	@echo "Generating Wire dependencies..."
	@wire gen ./internal/infrastructure/...
