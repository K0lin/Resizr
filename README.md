# 🖼️ RESIZR - High-Performance Image Processing Service

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=for-the-badge&logo=go)](https://golang.org/)
[![Redis](https://img.shields.io/badge/Redis-6.0+-DC382D?style=for-the-badge&logo=redis)](https://redis.io/)
[![AWS S3](https://img.shields.io/badge/AWS%20S3-Compatible-FF9900?style=for-the-badge&logo=amazon-aws)](https://aws.amazon.com/s3/)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=for-the-badge&logo=docker)](https://docker.com/)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

**RESIZR** is a high-performance, production-ready Go microservice for image upload, processing, and delivery. Built with modern cloud-native patterns, it provides RESTful APIs for image management with automatic multi-resolution processing, Redis-based metadata storage, and S3-compatible file storage.

---

## 🚀 Features

- **Multi-Resolution Processing**: Automatic thumbnail and custom resolution generation
- **High-Performance**: Streaming uploads/downloads, connection pooling, optimized processing
- **Production Ready**: Rate limiting, health checks, structured logging, error handling
- **Cloud Native**: Containerization with Docker and Kubernetes support
- **Flexible Storage**: AWS S3, MinIO, and S3-compatible backends
- **Smart Caching**: Multi-level caching with Redis and pre-signed URLs

---

## 🎯 Quick Start

### Prerequisites

- **Go 1.25+** - [Download Go](https://golang.org/dl/)
- **S3-Compatible Storage** - AWS S3 or MinIO for image storage
- **Cache Backend** (choose one):
  - **Redis 6.0+** - When using `CACHE_TYPE=redis` (default)
  - **File system** - When using `CACHE_TYPE=badger` (no external dependencies)
- **Docker** (optional) - For containerized deployment

### 1. Installation

```bash
# Clone the repository
git clone https://github.com/k0lin/resizr.git
cd resizr

# Install dependencies
go mod tidy
```

### 2. Configuration

Create a `.env` file from the example:

```bash
cp env.example .env
```

Configure your environment variables:

```env
# Server Configuration
PORT=8080                    # HTTP server port
GIN_MODE=release             # Gin framework mode (debug/release/test)

# Logging Configuration  
LOG_LEVEL=info               # Log level (debug/info/warn/error)
LOG_FORMAT=json              # Log format (json/console)

# Cache Configuration
CACHE_TYPE=redis                    # Cache backend: redis or badger
CACHE_DIRECTORY=./data/cache        # Directory for BadgerDB (only used when CACHE_TYPE=badger)
CACHE_TTL=3600                      # Default cache TTL in seconds

# Redis Configuration (only required when CACHE_TYPE=redis)
REDIS_URL=redis://localhost:6379  # Redis connection URL
REDIS_PASSWORD=              # Redis password (leave empty if no auth)
REDIS_DB=0                   # Redis database number (0-15)
REDIS_POOL_SIZE=10           # Connection pool size for Redis
REDIS_TIMEOUT=5              # Connection timeout in seconds

# S3 Storage Configuration
S3_ENDPOINT=https://s3.amazonaws.com  # S3 endpoint URL
S3_ACCESS_KEY=your_access_key         # S3 access key ID
S3_SECRET_KEY=your_secret_key         # S3 secret access key
S3_BUCKET=your_bucket_name            # S3 bucket name for image storage
S3_REGION=us-east-1                   # AWS region
S3_USE_SSL=true                       # Use SSL for S3 connections
S3_URL_EXPIRE=3600                    # Pre-signed URL expiration in seconds

# Image Processing Configuration
MAX_FILE_SIZE=10485760        # Maximum upload file size in bytes (10MB)
IMAGE_QUALITY=85              # JPEG compression quality (1-100, higher = better)
GENERATE_DEFAULT_RESOLUTIONS=true # Auto-generate thumbnail resolution
RESIZE_MODE=smart_fit        # Image resize algorithm (smart_fit, crop, stretch)
IMAGE_MAX_WIDTH=4096         # Maximum allowed width for requested/custom resolutions (up to 8192)
IMAGE_MAX_HEIGHT=4096        # Maximum allowed height for requested/custom resolutions (up to 8192)

# Rate Limiting Configuration (requests per minute)
RATE_LIMIT_UPLOAD=10         # Upload endpoint rate limit per IP
RATE_LIMIT_DOWNLOAD=100      # Download endpoint rate limit per IP  
RATE_LIMIT_INFO=50           # Info endpoint rate limit per IP

# Health Check Configuration
S3_HEALTHCHECKS_DISABLE=false # Disable S3 health checks to reduce API calls (default: false)
S3_HEALTHCHECKS_INTERVAL=30    # Interval between S3 health checks in seconds (default: 30s, minimum: 10s)
HEALTHCHECK_INTERVAL=30        # Docker health check interval in seconds (minimum: 10s)

# CORS Configuration
CORS_ENABLED=true            # Enable/disable CORS middleware entirely
CORS_ALLOW_ALL_ORIGINS=false # Allow all origins (*) - use with caution
CORS_ALLOWED_ORIGINS=https://domain.com,https://example.com
CORS_ALLOW_CREDENTIALS=false # Allow credentials in CORS requests

# Authentication Configuration
AUTH_ENABLED=false           # Enable/disable API key authentication (default: false)
AUTH_KEY_HEADER=X-API-Key    # HTTP header name for API key (default: X-API-Key)
AUTH_READWRITE_KEYS=rw_key_1,rw_key_2  # Comma-separated list of read-write API keys
AUTH_READONLY_KEYS=ro_key_1,ro_key_2   # Comma-separated list of read-only API keys
```

**Note on Resolution Processing:**
- When `GENERATE_DEFAULT_RESOLUTIONS=true` (default), the service automatically creates thumbnail (150x150) version of every uploaded image
- When set to `false`, only custom resolutions specified in the upload request will be generated
- This allows for more control over storage usage and processing time in scenarios where default resolutions aren't needed

**Maximum dimensions:**
Maximum dimensions for requested custom resolutions are controlled by `IMAGE_MAX_WIDTH` and `IMAGE_MAX_HEIGHT` (defaults: 4096x4096). Requests exceeding these limits are rejected during validation and processing. For safety, the service also enforces a hard upper bound of 8192 per side.

**Cache Type Options:**
- `redis` (default): Uses Redis for both metadata storage and caching. Requires Redis server.
- `badger`: Uses BadgerDB for both metadata storage and caching. No external dependencies, stores data in local files.


**Resize Mode Options:**
- `smart_fit` (default): Maintains aspect ratio, fits image within dimensions with padding if needed
- `crop`: Crops image to exact dimensions while maintaining aspect ratio
- `stretch`: Stretches image to exact dimensions, may distort aspect ratio

### 3. Development Setup

#### Option A: Docker Compose (Recommended)

```bash
# Start all services (app, Redis, MinIO)
docker-compose up -d

# View logs
docker-compose logs -f
```

#### Option B: Local Development

```bash
# Start Redis (using Docker)
docker run -d --name redis -p 6379:6379 redis:7-alpine

# Start MinIO (using Docker)
docker run -d --name minio \
  -p 9000:9000 -p 9001:9001 \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin \
  minio/minio server /data --console-address ":9001"

# Run the application
go run cmd/server/main.go
```

### 4. Test the API

```bash
# Health check
curl http://localhost:8080/health

# Upload an image with custom resolutions
curl -X POST http://localhost:8080/api/v1/images \
  -F "image=@test.jpg" \
  -F "resolutions=800x600,1200x900"

# Upload an image with aliased resolutions
curl -X POST http://localhost:8080/api/v1/images \
  -F "image=@test.jpg" \
  -F "resolutions=800x600:small,1200x900:medium,1920x1080:large"

# Get image info (replace {id} with actual image ID)
curl http://localhost:8080/api/v1/images/{id}/info

# Download thumbnail
curl http://localhost:8080/api/v1/images/{id}/thumbnail -o thumbnail.jpg

# Download by dimensions
curl http://localhost:8080/api/v1/images/{id}/800x600 -o image_800x600.jpg

# Download by alias
curl http://localhost:8080/api/v1/images/{id}/small -o image_small.jpg

# Delete entire image (with deduplication cleanup)
curl -X DELETE http://localhost:8080/api/v1/images/{id}

# Delete specific resolution (with reference tracking)
curl -X DELETE http://localhost:8080/api/v1/images/{id}/800x600

# Delete resolution by alias
curl -X DELETE http://localhost:8080/api/v1/images/{id}/small

---

## 🌐 API Documentation

### Base URL
```
https://your-domain.com/api/v1
```

### Endpoints Overview

| Method | Endpoint | Description | Rate Limit |
|--------|----------|-------------|------------|
| `POST` | `/images` | Upload image with optional resolutions | 10/min |
| `GET` | `/images/{id}/info` | Get image metadata | 50/min |
| `GET` | `/images/{id}/original` | Download original image | 100/min |
| `GET` | `/images/{id}/thumbnail` | Download thumbnail (150x150) | 100/min |
| `GET` | `/images/{id}/{resolution}` | Download custom resolution or alias | 100/min |
| `GET` | `/images/{id}/{resolution}/presigned-url` | Generate presigned URL for direct access | 50/min |
| `DELETE` | `/images/{id}` | Delete entire image with deduplication cleanup | 10/min |
| `DELETE` | `/images/{id}/{resolution}` | Delete specific resolution with reference tracking | 10/min |
| `GET` | `/health` | Health check with deduplication metrics | Unlimited |

### 🏷️ Resolution Aliases

RESIZR supports **resolution aliases** for easier API usage and better readability. You can assign custom names to resolutions during upload, then access images using either the dimensions or the alias.

#### How It Works

**During Upload:**
```bash
# Upload with aliased resolutions
curl -X POST http://localhost:8080/api/v1/images \
  -F "image=@photo.jpg" \
  -F "resolutions=100x100:thumb,800x600:small,1920x1080:large"
```

**During Download - Both Methods Work:**
```bash
# Access by alias (user-friendly)
curl http://localhost:8080/api/v1/images/{id}/thumb -o thumbnail.jpg
curl http://localhost:8080/api/v1/images/{id}/small -o small.jpg

# Access by dimensions (backward compatible)
curl http://localhost:8080/api/v1/images/{id}/100x100 -o thumbnail.jpg
curl http://localhost:8080/api/v1/images/{id}/800x600 -o small.jpg
```

#### Alias Format
- **Upload Format**: `WIDTHxHEIGHT:alias` (e.g., `800x600:small`)
- **Alias Rules**:
  - Alphanumeric characters, underscores, and hyphens only
  - 1-50 characters long
  - Case-sensitive

#### Benefits
- **User-Friendly URLs**: `/images/{id}/small` instead of `/images/{id}/800x600`
- **Storage Efficient**: No duplicate files - aliases map to same physical file
- **Future-Proof**: Change dimensions without breaking client code
- **Backward Compatible**: Existing dimension-based URLs continue to work
- **Flexible**: Mix aliased and non-aliased resolutions in the same image

#### Example Response
```json
{
  "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "message": "Image uploaded successfully",
  "resolutions": ["original", "thumbnail", "100x100:thumb", "800x600:small", "1920x1080:large"]
}
```

### � Deduplication & Resolution Tracking

RESIZR includes advanced **deduplication** functionality that optimizes storage by sharing identical image resolutions across multiple users while maintaining proper reference tracking per user.

#### How Deduplication Works

**Smart Resolution Sharing:**
- When multiple users upload images with identical content and request the same resolutions, RESIZR automatically detects duplicates
- Instead of storing multiple copies of the same processed image, it stores one physical file and tracks references per user
- Each user maintains their own metadata and access permissions, but shares the underlying storage

**Resolution Reference Tracking:**
- Each resolution tracks how many users are referencing it
- When a user deletes their image, only their reference is removed
- The physical file is only deleted when the last user reference is removed
- Prevents accidental deletion of shared resolutions used by other users

#### Deduplication Process

**Upload Flow with Deduplication:**
```
1. User uploads image with requested resolutions
2. System calculates content hash of original image
3. For each requested resolution:
   ├── Check if identical resolution already exists
   ├── If exists: Create reference to existing file
   ├── If not: Process and store new resolution
4. Store user-specific metadata with deduplication info
5. Track resolution references per user
```

**Deletion Flow with Reference Tracking:**
```
1. User requests image deletion
2. System decrements reference count for each resolution
3. Remove user's metadata entry
4. Delete physical files only when reference count reaches zero
5. Clean up orphaned metadata and cache entries
```

#### Benefits

- **Storage Efficiency**: Eliminates duplicate storage of identical image resolutions
- **Cost Optimization**: Reduces S3 storage costs and data transfer fees
- **Performance**: Faster uploads for duplicate content (no re-processing needed)
- **User Isolation**: Each user maintains independent access to their images
- **Data Integrity**: Proper cleanup prevents orphaned files and metadata

#### Configuration

Deduplication is **automatically enabled** and requires no additional configuration. The system works transparently with existing functionality.

#### API Behavior

**Upload (Automatic Deduplication):**
```bash
# First user uploads image
curl -X POST http://localhost:8080/api/v1/images \
  -F "image=@photo.jpg" \
  -F "resolutions=800x600,1200x900"

# Second user uploads identical image with same resolutions
curl -X POST http://localhost:8080/api/v1/images \
  -F "image=@photo.jpg" \
  -F "resolutions=800x600,1200x900"

# → Shares existing resolutions, no duplicate storage
```

**Delete (Reference Tracking):**

```bash
# User deletes their image
curl -X DELETE http://localhost:8080/api/v1/images/{id}

# → Only removes user's reference
# → Physical files remain if other users reference them
# → Files deleted only when last reference removed
```

#### Backward Compatibility

- **Existing Images**: All existing images continue to work without modification
- **API Compatibility**: No changes to existing API endpoints or request formats
- **Migration**: Automatic migration of existing data to deduplication system
- **Data Preservation**: All existing metadata and files are preserved

#### Monitoring Deduplication

Check deduplication status through the health endpoint:

```bash
curl http://localhost:8080/health
```

Response includes deduplication metrics:

```json
{
  "status": "healthy",
  "deduplication": {
    "enabled": true,
    "total_shared_resolutions": 1250,
    "storage_saved_mb": 450.5,
    "average_references_per_resolution": 2.3
  }
}
```

#### Storage Schema with Deduplication

**Redis Keys (Enhanced):**

```bash
image:metadata:{uuid}           # Hash: Image metadata + deduplication info
resolution:refs:{hash}:{res}    # Set: User UUIDs referencing this resolution
resolution:data:{hash}:{res}    # Hash: Resolution metadata (dimensions, format, etc.)
```

**S3 Structure (Optimized):**

```bash
s3://bucket/
└── images/
    ├── shared/                  # Shared resolution storage
    │   ├── abc123_800x600.jpg   # Shared resolution file
    │   └── def456_1200x900.jpg  # Another shared resolution
    └── users/                   # User-specific original images
        └── {user_id}/
            └── {uuid}/
                └── original.jpg
```

**Metadata Structure:**

```json
{
  "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "hash": "abc123def456",
  "isDeduped": true,
  "sharedImageId": "shared_abc123",
  "resolutions": ["original", "thumbnail", "800x600", "1200x900"],
  "deduplicationInfo": {
    "sharedResolutions": ["800x600", "1200x900"],
    "referenceCount": 3
  }
}
```

### �🔐 Authentication

RESIZR supports optional API key-based authentication with two permission levels:

- **Read-Write Keys**: Can upload images and access all read operations
- **Read-Only Keys**: Can only access download and info operations

#### Authentication Endpoints (No Auth Required)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/auth/generate-key` | Generate new API key |
| `GET` | `/auth/status` | Get authentication status |

#### Enabling Authentication

```bash
# Enable authentication
AUTH_ENABLED=true

# Set API keys (comma-separated)
AUTH_READWRITE_KEYS=your_rw_key_1,your_rw_key_2
AUTH_READONLY_KEYS=your_ro_key_1,your_ro_key_2

# Configure header name (optional, default: X-API-Key)
AUTH_KEY_HEADER=X-API-Key
```

#### Using API Keys

When authentication is enabled, include your API key in requests:

```bash
# Upload image (requires read-write key)
curl -X POST http://localhost:8080/api/v1/images \
  -H "X-API-Key: your_readwrite_api_key" \
  -F "image=@test.jpg"

# Download image (works with both read-write and read-only keys)
curl -H "X-API-Key: your_api_key" \
  http://localhost:8080/api/v1/images/{id}/thumbnail
```

#### Generate API Keys

```bash
# Generate an API key (works regardless of auth enabled/disabled)
curl "http://localhost:8080/api/v1/auth/generate-key"
```

**Note**: Generated API keys must be manually added to your environment configuration to be active.

See [OpenAPI Specification](openapi.yaml) for full API documentation.

---

## ⚙️ Configuration

RESIZR uses environment variables for configuration.

### Core Settings
- `PORT`: Server port (default: 8080)
- `LOG_LEVEL`: Logging level (debug/info/warn/error)
- `CACHE_TYPE`: Cache backend (redis/badger)

### Storage
- `S3_ENDPOINT`: S3 endpoint URL
- `S3_ACCESS_KEY`: Access key
- `S3_SECRET_KEY`: Secret key
- `S3_BUCKET`: Bucket name

### Processing
- `MAX_FILE_SIZE`: Max upload size (bytes)
- `IMAGE_QUALITY`: JPEG quality (1-100)
- `RESIZE_MODE`: smart_fit/crop/stretch

### Health Check Configuration
- `S3_HEALTHCHECKS_DISABLE`: Disable S3 health checks to reduce API calls (default: false)
- `S3_HEALTHCHECKS_INTERVAL`: Interval between S3 health checks in seconds (default: 30s, minimum: 10s)
- `HEALTHCHECK_INTERVAL`: Docker health check interval in seconds (minimum: 10s)

### Limits
- `RATE_LIMIT_UPLOAD`: Upload rate limit per IP
- `RATE_LIMIT_DOWNLOAD`: Download rate limit per IP
- `RATE_LIMIT_INFO`: Info rate limit per IP

---

## 🏥 Health Check Optimization

RESIZR includes advanced health check configuration to optimize production deployments and reduce cloud costs.

### Smart Health Check Features

- **Configurable S3 Health Checks**: Reduce expensive S3 API calls by disabling or adjusting health check frequency
- **Intelligent Caching**: Health check results are cached to prevent redundant API calls
- **Minimum Interval Protection**: Enforces a 10-second minimum interval to prevent excessive checking
- **Docker Integration**: Smart health check script that respects configuration settings

### Configuration Options

#### S3_HEALTHCHECKS_DISABLE
```env
S3_HEALTHCHECKS_DISABLE=false  # Default: false
```
- `false`: Health checks include S3 connectivity validation
- `true`: Skip S3 health checks entirely, reducing S3 API costs

#### S3_HEALTHCHECKS_INTERVAL
```env
S3_HEALTHCHECKS_INTERVAL=30  # Default: 30 seconds, minimum: 10 seconds
```
Controls how frequently S3 health checks are performed when enabled:
- Values below 10 seconds are automatically adjusted to 10 seconds
- Higher values reduce S3 API calls but may delay detection of S3 issues
- Recommended: 30-60 seconds for production environments

#### HEALTHCHECK_INTERVAL
```env
HEALTHCHECK_INTERVAL=30  # Default: 30 seconds, minimum: 10 seconds
```
Docker-specific health check interval:
- Used by the Docker health check script
- Overrides Docker Compose/Dockerfile interval settings
- Values below 10 seconds are automatically adjusted to 10 seconds

### Cost Optimization Benefits

**Without Optimization (default Docker health check every 30s):**
- S3 API calls: ~2,880 per day per container
- Estimated cost: $0.01-0.02 per day for S3 requests (varies by region)

**With Optimization (S3_HEALTHCHECKS_INTERVAL=300, S3 checks every 5 minutes):**
- S3 API calls: ~288 per day per container
- Cost reduction: 90% fewer S3 API calls
- Maintains service health monitoring with Redis/application checks every 30s

**Production Recommendation:**
```env
# For cost-sensitive production environments
S3_HEALTHCHECKS_DISABLE=false
S3_HEALTHCHECKS_INTERVAL=300        # Check S3 every 5 minutes
HEALTHCHECK_INTERVAL=30             # Check service health every 30 seconds

# For high-availability production environments
S3_HEALTHCHECKS_DISABLE=false
S3_HEALTHCHECKS_INTERVAL=60         # Check S3 every minute
HEALTHCHECK_INTERVAL=30             # Check service health every 30 seconds
```

### Docker Health Check Script

RESIZR includes a smart health check script (`healthcheck.sh`) that:
- Respects `S3_HEALTHCHECKS_DISABLE` and `S3_HEALTHCHECKS_INTERVAL` settings
- Provides detailed logging for troubleshooting
- Enforces minimum interval limits
- Falls back gracefully if configuration is missing

---

## 🐳 Deployment

### Docker

```bash
docker build -t resizr .
docker run -p 8080:8080 --env-file .env resizr
```

### Docker Compose

```yaml
services:
  resizr:
   image: k0lin/resizr:dev
   restart: unless-stopped
   ports:
     - "8080:8080"
   environment:
    - CACHE_TYPE=badger
    - CACHE_DIRECTORY=/data/badger
    - S3_ENDPOINT=
    - S3_ACCESS_KEY=
    - S3_SECRET_KEY=
    - S3_BUCKET=
    - RESIZE_MODE=smart_fit
    - RATE_LIMIT_UPLOAD=100         # Upload endpoint rate limit per IP
    - RATE_LIMIT_DOWNLOAD=100      # Download endpoint rate limit per IP
    - RATE_LIMIT_INFO=50           # Info endpoint rate limit per IP
    # Health Check Configuration for cost optimization
    - S3_HEALTHCHECKS_DISABLE=false # Disable S3 health checks to reduce API costs
    - S3_HEALTHCHECKS_INTERVAL=30   # S3 health check interval in seconds (minimum: 10s)
    - HEALTHCHECK_INTERVAL=30       # Docker health check interval in seconds (minimum: 10s)
   volumes:
    - ./badger-data:/data/badger
```

---

## 🏗️ Architecture

### System Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   HTTP Client   │────│  RESIZR Service │────│     Redis       │
│   (Frontend)    │    │   (Go Service)  │    │   (Metadata)    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                │
                                ▼
                       ┌─────────────────┐
                       │   S3 Storage    │
                       │ (Images/Files)  │
                       └─────────────────┘
```

### Component Architecture

RESIZR follows Clean Architecture principles with clear separation of concerns:

#### API Layer (`internal/api/`)
- **Handlers**: HTTP request/response processing
- **Middleware**: Cross-cutting concerns (CORS, rate limiting, logging)
- **Routes**: URL routing and endpoint definition

#### Service Layer (`internal/service/`)
- **Image Service**: Business logic for image operations
- **Health Service**: Health check and monitoring
- **Processor Service**: Image processing and transformation

#### Repository Layer (`internal/repository/`)
- **Redis Repository**: Metadata persistence and caching
- **Interfaces**: Abstraction for data access

#### Storage Layer (`internal/storage/`)
- **S3 Storage**: File storage operations
- **Interfaces**: Abstraction for file operations

#### Models (`internal/models/`)
- **Data Structures**: Image metadata, requests/responses
- **Validation**: Input validation and error handling
- **Types**: Custom types and business objects

### Data Flow

#### Upload Flow
```
1. HTTP POST /api/v1/images
   ├── Middleware: Request ID, CORS, Rate Limit, Size Check
   ├── Handler: Parse multipart, validate file
   └── Service: Process upload

2. Service Layer Processing
   ├── Validate image format and size
   ├── Generate unique UUID
   ├── Process original image
   ├── Upload to S3: images/{uuid}/original.ext
   ├── Process requested resolutions
   │   ├── Thumbnail: Smart fit to 150x150
   │   └── Custom: Parse and process "WIDTHxHEIGHT"
   ├── Upload processed images to S3
   └── Store metadata in Redis

3. Response
   └── Return UUID and processed resolutions
```

#### Download Flow
```
1. HTTP GET /api/v1/images/{id}/thumbnail
   ├── Middleware: Request ID, CORS, Rate Limit
   ├── Handler: Validate UUID, extract resolution
   └── Service: Get image stream

2. Service Layer Processing
   ├── Get metadata from Redis
   ├── Check resolution exists
   ├── Get cached pre-signed URL or generate new
   ├── Stream image from S3
   └── Return stream with proper headers

3. Response
   ├── Set headers (Content-Type, Cache-Control, ETag)
   └── Stream binary data to client
```

### Storage Schema

#### Redis Keys
```
image:metadata:{uuid}        # Hash: Image metadata
image:cache:{uuid}:{res}     # String: Pre-signed URL (TTL: 1h)
```

#### S3 Structure
```
s3://bucket/
└── images/
    └── {uuid}/
        ├── original.jpg     # Original uploaded image
        ├── thumbnail.jpg    # 150x150 thumbnail
        ├── 800x600.jpg      # Custom resolution (accessible via dimensions OR alias)
        └── 1920x1080.jpg    # Another resolution (no duplicates stored)
```

**Storage Optimization:**
- Files are stored **only once** using dimension-based names (e.g., `800x600.jpg`)
- Aliases are metadata that resolve to the same physical file
- No duplicate storage: `800x600:small` → points to `800x600.jpg`
- Both `/images/{id}/small` and `/images/{id}/800x600` access the same file

---

## 🛠️ Development

### Prerequisites

- **Go 1.25+**
- **Redis** (Docker recommended)
- **MinIO** (Docker recommended)
- **Make** (optional, for convenience commands)

### Setup Development Environment

```bash
# Clone repository
git clone https://github.com/k0lin/resizr.git
cd resizr

# Install dependencies
go mod tidy

# Copy environment configuration
cp env.example .env

# Start dependencies
docker-compose up -d redis minio

# Run the application
go run cmd/server/main.go
```

---

## 🤝 Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for detailed contribution guidelines.

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
