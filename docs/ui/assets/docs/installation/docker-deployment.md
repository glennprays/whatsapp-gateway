# Docker Deployment

## Purpose

This document describes how to deploy Whatsapp Gateway using Docker.  
Docker is the primary and recommended distribution method for both development and production environments.

The official container image:

glennprays/whatsapp-gateway

## Docker Image

The gateway is distributed as a single container image:

glennprays/whatsapp-gateway

This image contains:

- Compiled gateway binary
- Runtime dependencies
- Default internal configuration

The container exposes port:

3000 (internal container port)

## Running with Docker (Basic Example)

Minimal example using PostgreSQL without queue mode:

```bash
docker run -d \
  --name whatsapp-gateway \
  -p 9000:3000 \
  --env-file .env \
  glennprays/whatsapp-gateway
```

Explanation:

- `-p 9000:3000` maps host port 9000 to container port 3000.
- `--env-file .env` loads environment variables.
- `.env` must contain database configuration and JWT settings.

The service will be accessible at:

http://localhost:9000

## SQLite Database Persistence

### Important: Container Ephemeral Nature

The `/dbs` directory is pre-created in the container image, so SQLite works immediately. However:

- **Without volume mount**: Database exists only while container runs. Removing/recreating the container deletes all data (WhatsApp sessions, message tracking).
- **With volume mount**: Database persists across container restarts and recreations.

### Development/Testing (No Persistence)

For quick testing without persistence:
```bash
docker run -p 3000:3000 --env-file .env whatsapp-gateway
```

**Warning**: All data is lost when container stops.

### Production/Development with Persistence

For persistent data storage (recommended):
```bash
# Using local directory
docker run -p 3000:3000 \
  -v $(pwd)/data/whatsapp:/dbs \
  --env-file .env \
  whatsapp-gateway

# Using Docker volume
docker run -p 3000:3000 \
  -v whatsapp-data:/dbs \
  --env-file .env \
  whatsapp-gateway
```

### Docker Compose Example

```yaml
services:
  gateway:
    image: glennprays/whatsapp-gateway
    container_name: whatsapp-gateway
    restart: unless-stopped
    env_file:
      - .env
    volumes:
      - ./data/whatsapp:/dbs  # Persist SQLite database
    ports:
      - "9000:3000"
```

**Why persistence matters**:
- WhatsApp sessions are stored in the database
- Re-authentication (QR code scanning) required after data loss
- Message tracking history affects webhook delivery

## Docker Compose Deployment (Recommended)
For production-grade deployments, Docker Compose is recommended to orchestrate:
	•	Gateway
	•	PostgreSQL
	•	RabbitMQ (optional)

Example docker-compose.yml:
```
services:
  rabbitmq:
    image: rabbitmq:3-management
    container_name: whatsapp-rabbitmq
    restart: unless-stopped
    ports:
      - "5672:5672"
      - "15672:15672"

  gateway:
    image: glennprays/whatsapp-gateway
    container_name: whatsapp-gateway
    restart: unless-stopped
    env_file:
      - .env
    volumes:
      - ./dbs:/dbs
    depends_on:
      - postgres
      - rabbitmq
    ports:
      - "9000:3000"
```

Environtment variables in `.env` example:
```
# Server
PORT=3000
BASIC_AUTH_SECRET_KEY=secret
ENV=production

BASE_PATH=/api

# Database
WHATSAPP_DATASTORE_TYPE=sqlite
WHATSAPP_DATASTORE_URI=file:dbs/whatsapp.db?_pragma=foreign_keys(1)

# JWT
JWT_SECRET=your_jwt_secret

# Webhook
WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY=hmac_secret

# Queue Mode
RABBITMQ_ENABLED=true
RABBITMQ_URL=amqp://user:user@localhost:5672/
RABBITMQ_CONNECTION_NAME=whatsapp-gateway
```
All configuration must be provided through environment variables.
Refer to the Configuration section for a complete list.

### Port Mapping 
The container listens internally on:
3000
You may map it to any host port:
```
ports:
  - "9000:3000"
```
In this example:
	•	Internal container port: 3000
	•	Host access port: 9000

Access URL:
`http://localhost:9000`

### Queue Mode Consideration
If queue mode is enabled:
	•	RabbitMQ must be available. 
	•	The rabbitmq service must be reachable from the gateway container.

If queue mode is disabled:
	•	RabbitMQ service may be removed from docker-compose.
	•	The gateway operates in direct dispatch mode.

### Production Recommendations 
For production deployment:
	•	Do not expose PostgreSQL publicly.
	•	Do not expose RabbitMQ publicly.
	•	Place the gateway behind a reverse proxy.
	•	Enable HTTPS termination at the reverse proxy layer.
	•	Store secrets securely (do not hardcode in compose file).

Recommended architecture:
Client → Backend → Gateway → WhatsApp
The gateway should not be directly exposed to end users.

### Persistent Data
Ensure persistence for:
  • SQLite database volume (if using SQLite)
  • PostgreSQL volume (if using PostgreSQL)
  • RabbitMQ volume (if required)

Failure to persist database volume may result in:
  • Loss of WhatsApp session state
  • Loss of message tracking history
  • Need to re-scan QR code for WhatsApp reconnection

### Updating the Gateway 
To update: 
1. Pull the latest image:
```
docker pull glennprays/whatsapp-gateway
```

2. Restart the container:
```
docker-compose up -d
```

### Summary 
Docker deployment provides:
	•	Platform independence
	•	Simplified dependency management
	•	Predictable runtime behavior
	•	Clear separation of infrastructure components

It is the recommended method for running Whatsapp Gateway in both development and production environments.
