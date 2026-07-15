# Storage Configuration Guide

## Overview

Whatsapp Gateway supports two production-ready storage backends for managing file uploads and media:

- **Local Filesystem**: Direct disk storage with full control over data
- **S3/S3-Compatible**: Cloud object storage with CDN and distributed capabilities

Both backends implement the same API - you can switch between them by changing a single configuration value.

## Choosing a Storage Provider

### Local Filesystem

**Best for:**
- Self-hosted deployments with persistent volumes
- Organizations requiring full data sovereignty
- Simple infrastructure with minimal external dependencies
- Low-latency local access

**Trade-offs:**
- Limited horizontal scaling without distributed file system
- Requires manual backup/replication strategy
- File access requires direct filesystem access or web server configuration

**Production Setup:**

For Docker deployments, use a bind mount or named volume:

```yaml
# docker-compose.yml
volumes:
  - ./storage:/var/lib/whatsapp-gateway/storage
```

For systemd or native deployments, ensure the storage path has appropriate permissions:

```bash
sudo mkdir -p /var/lib/whatsapp-gateway/storage
sudo chown whatsapp-gateway:whatsapp-gateway /var/lib/whatsapp-gateway/storage
sudo chmod 750 /var/lib/whatsapp-gateway/storage
```

For public file access, you can use either:

**Option 1: Direct Gateway Serving**

Configure the gateway to serve files directly:

```bash
STORAGE_API_PATH=/storage
```

Files will be accessible at: `http://your-gateway:3000/storage/path/to/file`

**Option 2: Reverse Proxy Serving (Nginx example)**

```nginx
location /storage/ {
    alias /var/lib/whatsapp-gateway/storage/;
    autoindex off;
}
```

### S3/S3-Compatible Storage

**Best for:**
- Distributed deployments across multiple instances
- CDN integration for global content delivery
- Automatic backup and versioning
- Easy horizontal scaling

**Trade-offs:**
- External dependency (network latency, service availability)
- Potential egress/bandwidth costs
- Less direct control over data locality

**Supported Services:**

- AWS S3
- MinIO (self-hosted S3-compatible)
- DigitalOcean Spaces
- Wasabi
- Any S3-compatible service

**AWS S3 Configuration:**

```bash
STORAGE_PROVIDER=s3
STORAGE_S3_ENDPOINT=s3.amazonaws.com
STORAGE_S3_ACCESS_KEY_ID=your_access_key
STORAGE_S3_SECRET_ACCESS_KEY=your_secret_key
STORAGE_S3_REGION=us-east-1
STORAGE_S3_BUCKET=whatsapp-gateway
STORAGE_S3_USE_SSL=true
STORAGE_API_PATH=/storage
```

**MinIO Configuration:**

```bash
STORAGE_PROVIDER=s3
STORAGE_S3_ENDPOINT=localhost:9000
STORAGE_S3_ACCESS_KEY_ID=minioadmin
STORAGE_S3_SECRET_ACCESS_KEY=minioadmin
STORAGE_S3_REGION=us-east-1
STORAGE_S3_BUCKET=whatsapp-gateway
STORAGE_S3_USE_SSL=false
STORAGE_API_PATH=/storage
```

**DigitalOcean Spaces Configuration:**

```bash
STORAGE_PROVIDER=s3
STORAGE_S3_ENDPOINT=nyc3.digitaloceanspaces.com
STORAGE_S3_ACCESS_KEY_ID=your_spaces_key
STORAGE_S3_SECRET_ACCESS_KEY=your_spaces_secret
STORAGE_S3_REGION=us-east-1
STORAGE_S3_BUCKET=whatsapp-gateway
STORAGE_S3_USE_SSL=true
STORAGE_API_PATH=/storage
```

## Storage Operations

The storage client supports the following operations:

- **UploadFile**: Upload files with specified content type
- **GetFile**: Read file content with metadata (used for direct file serving)
- **GetPublicURL**: Get publicly accessible URL (for public files)
- **GetPresignedURL**: Get time-limited signed URL (for private files)
- **CreateFolder**: Create folders/prefixes for organization
- **SetFolderAccess**: Control access at folder/prefix level
- **DeleteFile**: Remove files from storage
- **ListFiles**: Enumerate files in a folder

