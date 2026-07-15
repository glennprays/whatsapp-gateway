# Binary Build

## Purpose

This document explains how to build and run Whatsapp Gateway as a native Go binary.

Binary execution is intended for:

- Development environments
- Custom builds
- Source-level modifications
- Advanced infrastructure setups

Docker remains the recommended deployment method for production.

## Requirements

Minimum required version:

Go 1.25

Verify installation:

```bash
go version
```

Ensure your environment supports Go modules.

## Clone Repository

```bash
git clone https://github.com/glennprays/whatsapp-gateway.git
cd whatsapp-gateway
```

## Install Dependencies

```bash
go mod tidy
```

This ensures all required modules are downloaded.

## Build the Binary

```bash
go build -o whatsapp-gateway
```

This produces a binary named:

whatsapp-gateway

For optimized production build:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o whatsapp-gateway
```

Adjust `GOOS` and `GOARCH` as needed.

---

## Running the Binary

The gateway requires environment variables for configuration.

Example:

```bash
export PORT=3000
export WHATSAPP_DATASTORE_TYPE=sqlite
export WHATSAPP_DATASTORE_URI=file:dbs/whatsapp.db?_pragma=foreign_keys(1)
export JWT_SECRET=your_jwt_secret
export WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY=hmac_secret
export RABBITMQ_ENABLED=true
export RABBITMQ_URL=amqp://user:user@localhost:5672/
export RABBITMQ_CONNECTION_NAME=whatsapp-gateway

./whatsapp-gateway
```

The service will start on:

http://localhost:3000

---

## Configuration Handling

All configuration must be provided through environment variables.

The binary does not require a configuration file by default.

Refer to the Configuration section for the full variable reference.

---

## Database Requirement

The database is mandatory.

Ensure:

- PostgreSQL 16+ is running (recommended for production)
- SQLite file path is properly configured if using SQLite

The application will fail to start if database connection cannot be established.

---

## RabbitMQ (Optional)

If queue mode is enabled:

- RabbitMQ 3.x must be running
- `RABBITMQ_ENABLED` must be set to `true`

If queue mode is disabled:

- RabbitMQ is not required
- Outbound messages are dispatched immediately

---

## Recommended Development Workflow

For development:

1. Use PostgreSQL locally or via Docker.
2. Use SQLite for quick experimentation.
3. Disable queue mode unless testing queue behavior.
4. Enable verbose logging if available.

Binary execution is suitable for debugging and testing code modifications.

---

## Production Considerations

When running as a binary in production:

- Use a process manager (e.g., systemd)
- Configure automatic restart
- Use reverse proxy for TLS termination
- Secure environment variables
- Restrict network exposure

Example systemd service:

```ini
[Unit]
Description=Whatsapp Gateway
After=network.target

[Service]
ExecStart=/usr/local/bin/whatsapp-gateway
Restart=always
User=whatsapp
EnvironmentFile=/etc/whatsapp-gateway.env

[Install]
WantedBy=multi-user.target
```

---

## Cross Compilation

Example cross-compilation for Linux from macOS:

```bash
GOOS=linux GOARCH=amd64 go build -o whatsapp-gateway
```

Ensure compatibility with your deployment target.

---

## When to Use Binary Deployment

Binary deployment is appropriate when:

- Docker is not allowed in the environment
- You require system-level process control
- You need customized builds
- You integrate into existing VM-based infrastructure

For most users, Docker deployment remains simpler and safer.
