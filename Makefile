.PHONY:
.SILENT:

run:
	go run cmd/api/main.go

generate:
	@echo "Generating Wire dependencies..."
	@wire gen ./internal/infrastructure/...
