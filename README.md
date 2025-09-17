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

# CORS Configuration
CORS_ENABLED=true            # Enable/disable CORS middleware entirely
CORS_ALLOW_ALL_ORIGINS=false # Allow all origins (*) - use with caution
CORS_ALLOWED_ORIGINS=https://domain.com,https://example.com
CORS_ALLOW_CREDENTIALS=false # Allow credentials in CORS requests
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

# Upload an image
curl -X POST http://localhost:8080/api/v1/images \
  -F "image=@test.jpg" \
  -F "resolutions=800x600,1200x900"

# Get image info (replace {id} with actual image ID)
curl http://localhost:8080/api/v1/images/{id}/info

# Download thumbnail
curl http://localhost:8080/api/v1/images/{id}/thumbnail -o thumbnail.jpg
```

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

| `GET` | `/images/{id}/{resolution}` | Download custom resolution | 100/min |
| `GET` | `/images/{id}/{resolution}/presigned-url` | Generate presigned URL for direct access | 50/min |
| `GET` | `/health` | Health check | Unlimited |

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

### Limits
- `RATE_LIMIT_UPLOAD`: Upload rate limit per IP
- `RATE_LIMIT_DOWNLOAD`: Download rate limit per IP
- `RATE_LIMIT_INFO`: Info rate limit per IP

---

## � Deployment

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
        └── 800x600.jpg      # Custom resolution
```

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
