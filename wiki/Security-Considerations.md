# Security Considerations

This page outlines critical security considerations and warnings for using the WhatsApp Gateway safely and responsibly.

## ⚠️ CRITICAL WARNING

**THE WHATSAPP GATEWAY MUST ALWAYS BE WRAPPED BY A PROPER BACKEND SERVICE**

**DO NOT directly integrate this gateway with end-user applications or expose it to the public internet without a protective backend layer.**

## 🚨 Major Security Concern: JWT Token Vulnerability

### The Problem

There is a critical security design consideration you must understand:

**If you generate two JWT tokens for the same phone number, BOTH tokens can access the WhatsApp account once either token successfully logs in.**

### Example Scenario

```
1. User A registers phone number "6281234567890"
   → Receives JWT Token 1

2. User B registers the SAME phone number "6281234567890"
   → Receives JWT Token 2

3. User A logs into WhatsApp using Token 1
   → User A can now send/receive messages

4. ⚠️ SECURITY ISSUE: User B can ALSO access the same WhatsApp
   session using Token 2, even though User A logged in!
```

### Why This Happens

The gateway associates JWT tokens with phone numbers, not with individual sessions. Once a phone number is authenticated with WhatsApp, ANY valid JWT token for that phone number can access the WhatsApp session.

### Security Implications

This design means:
- Multiple users could potentially control the same WhatsApp account
- Unauthorized access is possible if JWT tokens are not properly managed
- No session isolation between different token holders for the same phone number

## 🛡️ Required Protection: Backend Wrapper

To mitigate these security risks, you **MUST** implement a proper backend service that:

### 1. Controls Token Generation

Your backend should:
- **Authenticate users properly** before allowing them to register
- **Implement strict authorization** to prevent unauthorized phone number registration
- **Track which users are authorized** for which phone numbers
- **Prevent duplicate registrations** for the same phone number from different users
- **Invalidate or revoke tokens** when users should no longer have access

**Example Backend Logic:**
```python
# DON'T allow anyone to register any phone number
def register_whatsapp(user_id, phone_number):
    # WRONG - No authorization check
    # return gateway.register(phone_number)
    
    # RIGHT - Verify user owns this phone number
    if not verify_user_owns_phone(user_id, phone_number):
        raise Unauthorized("You don't own this phone number")
    
    # Check if phone is already registered to another user
    if is_phone_registered_to_another_user(phone_number, user_id):
        raise Conflict("Phone number already registered")
    
    # Only then register with gateway
    token = gateway.register(phone_number)
    store_token_mapping(user_id, phone_number, token)
    return token
```

### 2. Implements Access Control

Your backend must:
- **Validate user identity** before proxying requests to the gateway
- **Ensure users can only access their own phone numbers**
- **Log all access attempts** for auditing
- **Implement rate limiting** to prevent abuse

**Example Backend Proxy:**
```python
@app.post("/send-message")
def send_message(user_id, phone_number, recipient, message):
    # Verify user is authorized for this phone number
    if not user_authorized_for_phone(user_id, phone_number):
        raise Forbidden("Not authorized for this phone number")
    
    # Get the stored JWT token for this user's phone
    jwt_token = get_user_gateway_token(user_id, phone_number)
    
    # Proxy the request to gateway
    return gateway.send_message(jwt_token, recipient, message)
```

### 3. Manages Session Lifecycle

Your backend should:
- **Track active sessions** per user
- **Implement session timeouts**
- **Force logout** when users should lose access
- **Monitor for suspicious activity** (e.g., same phone number accessed from multiple IPs)

### 4. Secures Webhooks

Your backend must:
- **Verify webhook signatures** using the HMAC secret
- **Validate the source** of webhook requests
- **Route webhook events** only to authorized users
- **Sanitize webhook data** before processing

**Example Webhook Handler:**
```python
@app.post("/webhook/whatsapp")
def handle_webhook(request):
    # Verify HMAC signature
    signature = request.headers.get('X-Signature')
    if not verify_hmac(request.body, signature, WEBHOOK_SECRET):
        raise Unauthorized("Invalid signature")
    
    # Parse the event
    event = parse_webhook_event(request.body)
    
    # Route to appropriate user based on phone number
    user = get_user_by_phone(event['phone_number'])
    if user:
        notify_user(user, event)
```

## 🔒 Additional Security Best Practices

### 1. Environment Security

- **Never commit secrets** to version control
- **Use environment variables** for all sensitive configuration
- **Rotate secrets regularly** (JWT secret, HMAC keys, etc.)
- **Use different secrets** for development and production
- **Restrict access** to the `.env` file on production servers

### 2. Network Security

- **Never expose the gateway directly** to the public internet
- **Use a reverse proxy** (Nginx, Apache) with proper SSL/TLS
- **Implement IP whitelisting** if possible
- **Use VPC or private networks** in cloud environments
- **Enable firewall rules** to restrict access

**Example Architecture:**
```
[End Users] → [Your Backend API] → [Gateway] → [WhatsApp]
              ↑
              |
          Firewall
          Authentication
          Authorization
```

### 3. Database Security

- **Encrypt the database** at rest (especially for PostgreSQL)
- **Use strong database passwords**
- **Enable SSL/TLS** for database connections
- **Restrict database access** to gateway application only
- **Regular backups** of WhatsApp session data
- **Monitor for unauthorized access**

