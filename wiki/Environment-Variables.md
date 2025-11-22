# Environment Variables Configuration

This page provides a comprehensive reference for all environment variables used to configure the WhatsApp Gateway.

## Configuration Overview

The WhatsApp Gateway uses environment variables for configuration. Copy `.env.example` to `.env` and customize the values according to your needs.

```bash
cp .env.example .env
```

## Configuration Sections

### Server Configuration

#### `PORT`
- **Description**: The port on which the HTTP server will listen
- **Type**: Integer
- **Default**: `3000`
- **Example**: `PORT=3000`
- **Note**: If running in Docker, ensure this matches the exposed port in your Docker configuration

#### `BASIC_AUTH_SECRET_KEY`
- **Description**: Secret key for basic authentication (if used)
- **Type**: String
- **Default**: `secret`
- **Example**: `BASIC_AUTH_SECRET_KEY=my_secure_secret_key_123`
- **Security**: Change this in production environments

---

### HTTP Configuration

#### `BASE_PATH`
- **Description**: **Dynamic base path** for all API endpoints. This allows you to mount the API at any path prefix.
- **Type**: String
- **Default**: `/api`
- **Example**: `BASE_PATH=/api` or `BASE_PATH=/gateway` or `BASE_PATH=/whatsapp`
- **Usage**: If set to `/api`, all endpoints will be accessible at `http://localhost:3000/api/v1/*`
- **Note**: This is dynamic and can be changed to match your infrastructure needs (e.g., if using a reverse proxy or API gateway)

---

### Swagger Documentation Configuration

#### `ENABLE_SWAGGER`
- **Description**: Enable or disable the Swagger UI documentation interface
- **Type**: Boolean
- **Default**: `true`
- **Example**: `ENABLE_SWAGGER=true`
- **Note**: Set to `false` in production for security

#### `SWAGGER_USER`
- **Description**: **Username required to access the Swagger documentation**
- **Type**: String
- **Default**: `secret`
- **Example**: `SWAGGER_USER=admin`
- **Security**: The Swagger docs are protected by basic authentication. You must provide this username to access `/docs`

#### `SWAGGER_PASSWORD`
- **Description**: **Password required to access the Swagger documentation**
- **Type**: String
- **Default**: `secret`
- **Example**: `SWAGGER_PASSWORD=secure_password_123`
- **Security**: The Swagger docs are protected by basic authentication. You must provide this password to access `/docs`

#### `SWAGGER_BASE_PATH`
- **Description**: **Dynamic base path** for the Swagger documentation interface
- **Type**: String
- **Default**: `/docs`
- **Example**: `SWAGGER_BASE_PATH=/docs` or `SWAGGER_BASE_PATH=/api-docs`
- **Usage**: Access Swagger UI at `http://localhost:3000/docs` (or your configured path)
- **Note**: This path is dynamic and can be customized to match your documentation URL structure

**⚠️ Important**: The Swagger documentation requires both username and password for access. When accessing the docs at the configured base path, you'll be prompted for credentials.

---

### JWT Configuration

#### `JWT_SECRET`
- **Description**: Secret key used to sign and verify JWT tokens
- **Type**: String
- **Default**: `secret`
- **Example**: `JWT_SECRET=your_very_secure_random_string_here`
- **Security**: **CRITICAL** - Use a long, random, and secure secret in production
- **Note**: Changing this will invalidate all existing tokens

#### `JWT_DURATION_MINUTES`
- **Description**: Duration (in minutes) for which the JWT token remains valid
- **Type**: Integer
- **Default**: `60`
- **Example**: `JWT_DURATION_MINUTES=1440` (24 hours)
- **Note**: Balance security (shorter duration) with user convenience (longer duration)

#### `JWT_ISSUER`
- **Description**: The issuer claim for JWT tokens (identifies who issued the token)
- **Type**: String
- **Default**: `whatsapp-gateway`
- **Example**: `JWT_ISSUER=whatsapp-gateway`
- **Note**: This is a standard JWT claim used for token validation

**JWT Overview**: This gateway uses industry-standard JSON Web Tokens (JWT) for authentication. When you register via the `/register` endpoint, you receive a JWT token that must be included in the `Authorization: Bearer <token>` header for all subsequent API requests. The JWT contains claims about the authenticated phone number and is signed with the `JWT_SECRET` to prevent tampering.

---

### WhatsApp Configuration

#### `WHATSAPP_DATASTORE_TYPE`
- **Description**: Type of database to use for storing WhatsApp session data and device information
- **Type**: String (Enum)
- **Allowed Values**: 
  - `sqlite` or `sqlite3` - Use SQLite database (file-based, good for development and small deployments)
  - `postgres` - Use PostgreSQL database (recommended for production)
- **Default**: `sqlite`
- **Example**: 
  ```
  WHATSAPP_DATASTORE_TYPE=sqlite3
  # or
  WHATSAPP_DATASTORE_TYPE=postgres
  ```
- **Note**: The value for SQLite can be either `"sqlite"` or `"sqlite3"` - both are accepted

