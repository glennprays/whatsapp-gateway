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
	•	PostgreSQL volume
	•	RabbitMQ volume (if required)

Failure to persist database volume may result in:
	•	Loss of WhatsApp session
	•	Loss of message tracking history

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