### 4. JWT Token Security

- **Use strong JWT secrets** (minimum 32 characters, cryptographically random)
- **Set appropriate token expiration** (balance security vs. usability)
- **Implement token refresh mechanism** in your backend
- **Consider token revocation** for compromised accounts
- **Never expose JWT tokens** in URLs or logs
- **Store tokens securely** in your backend (encrypted if possible)

### 5. Webhook Security

- **Always use HMAC verification** for webhooks
- **Use HTTPS endpoints** for webhook URLs
- **Validate payload structure** before processing
- **Implement replay attack protection** (timestamp validation)
- **Rate limit webhook processing**
- **Log all webhook events** for auditing

### 6. Logging and Monitoring

Implement comprehensive logging:
- **Authentication attempts** (successful and failed)
- **API access patterns** (unusual activity detection)
- **Webhook deliveries** (success and failures)
- **Error conditions** (for debugging and security monitoring)
- **Token generation events**

**Important:** 
- Never log JWT tokens, passwords, or HMAC secrets
- Sanitize logs to remove sensitive data
- Implement log rotation and retention policies

### 7. Swagger Documentation

- **Disable Swagger in production** (`ENABLE_SWAGGER=false`)
- If enabled in production:
  - Use **strong credentials** for Swagger access
  - Change default username/password from `.env.example`
  - Consider IP whitelisting for documentation access
  - Monitor access to documentation endpoints

## 🔐 Encryption and Data Protection

### HMAC Master Key

The `WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY` is critical:
- **Protects webhook HMAC secrets** at rest in the database
- **Must be 32 hexadecimal characters** (16 bytes for AES-256)
- **Generate using**: `openssl rand -hex 16`
- **Never change** without migrating existing data (will make encrypted secrets unreadable)
- **Backup securely** before rotating

### Data at Rest

- WhatsApp session data is stored in the configured database
- Consider encrypting:
  - Database files (SQLite)
  - Database connections (PostgreSQL with SSL)
  - Filesystem where database files reside
  - Backup storage

### Data in Transit

- Always use HTTPS/TLS for:
  - API communication with the gateway
  - Webhook delivery URLs
  - Database connections (PostgreSQL)
- Configure TLS properly:
  - Use modern TLS versions (1.2+)
  - Strong cipher suites
  - Valid certificates from trusted CAs

## 🚦 Deployment Recommendations

### Development Environment

```
✅ SQLite database
✅ Swagger enabled
✅ Detailed logging
✅ Development secrets
❌ Public internet exposure
```

### Production Environment

```
✅ PostgreSQL database with SSL
✅ Strong, unique secrets
✅ Backend wrapper for access control
✅ Webhook HMAC verification
✅ Comprehensive monitoring
✅ Regular security audits
✅ Network isolation (VPC/private network)
❌ Direct public access
❌ Swagger documentation (or very restricted)
❌ Default credentials
```

## 🔍 Security Checklist

Before deploying to production, ensure:

- [ ] Gateway is behind a secure backend wrapper
- [ ] Strong, unique secrets for all configuration values
- [ ] PostgreSQL database with SSL enabled
- [ ] Webhook HMAC verification implemented
- [ ] JWT tokens managed securely in your backend
- [ ] Swagger documentation disabled or heavily restricted
- [ ] HTTPS/TLS enabled for all communication
- [ ] Network access restricted (firewall/VPC)
- [ ] Comprehensive logging and monitoring in place
- [ ] Regular backup strategy implemented
- [ ] Incident response plan documented
- [ ] Security audit completed
- [ ] Team trained on security best practices

## 🆘 What to Do If Compromised

If you suspect a security breach:

1. **Immediately rotate all secrets**:
   - JWT_SECRET
   - BASIC_AUTH_SECRET_KEY
   - WHATSAPP_WEBHOOK_HMAC_ENCRYPTION_MASTER_KEY
   - Database passwords

2. **Force logout all sessions**:
   - Call `/logout` endpoint for all registered numbers
   - Clear the database if necessary

3. **Audit access logs**:
   - Review all authentication attempts
   - Check for unusual API usage patterns
   - Examine webhook delivery logs

4. **Notify affected users**:
   - Inform users of the potential breach
   - Request they re-authenticate

5. **Review and patch**:
   - Identify the vulnerability
   - Apply security patches
   - Update security measures

## 📚 Additional Resources

- [OWASP API Security Top 10](https://owasp.org/www-project-api-security/)
- [JWT Best Practices](https://tools.ietf.org/html/rfc8725)
- [Webhook Security Best Practices](https://webhooks.fyi/security/overview)

## 📞 Reporting Security Issues

If you discover a security vulnerability in the WhatsApp Gateway:

1. **Do NOT open a public issue**
2. Contact the maintainers privately
3. Provide detailed information about the vulnerability
4. Allow time for a fix before public disclosure

---

## ⚠️ Final Warning

**This gateway is a powerful tool but must be used responsibly. Always prioritize security and never expose it directly to end users. Implement a proper backend wrapper with authentication, authorization, and access control.**

The maintainers of this gateway are not responsible for security breaches resulting from improper deployment or usage.

---

[← Back to Home](Home.md)
