.PHONY:
.SILENT:

run:
	go run cmd/api/main.go | jq -R 'fromjson? // { type: "raw", message: . }'

generate:
	@echo "Generating Wire dependencies..."
	@wire gen ./internal/infrastructure/...