### Direct File Serving

When `STORAGE_API_PATH` is configured (default: `/storage`), the gateway serves files directly via HTTP:

**Features:**
- Proper Content-Type headers based on file extension
- Cache-Control headers for efficient client caching
- Content-Disposition for inline display
- Last-Modified and ETag headers for conditional requests
- Accept-Ranges header advertised (files are streamed in full; byte-range/partial-content requests are not yet served)

**Example Usage:**

```
# Upload a file
# (No public upload endpoint: storage is populated automatically
# when inbound webhook media is downloaded)

# Access the file
GET /storage/uploads/photo.jpg

# Response includes proper headers:
# Content-Type: image/jpeg
# Cache-Control: public, max-age=31536000
# Last-Modified: Mon, 24 Feb 2026 12:00:00 GMT
# ETag: "1234567890"
```

## Access Control

### Local Filesystem

Access control is managed via operating system file permissions. The gateway process needs write access to the storage directory.

For public file access via web:
1. Configure reverse proxy to serve files
2. Set appropriate directory permissions
3. Use `.htaccess` or web server rules for finer control

### S3 Storage

Per-folder access control is implemented via bucket policies:
- Public folders: Objects accessible via public URLs
- Private folders: Access only via presigned URLs or authenticated requests

**Example: Setting up a public folder:**

The gateway will automatically configure bucket policies when `SetFolderAccess` is called for a prefix.

### S3 Storage Access

S3-stored media files are accessed via presigned URLs. URLs expire after `STORAGE_S3_PRESIGNED_URL_EXPIRY_SECONDS`.

**How It Works:**

1. Gateway uploads files to S3
2. Gateway generates presigned URLs with expiration time
3. URLs are valid for the configured duration (default: 1 day)
4. After expiration, URLs become invalid

**Configuration:**

```bash
# Presigned URL expiration time in seconds
STORAGE_S3_PRESIGNED_URL_EXPIRY_SECONDS=86400  # 1 day
```

**Security Benefits:**

- URLs expire automatically
- No permanent public access
- No bucket policy or ACL configuration needed
- Works with all S3-compatible services

## Security Considerations

### Local Filesystem

- Store files outside web root if not using reverse proxy serving
- Use proper file permissions (750 for directories, 640 for files)
- Implement filesystem-level backups
- Monitor disk usage

### S3 Storage