#### `WHATSAPP_DATASTORE_URI`
- **Description**: Connection string/URI for the selected datastore
- **Type**: String (Connection URI)
- **Examples**:
  
  **For SQLite:**
  ```
  WHATSAPP_DATASTORE_URI=file:dbs/whatsapp.db?_pragma=foreign_keys(1)
  ```
  - `file:dbs/whatsapp.db` - Path to the SQLite database file
  - `?_pragma=foreign_keys(1)` - Enables foreign key constraints
  
  **For PostgreSQL:**
  ```
  WHATSAPP_DATASTORE_URI=postgresql://username:password@localhost:5432/whatsapp_gateway?sslmode=disable
  # or
  WHATSAPP_DATASTORE_URI=postgres://username:password@localhost:5432/whatsapp_gateway?sslmode=require
  ```
  - Replace `username`, `password`, `localhost`, `5432`, and `whatsapp_gateway` with your PostgreSQL credentials
  - `sslmode=disable` for local development, `sslmode=require` for production

- **Note**: Ensure the directory exists for SQLite, or the database exists for PostgreSQL

#### `WHATSAPP_DEVICE_LABEL`
- **Description**: Label/name for the WhatsApp device that will appear in WhatsApp's "Linked Devices" section
- **Type**: String
- **Default**: `"Whatsapp Gateway"`
- **Example**: `WHATSAPP_DEVICE_LABEL="My Company WhatsApp Gateway"`
- **Note**: This helps identify the gateway when viewing connected devices in WhatsApp

#### `WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY`
- **Description**: **Master encryption key used to encrypt the HMAC secrets for device webhooks before storing them in the database**
- **Type**: String (Hexadecimal, must be 32 characters representing 16 bytes)
- **Default**: `0123456789abcdef0123456789abcdef`
- **Example**: `WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY=a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6`
- **Security**: **CRITICAL** - This key is used to encrypt HMAC secrets before they are stored in the database
- **Purpose**: 
  - When you register a webhook with an `hmac_secret`, this secret is encrypted using this master key before being stored
  - The encrypted HMAC secret is then used to sign webhook payloads sent to your backend
  - This ensures that even if the database is compromised, the actual HMAC secrets remain protected
- **Note**: 
  - Must be exactly 32 hexadecimal characters (16 bytes for AES-128 encryption)
  - Use a cryptographically secure random string
  - Changing this key will make existing encrypted webhook secrets unreadable
  - Generate using: `openssl rand -hex 16`

**Webhook HMAC Flow**:
1. You register a webhook URL with an optional `hmac_secret`
2. The gateway encrypts this `hmac_secret` with the `WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY` and stores it
3. When a WhatsApp event occurs (e.g., incoming message), the gateway:
   - Decrypts the stored HMAC secret
   - Signs the webhook payload with the decrypted secret
   - Sends the payload with the HMAC signature to your backend
4. Your backend can verify the signature using the original `hmac_secret` you provided

---

## Security Best Practices

1. **Never commit `.env` files**: Always use `.env.example` as a template
2. **Use strong secrets**: Generate cryptographically secure random strings for all secret keys
3. **Protect Swagger docs**: Use strong credentials for `SWAGGER_USER` and `SWAGGER_PASSWORD`
4. **Disable Swagger in production**: Set `ENABLE_SWAGGER=false` in production
5. **Use PostgreSQL in production**: SQLite is good for development, but PostgreSQL is recommended for production
6. **Secure your database**: Ensure proper authentication and network security for your database
7. **Rotate secrets regularly**: Periodically update JWT secrets and encryption keys (note: this will invalidate existing tokens/data)

## Generating Secure Secrets

Use these commands to generate secure random strings:

**For JWT_SECRET and BASIC_AUTH_SECRET_KEY:**
```bash
openssl rand -base64 32
```

**For WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY (32 hex characters):**
```bash
openssl rand -hex 16
```

## Example Configuration Files

### Development (.env.development)
```env
PORT=3000
BASIC_AUTH_SECRET_KEY=dev_secret
BASE_PATH=/api
ENABLE_SWAGGER=true
SWAGGER_USER=admin
SWAGGER_PASSWORD=admin
SWAGGER_BASE_PATH=/docs
JWT_SECRET=dev_jwt_secret_key
JWT_DURATION_MINUTES=60
JWT_ISSUER=whatsapp-gateway-dev
WHATSAPP_DATASTORE_TYPE=sqlite3
WHATSAPP_DATASTORE_URI=file:dbs/whatsapp.db?_pragma=foreign_keys(1)
WHATSAPP_DEVICE_LABEL="Development Gateway"
WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY=0123456789abcdef0123456789abcdef
```

### Production (.env.production)
```env
PORT=3000
BASIC_AUTH_SECRET_KEY=<strong-random-secret>
BASE_PATH=/api
ENABLE_SWAGGER=false
SWAGGER_USER=<secure-username>
SWAGGER_PASSWORD=<strong-password>
SWAGGER_BASE_PATH=/docs
JWT_SECRET=<strong-random-jwt-secret>
JWT_DURATION_MINUTES=1440
JWT_ISSUER=whatsapp-gateway-prod
WHATSAPP_DATASTORE_TYPE=postgres
WHATSAPP_DATASTORE_URI=postgresql://user:pass@db-host:5432/whatsapp?sslmode=require
WHATSAPP_DEVICE_LABEL="Production WhatsApp Gateway"
WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY=<secure-random-hex-32-chars>
```

## Next Steps

- [Development Guide](Development-Guide.md) - Learn how to run the gateway
- [Gateway Usage Flow](Gateway-Usage-Flow.md) - Understand how to use the API
- [Security Considerations](Security-Considerations.md) - Important security information

---

[← Back to Home](Home.md)
