# Security Considerations

This page outlines critical security considerations and warnings for using the WhatsApp Gateway safely and responsibly.

## CRITICAL WARNING

**THE WHATSAPP GATEWAY MUST ALWAYS BE WRAPPED BY A PROPER BACKEND SERVICE**

**DO NOT directly integrate this gateway with end-user applications or expose it to the public internet without a protective backend layer.**

## Major Security Concern: JWT Token Vulnerability

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

## Required Protection: Backend Wrapper

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
    # Verify HMAC signature (header: X-Webhook-Signature, format: "sha256=<hex>")
    signature = request.headers.get('X-Webhook-Signature')
    if not verify_hmac(request.body, signature, WEBHOOK_SECRET):
        raise Unauthorized("Invalid signature")
    
    # Parse the event
    event = parse_webhook_event(request.body)
    
    # Route to appropriate user based on phone number
    user = get_user_by_phone(event['phone_number'])
    if user:
        notify_user(user, event)
```
