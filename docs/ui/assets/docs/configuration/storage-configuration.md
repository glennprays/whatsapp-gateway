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

For public file access, configure your reverse proxy (Nginx example):

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
```

## Storage Operations

The storage client supports the following operations (available via API endpoints in future releases):

- **UploadFile**: Upload files with specified content type
- **GetPublicURL**: Get publicly accessible URL (for public files)
- **GetPresignedURL**: Get time-limited signed URL (for private files)
- **CreateFolder**: Create folders/prefixes for organization
- **SetFolderAccess**: Control access at folder/prefix level
- **DeleteFile**: Remove files from storage
- **ListFiles**: Enumerate files in a folder

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
