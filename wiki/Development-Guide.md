# Development Guide

This guide will help you set up and run the WhatsApp Gateway in development mode, as well as build it using Docker.

## Prerequisites

Before you begin, ensure you have the following installed:

- **Go 1.25 or higher** - [Download Go](https://golang.org/dl/)
- **Git** - For cloning the repository
- **SQLite or PostgreSQL** - For storing WhatsApp session data
- **Docker** (optional) - For containerized deployment

## Running in Development Mode

### 1. Clone the Repository

```bash
git clone https://github.com/glennprays/whatsapp-gateway.git
cd whatsapp-gateway
```

### 2. Configure Environment Variables

Copy the example environment file and configure it according to your needs:

```bash
cp .env.example .env
```

Edit the `.env` file with your preferred settings. See [Environment Variables](Environment-Variables.md) for detailed configuration options.

### 3. Install Dependencies

Download all required Go modules:

```bash
go mod download
```

### 4. Install Wire (Dependency Injection Tool)

The project uses [Google Wire](https://github.com/google/wire) for dependency injection. Install it:

```bash
go install github.com/google/wire/cmd/wire@latest
```

If you modify any `provider.go` files or `internal/infrastructure/wire.go`, regenerate the Wire code:

```bash
wire gen ./internal/infrastructure
```

### 5. Run the Application

You can run the application using either of these methods:

**Using Go directly:**
```bash
go run cmd/api/main.go
```

**Using Make:**
```bash
make run
```

The server will start on the configured port (default: 3000). You can access:
- **API endpoints**: `http://localhost:3000/api/v1/`
- **Swagger documentation**: `http://localhost:3000/docs/` (if enabled)

### 6. Verify Installation

Check if the server is running:

```bash
curl http://localhost:3000/api/v1/health
```

You should receive a response indicating the service is healthy.

## Docker Build and Deployment

### Building the Docker Image

Build the Docker image using the provided Dockerfile:

```bash
docker build -t whatsapp-gateway .
```

The Dockerfile uses a multi-stage build process:
1. **Stage 1**: Builds the Go application
2. **Stage 2**: Prepares CA certificates and timezone data
3. **Stage 3**: Creates a minimal image using scratch

### Running with Docker

**Basic run command:**
```bash
docker run -p 3000:3000 --env-file .env whatsapp-gateway
```

**With volume mounts (for SQLite database persistence):**
```bash
docker run -p 3000:3000 \
  --env-file .env \
  -v $(pwd)/dbs:/dbs \
  whatsapp-gateway
```

**Setting environment variables directly:**
```bash
docker run -p 3000:3000 \
  -e PORT=3000 \
  -e JWT_SECRET=your_secret_here \
  -e WHATSAPP_DATASTORE_TYPE=sqlite \
  -e WHATSAPP_DATASTORE_URI="file:/dbs/whatsapp.db?_pragma=foreign_keys(1)" \
  -v $(pwd)/dbs:/dbs \
  whatsapp-gateway
```

### Docker Compose (Optional)

For easier management, you can create a `docker-compose.yml` file:

```yaml
version: '3.8'

services:
  whatsapp-gateway:
    build: .
    ports:
      - "3000:3000"
    env_file:
      - .env
    volumes:
      - ./dbs:/dbs
    restart: unless-stopped
```

Then run:
```bash
docker-compose up -d
```

## Development Tips

### Understanding the Architecture

The project follows **Clean Architecture** with four layers:
1. **Presentation** - HTTP handlers (thin layer)
2. **Use Case** - Business logic
3. **Domain** - Core models and interfaces
4. **Infrastructure** - External services (WhatsApp, Database, Queue)

See the [Architecture Guide](Architecture-Guide.md) for detailed information.

### Working with Dependency Injection

The project uses Google Wire for compile-time DI. When you:
- Create a new service/usecase/handler
- Modify any `provider.go` file
- Update `internal/infrastructure/wire.go`

**Always run**: `wire gen ./internal/infrastructure`

### Database Management

**Using SQLite (default):**
- Database file will be created at `dbs/whatsapp.db`
- No additional setup required

**Using PostgreSQL:**
1. Install and start PostgreSQL
2. Create a database:
   ```sql
   CREATE DATABASE whatsapp_gateway;
   ```
3. Update your `.env` file:
   ```
   WHATSAPP_DATASTORE_TYPE=postgres
   WHATSAPP_DATASTORE_URI=postgresql://user:password@localhost:5432/whatsapp_gateway?sslmode=disable
   ```

### Viewing Logs

The application logs are written to stdout. In development mode, you'll see detailed logs in your console.

For Docker:
```bash
docker logs -f <container_id>
```

## Building for Production

To build a production binary:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o main ./cmd/api/main.go
```

This creates a statically linked binary that can run on any Linux system.

## Troubleshooting

### Port Already in Use
If port 3000 is already in use, change the `PORT` variable in your `.env` file:
```
PORT=8080
```

### Database Connection Issues
- Ensure the `dbs/` directory exists and is writable
- For PostgreSQL, verify the connection string and database credentials

### Module Download Issues
If you encounter issues downloading dependencies:
```bash
go clean -modcache
go mod download
```

## Next Steps

- Configure your environment variables: [Environment Variables](Environment-Variables.md)
- Learn how to use the gateway: [Gateway Usage Flow](Gateway-Usage-Flow.md)
- Understand security considerations: [Security Considerations](Security-Considerations.md)

---

[← Back to Home](Home.md)
