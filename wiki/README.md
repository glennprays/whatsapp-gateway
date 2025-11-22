# Wiki Documentation

This directory contains the complete wiki documentation for the WhatsApp Gateway project.

## Wiki Pages

1. **Home.md** - Main landing page with navigation
2. **Development-Guide.md** - Instructions for running in development mode and Docker
3. **Environment-Variables.md** - Complete reference for all configuration options
4. **Gateway-Usage-Flow.md** - Step-by-step guide for using the gateway API
5. **Security-Considerations.md** - Critical security warnings and best practices

## Publishing to GitHub Wiki

GitHub wikis are managed as a separate Git repository. To publish these pages to the GitHub wiki:

### Option 1: Clone Wiki Repository and Copy Files

```bash
# Clone the wiki repository
git clone https://github.com/glennprays/whatsapp-gateway.wiki.git

# Copy all markdown files from the wiki directory
cp wiki/*.md whatsapp-gateway.wiki/

# Commit and push to wiki
cd whatsapp-gateway.wiki
git add .
git commit -m "Add comprehensive wiki documentation"
git push origin master
```

### Option 2: Manually Create Pages on GitHub

1. Go to https://github.com/glennprays/whatsapp-gateway/wiki
2. Click "Create the first page" or "New Page"
3. For each file in the `wiki/` directory:
   - Create a new page with the filename (without .md extension)
   - Copy and paste the content from the markdown file
   - Save the page

### Page Creation Order

Create pages in this order to ensure internal links work:

1. Home (use content from `Home.md` - this becomes the wiki homepage)
2. Development-Guide (use content from `Development-Guide.md`)
3. Environment-Variables (use content from `Environment-Variables.md`)
4. Gateway-Usage-Flow (use content from `Gateway-Usage-Flow.md`)
5. Security-Considerations (use content from `Security-Considerations.md`)

### Page Titles on GitHub Wiki

When creating pages on GitHub wiki, use these exact titles:

- `Home` (this is the wiki homepage)
- `Development Guide`
- `Environment Variables`
- `Gateway Usage Flow`
- `Security Considerations`

**Note:** GitHub wiki automatically converts spaces to hyphens in URLs, so:
- "Development Guide" becomes accessible at `.../wiki/Development-Guide`
- "Environment Variables" becomes accessible at `.../wiki/Environment-Variables`
- etc.

## Content Overview

### Home Page (Home.md)
- Welcome message and overview
- Navigation to all wiki pages
- Quick links to resources
- Key features summary

### Development Guide (Development-Guide.md)
- Prerequisites and installation
- Running in development mode (`go run`, `make run`)
- Docker build instructions
- Development tips (hot reloading, database management)
- Troubleshooting common issues

### Environment Variables (Environment-Variables.md)
Complete documentation for all `.env` variables:
- Server configuration (PORT, BASIC_AUTH_SECRET_KEY)
- HTTP configuration (BASE_PATH - dynamic)
- Swagger configuration (ENABLE_SWAGGER, SWAGGER_USER, SWAGGER_PASSWORD, SWAGGER_BASE_PATH - dynamic, requires authentication)
- JWT configuration (JWT_SECRET, JWT_DURATION_MINUTES, JWT_ISSUER - standard JWT)
- WhatsApp configuration:
  - WHATSAPP_DATASTORE_TYPE (sqlite/sqlite3 or postgres)
  - WHATSAPP_DATASTORE_URI (connection strings)
  - WHATSAPP_DEVICE_LABEL
  - WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY (encrypts HMAC secrets for webhook security)

### Gateway Usage Flow (Gateway-Usage-Flow.md)
Step-by-step usage guide:
1. Register via `/register` endpoint to get JWT token
2. Login using QR code or pairing code
3. Configure webhooks (optional but recommended)
   - Currently supports incoming chat events
   - Refer to Swagger docs for detailed webhook payload structure
4. Send/receive messages
5. Session management

### Security Considerations (Security-Considerations.md)
**CRITICAL WARNINGS:**
- Gateway must be wrapped by a proper backend
- Cannot be directly integrated with end users
- JWT security concern: Multiple JWT tokens for the same phone number can access the same WhatsApp session
- Backend wrapper requirements
- Security best practices
- What to do if compromised

## Internal Links

The wiki pages use relative markdown links that work within the wiki directory. When published to GitHub wiki, these links will automatically work:

- `[Development Guide](Development-Guide.md)` → Links to the Development Guide page
- `[Environment Variables](Environment-Variables.md)` → Links to the Environment Variables page
- etc.

## References

The wiki documentation references:
- `.env.example` file for configuration examples
- `docs/swagger.yaml` for API documentation details
- Dockerfile for Docker build information
- Makefile for build commands

## 🔄 Updating the Wiki

To update wiki documentation:

1. Edit the markdown files in the `wiki/` directory
2. Commit changes to the main repository
3. Re-publish to GitHub wiki using one of the methods above

## Tips

- Keep wiki pages synchronized with code changes
- Update environment variable documentation when adding new config options
- Add examples and use cases as they emerge
- Include troubleshooting tips based on user feedback
- Maintain consistent formatting across all pages

## Need Help?

If you have questions about the wiki documentation or need to make changes:

1. Review the markdown files in this directory
2. Check the GitHub wiki: https://github.com/glennprays/whatsapp-gateway/wiki
3. Open an issue on GitHub

---

**Note:** The wiki/ directory is maintained in the main repository for version control and easier collaboration. The actual GitHub wiki is a separate repository that needs to be updated separately.