- Use IAM roles when possible (avoid hardcoded credentials)
- Enable bucket encryption
- Configure bucket policies carefully (don't make everything public)
- Use presigned URLs with appropriate expiry times
- Enable S3 versioning for critical files
- Monitor storage costs

## Troubleshooting

### Local Filesystem Issues

**Permission Denied:**
```bash
# Check permissions
ls -la /var/lib/whatsapp-gateway/storage

# Fix permissions
sudo chown -R user:group /var/lib/whatsapp-gateway/storage
sudo chmod -R 750 /var/lib/whatsapp-gateway/storage
```

**Disk Full:**
```bash
# Check disk usage
df -h /var/lib/whatsapp-gateway/storage

# Clean up old files
find /var/lib/whatsapp-gateway/storage -type f -mtime +30 -delete
```

### S3 Storage Issues

**Connection Timeout:**
- Verify endpoint URL
- Check network connectivity
- Verify firewall rules allow outbound HTTPS (or HTTP if SSL disabled)

**Access Denied:**
- Verify access key and secret key
- Check IAM permissions or bucket policy
- Ensure bucket exists and region is correct

**Presigned URL Fails:**
- Check that system time is synchronized
- Verify URL hasn't expired
- Ensure bucket policy allows signed URL access

## Migration

### Migrating Between Providers

Since both providers implement the same interface, you can:

1. Export files from current provider
2. Configure new provider
3. Import files to new provider
4. Update configuration

**Example: Local to S3**

```bash
# Sync local files to S3 using awscli
aws s3 sync /var/lib/whatsapp-gateway/storage s3://whatsapp-gateway/ --delete

# Update configuration
STORAGE_PROVIDER=s3
STORAGE_S3_ENDPOINT=s3.amazonaws.com
...
```

## Monitoring

Monitor storage usage regardless of provider:

- **Local**: Disk usage metrics
- **S3**: Bucket size metrics (available in AWS CloudWatch, MinIO console, etc.)

Configure alerts for:
- Disk usage > 80% (local)
- Storage cost thresholds (S3)
- Failed upload operations

## Lifecycle and Auto-Delete

### Local Storage Auto-Delete

Local storage supports automatic cleanup of expired files. When enabled, a background goroutine periodically scans the storage directory and deletes files older than the retention period.

**Configuration:**

```bash
STORAGE_AUTO_DELETE_ENABLED=true
STORAGE_AUTO_DELETE_INTERVAL_HOURS=24
WEBHOOK_MEDIA_RETENTION_DAYS=30
```

**How it works:**
1. Background goroutine starts when storage is initialized
2. Every `STORAGE_AUTO_DELETE_INTERVAL_HOURS`, scans the storage directory
3. Files with modification time older than `WEBHOOK_MEDIA_RETENTION_DAYS` are deleted
4. Empty directories are cleaned up automatically
5. Cleanup activity is logged with trace IDs for correlation

**Important Notes:**
- Set `WEBHOOK_MEDIA_RETENTION_DAYS=0` to disable auto-delete
- The goroutine respects the `stopCleanup` channel for graceful shutdown
- Hidden files (starting with `.`) are never deleted
- Directories are skipped; only files are evaluated for deletion

### S3 Storage Auto-Delete

S3 storage uses native S3 lifecycle policies for automatic expiration. This is more efficient than client-side deletion and works automatically even when the gateway is offline.

**Configuration:**

```bash
STORAGE_AUTO_DELETE_ENABLED=true
WEBHOOK_MEDIA_RETENTION_DAYS=30
```

**How it works:**
1. When storage is initialized and auto-delete is enabled, a lifecycle policy is set on the bucket
2. S3 automatically expires objects matching the prefix after `WEBHOOK_MEDIA_RETENTION_DAYS`
3. No background goroutines needed - S3 handles expiration internally
4. The lifecycle policy applies only to the `webhook/media/` prefix

**Lifecycle Policy Example:**

```xml
<LifecycleConfiguration>
  <Rule>
    <ID>AutoDeleteMedia</ID>
    <Prefix>webhook/media/</Prefix>
    <Status>Enabled</Status>
    <Expiration>
      <Days>30</Days>
    </Expiration>
  </Rule>
</LifecycleConfiguration>
```

**Important Notes:**
- AWS S3 and compatible services (MinIO, DigitalOcean Spaces) support lifecycle policies
- Changes to lifecycle policy may take up to 24 hours to take effect in AWS
- Expired objects incur storage charges until they are deleted
- For MinIO, lifecycle policies may need to be enabled in the server configuration

### Media Retention Best Practices

**Short-term storage (1-7 days):**
- Good for ephemeral content that's consumed immediately
- Reduces storage costs
- Suitable for chatbots and auto-replies

**Medium-term storage (14-30 days):**
- Good for content that might need to be re-accessed
- Balances cost and accessibility
- Suitable for most applications

**Long-term storage (90+ days):**
- Required for compliance or archival purposes
- Consider using cheaper storage tiers (e.g., S3 Glacier)
- May require additional backup strategies

### Monitoring Auto-Delete

Monitor these metrics for auto-delete health:

- **Local**: Log messages from cleanup goroutine
- **S3**: CloudWatch metrics for object count and storage size
- **Alerting**: Set alerts if storage size grows unexpectedly

**Example Log Output:**

```
INFO cleanup-1234567890 Starting expired files cleanup
DEBUG cleanup-1234567890 Deleted expired file: /var/lib/whatsapp-gateway/storage/webhook/media/phone123/2024/01/old_file.jpg
INFO cleanup-1234567890 Cleanup completed: 5 files deleted
```

### Disabling Auto-Delete

To disable auto-delete completely:

```bash
# Option 1: Disable auto-delete (S3 only, local storage ignores this flag; use Option 2)
STORAGE_AUTO_DELETE_ENABLED=false

# Option 2: Set retention to 0 (files never expire)
WEBHOOK_MEDIA_RETENTION_DAYS=0
```

**Manual cleanup can still be performed via:**
- Storage management interface (S3 console)
- File system access (local storage)
- Direct API calls to `DeleteFile` method
