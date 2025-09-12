# 🤝 Contributing to RESIZR

Thank you for your interest in contributing to RESIZR! This guide will help you understand our development process and how to contribute effectively to the project.

---

## 📋 Table of Contents

1. [Code of Conduct](#code-of-conduct)
2. [Getting Started](#getting-started)
3. [Development Setup](#development-setup)
4. [Contributing Workflow](#contributing-workflow)
5. [Coding Standards](#coding-standards)
6. [Testing Guidelines](#testing-guidelines)
7. [Documentation](#documentation)
8. [Pull Request Process](#pull-request-process)
9. [Issue Guidelines](#issue-guidelines)
10. [Community](#community)

---

## 📜 Code of Conduct

By participating in this project, you agree to abide by our Code of Conduct. We are committed to providing a welcoming and inclusive environment for all contributors.

### Our Standards

**Positive behaviors include:**
- Using welcoming and inclusive language
- Being respectful of differing viewpoints and experiences
- Gracefully accepting constructive criticism
- Focusing on what is best for the community
- Showing empathy towards other community members

**Unacceptable behaviors include:**
- Use of sexualized language or imagery
- Trolling, insulting/derogatory comments, and personal attacks
- Public or private harassment
- Publishing others' private information without explicit permission
- Other conduct which could reasonably be considered inappropriate

### Enforcement

Instances of abusive, harassing, or otherwise unacceptable behavior may be reported by contacting the project team at conduct@resizr.dev. All complaints will be reviewed and investigated promptly and fairly.

---

## 🚀 Getting Started

### Prerequisites

Before contributing, ensure you have:

- **Go 1.25+** installed ([Download Go](https://golang.org/dl/))
- **Git** for version control
- **Docker** for local development environment
- **Redis** and **MinIO** (via Docker Compose)
- Basic understanding of Go, HTTP APIs, and containerization

### Areas for Contribution

We welcome contributions in several areas:

- 🐛 **Bug Fixes**: Help us identify and fix issues
- ✨ **Features**: Add new functionality or enhance existing features
- 📚 **Documentation**: Improve or add documentation
- 🔧 **Performance**: Optimize code and improve efficiency
- 🧪 **Testing**: Add or improve test coverage
- 🎨 **UI/UX**: Enhance user experience and interfaces
- 🔒 **Security**: Identify and fix security vulnerabilities

---

## 🛠️ Development Setup

### 1. Fork and Clone

```bash
# Fork the repository on GitHub
# Clone your fork
git clone https://github.com/k0lin/resizr.git
cd resizr

# Add upstream remote
git remote add upstream https://github.com/original-org/resizr.git
```

### 2. Install Dependencies

```bash
# Install Go dependencies
go mod download

# Install development tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/cosmtrek/air@latest
go install golang.org/x/tools/cmd/goimports@latest
```

### 3. Environment Setup

```bash
# Copy environment template
cp env.example .env

# Start dependencies with Docker Compose
docker-compose up -d redis minio

# Verify services are running
docker-compose ps
```

### 4. Run the Application

```bash
# Run with hot reload
air

# Or run directly
go run cmd/server/main.go
```

### 5. Verify Setup

```bash
# Test health endpoint
curl http://localhost:8080/health

# Expected response:
# {"status":"healthy","services":{"redis":"connected","s3":"connected"},...}
```

---

## 🔄 Contributing Workflow

### 1. Issue First

- Check existing issues before starting work
- Create a new issue if one doesn't exist
- Discuss your approach with maintainers
- Get approval for significant changes

### 2. Branch Creation

```bash
# Update your local main branch
git checkout main
git pull upstream main

# Create a feature branch
git checkout -b feature/your-feature-name

# Or for bug fixes
git checkout -b fix/issue-description
```

### 3. Development

- Write clean, well-documented code
- Follow our coding standards
- Add tests for new functionality
- Update documentation as needed

### 4. Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run integration tests
go test -tags=integration ./test/...

# Lint your code
golangci-lint run
```

### 5. Commit and Push

```bash
# Add your changes
git add .

# Commit with a descriptive message
git commit -m "feat: add image rotation functionality

- Add rotation support in processor service
- Update API endpoints for rotation parameter
- Add tests for rotation functionality
- Update documentation

Closes #123"

# Push to your fork
git push origin feature/your-feature-name
```

### 6. Pull Request

- Open a pull request against the main branch
- Fill out the PR template completely
- Link related issues
- Wait for review and address feedback

---

## 📏 Coding Standards

### Go Style Guide

We follow the official Go style guide and additional conventions:

#### Formatting

```bash
# Format your code
go fmt ./...

# Organize imports
goimports -w .

# Lint your code
golangci-lint run
```

#### Naming Conventions

```go
// ✅ Good: Exported functions use PascalCase
func ProcessImage(data []byte) error

// ✅ Good: Unexported functions use camelCase
func validateImageFormat(data []byte) error

// ✅ Good: Constants use descriptive names
const MaxImageWidth = 4096  // default, configurable via IMAGE_MAX_WIDTH
const MaxImageHeight = 4096 // default, configurable via IMAGE_MAX_HEIGHT

// ✅ Good: Interfaces end with -er when appropriate
type ImageProcessor interface {
    Process(data []byte) error
}

// ❌ Avoid: Unclear abbreviations
func ProcImg(d []byte) error // Too abbreviated

// ❌ Avoid: Stuttering
type ImageImageService struct{} // Redundant
```

#### Error Handling

```go
// ✅ Good: Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to process image: %w", err)
}

// ✅ Good: Use custom error types
return models.ValidationError{
    Field:   "resolution",
    Message: "invalid format",
}

// ❌ Avoid: Ignoring errors
result, _ := processImage(data) // Silent failure
```

#### Logging

```go
// ✅ Good: Structured logging
logger.InfoWithContext(ctx, "Processing image",
    zap.String("image_id", imageID),
    zap.Duration("duration", elapsed))

// ✅ Good: Log levels
logger.Debug("Detailed debug information")
logger.Info("General information")
logger.Warn("Warning condition")
logger.Error("Error occurred", zap.Error(err))

// ❌ Avoid: String formatting in logs
logger.Info(fmt.Sprintf("Processing %s", imageID))
```

### Project Structure

Follow our established project structure:

```
internal/           # Private application code
├── api/           # HTTP layer (handlers, middleware, routes)
├── service/       # Business logic layer
├── repository/    # Data access layer
├── storage/       # Storage abstraction layer
├── config/        # Configuration management
└── models/        # Data models and types

pkg/               # Public packages
├── logger/        # Logging utilities
└── utils/         # Shared utilities

cmd/               # Application entry points
└── server/        # HTTP server

docs/              # Documentation
test/              # Test files
```

### Code Organization

```go
// ✅ Good: Group imports logically
import (
    // Standard library
    "context"
    "fmt"
    "time"

    // Third-party packages
    "github.com/gin-gonic/gin"
    "go.uber.org/zap"

    // Local packages
    "resizr/internal/models"
    "resizr/internal/service"
)

// ✅ Good: Organize struct fields logically
type ImageService struct {
    // Dependencies first
    repository ImageRepository
    storage    ImageStorage
    processor  ProcessorService
    
    // Configuration
    config *config.Config
    
    // State/metrics
    metrics *ServiceMetrics
}

// ✅ Good: Group related functions
func (s *ImageService) Upload() error { ... }
func (s *ImageService) Download() error { ... }
func (s *ImageService) Delete() error { ... }

// Health-related methods grouped together
func (s *ImageService) Health() error { ... }
func (s *ImageService) Status() string { ... }
```

---

## 🧪 Testing Guidelines

### Test Structure

We use a comprehensive testing strategy:

```go
// Unit test example
func TestImageService_Upload(t *testing.T) {
    // Arrange
    mockRepo := &MockImageRepository{}
    mockStorage := &MockImageStorage{}
    service := NewImageService(mockRepo, mockStorage, nil, testConfig)
    
    // Act
    result, err := service.Upload(ctx, testData, "test.jpg", 1024, nil)
    
    // Assert
    assert.NoError(t, err)
    assert.NotNil(t, result)
    assert.Equal(t, "test.jpg", result.Filename)
}
```

### Test Categories

#### 1. Unit Tests
```bash
# Run unit tests
go test ./internal/service/
go test ./internal/repository/
```

**Requirements:**
- Test all public functions
- Use mocks for dependencies
- Cover happy path and error cases
- Fast execution (<100ms per test)

#### 2. Integration Tests
```bash
# Run integration tests
go test -tags=integration ./test/integration/
```

**Requirements:**
- Test component interactions
- Use real dependencies (Redis, MinIO via testcontainers)
- Test actual file operations
- Slower execution acceptable

#### 3. API Tests
```bash
# Run API tests
go test ./internal/api/handlers/
```

**Requirements:**
- Test HTTP endpoints
- Validate request/response formats
- Test middleware behavior
- Use Gin test context

### Testing Best Practices

```go
// ✅ Good: Descriptive test names
func TestImageService_Upload_WhenFileExceedsLimit_ReturnsValidationError(t *testing.T)

// ✅ Good: Table-driven tests for multiple scenarios
func TestParseResolution(t *testing.T) {
    testCases := []struct {
        name     string
        input    string
        expected ResolutionConfig
        wantErr  bool
    }{
        {
            name:     "valid custom resolution",
            input:    "800x600",
            expected: ResolutionConfig{Width: 800, Height: 600},
            wantErr:  false,
        },
        {
            name:     "invalid format",
            input:    "invalid",
            expected: ResolutionConfig{},
            wantErr:  true,
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            result, err := ParseResolution(tc.input)
            
            if tc.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tc.expected, result)
            }
        })
    }
}

// ✅ Good: Test helpers for common setup
func setupTestService(t *testing.T) (*ImageService, *MockRepository, *MockStorage) {
    mockRepo := &MockRepository{}
    mockStorage := &MockStorage{}
    config := &config.Config{/* test config */}
    
    service := NewImageService(mockRepo, mockStorage, nil, config)
    return service, mockRepo, mockStorage
}
```

### Mock Generation

```go
//go:generate mockgen -source=interfaces.go -destination=mocks/mock_interfaces.go

// Use generated mocks in tests
func TestImageService_Upload(t *testing.T) {
    ctrl := gomock.NewController(t)
    defer ctrl.Finish()
    
    mockRepo := mocks.NewMockImageRepository(ctrl)
    mockRepo.EXPECT().Store(gomock.Any(), gomock.Any()).Return(nil)
    
    // ... test implementation
}
```

### Test Coverage

We aim for:
- **Service Layer**: >90% coverage
- **Repository Layer**: >85% coverage
- **Handler Layer**: >80% coverage
- **Overall Project**: >80% coverage

```bash
# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

---

## 📚 Documentation

### Code Documentation

```go
// ✅ Good: Package documentation
// Package service provides business logic for image operations.
// It orchestrates between repository, storage, and processing layers
// to handle image upload, download, and metadata management.
package service

// ✅ Good: Function documentation
// Upload processes and stores a new image with optional resolutions.
// It validates the input, generates a unique ID, processes requested
// resolutions, stores files in S3, and saves metadata to Redis.
//
// Parameters:
//   - ctx: Context for cancellation and tracing
//   - file: Image file reader
//   - filename: Original filename
//   - size: File size in bytes
//   - resolutions: Optional custom resolutions to generate
//
// Returns:
//   - *models.UploadResponse: Upload result with image ID and resolutions
//   - error: Any error that occurred during processing
func (s *ImageService) Upload(ctx context.Context, file io.Reader, filename string, size int64, resolutions []string) (*models.UploadResponse, error)

// ✅ Good: Type documentation
// ImageMetadata represents image metadata stored in Redis.
// It contains all information needed to locate and serve images,
// including available resolutions and processing history.
type ImageMetadata struct {
    ID          string    `json:"id"`          // Unique UUID identifier
    Filename    string    `json:"filename"`    // Original filename
    // ... other fields with comments
}
```

### README Updates

When adding features, update:
- Feature list
- API endpoint documentation
- Configuration options
- Environment variables
- Usage examples

### API Documentation

Update the OpenAPI specification (`openapi.yaml`) for any API changes:

```yaml
# Add new endpoints
paths:
  /api/v1/images/{id}/rotate:
    post:
      summary: Rotate image
      description: Rotate an image by specified degrees
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
            format: uuid
        - name: degrees
          in: query
          required: true
          schema:
            type: integer
            minimum: 0
            maximum: 360
```

---

## 🔍 Pull Request Process

### PR Template

When creating a pull request, use our template:

```markdown
## Description
Brief description of changes and motivation.

## Type of Change
- [ ] Bug fix (non-breaking change which fixes an issue)
- [ ] New feature (non-breaking change which adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] Documentation update

## Related Issues
Closes #123
References #456

## Testing
- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] Manual testing completed
- [ ] All tests pass locally

## Documentation
- [ ] Code comments updated
- [ ] README updated (if applicable)
- [ ] API documentation updated (if applicable)

## Checklist
- [ ] Code follows the style guidelines
- [ ] Self-review of the code completed
- [ ] Changes generate no new warnings
- [ ] New and existing unit tests pass locally
- [ ] Any dependent changes merged
```

### Review Process

1. **Automated Checks**: All CI checks must pass
2. **Code Review**: At least one maintainer review required
3. **Testing**: Verify tests pass and coverage is adequate
4. **Documentation**: Ensure documentation is updated
5. **Approval**: Maintainer approval required for merge

### Review Criteria

Reviewers will check:
- Code quality and adherence to standards
- Test coverage and quality
- Documentation completeness
- Security implications
- Performance impact
- Backward compatibility

### Addressing Review Feedback

```bash
# Make requested changes
git add .
git commit -m "address review feedback: improve error handling"

# Force push if you need to amend previous commits
git push origin feature/your-feature-name --force-with-lease
```

---

## 🐛 Issue Guidelines

### Bug Reports

When reporting bugs, include:

```markdown
**Bug Description**
Clear description of what the bug is.

**To Reproduce**
Steps to reproduce the behavior:
1. Go to '...'
2. Click on '....'
3. Scroll down to '....'
4. See error

**Expected Behavior**
What you expected to happen.

**Screenshots**
If applicable, add screenshots.

**Environment:**
- OS: [e.g. Ubuntu 20.04]
- Go Version: [e.g. 1.25.1]
- Docker Version: [e.g. 20.10.8]
- RESIZR Version: [e.g. 0.0.1]

**Additional Context**
Any other context about the problem.

**Logs**
```
Paste relevant logs here
```
```

### Feature Requests

When requesting features, include:

```markdown
**Feature Description**
Clear description of the feature you'd like to see.

**Problem Statement**
What problem would this solve?

**Proposed Solution**
Describe your proposed solution.

**Alternatives Considered**
Alternative solutions you've considered.

**Additional Context**
Any other context, mockups, or examples.
```

### Issue Labels

We use the following labels:
- `bug`: Something isn't working
- `enhancement`: New feature or request
- `documentation`: Improvements or additions to documentation
- `good first issue`: Good for newcomers
- `help wanted`: Extra attention is needed
- `performance`: Performance related
- `security`: Security related
- `api`: API related changes
- `breaking change`: Breaking change

---

## 🌟 Community

### Communication Channels

- **GitHub Issues**: Bug reports and feature requests

### Recognition

We recognize contributors through:
- **Contributors file**: All contributors are listed
- **Release notes**: Significant contributions are highlighted
- **Social media**: Outstanding contributions are shared

### Mentorship

New contributors can:
- Start with `good first issue` labeled issues
- Ask for help in issue comments
- Request code review guidance
- Participate in discussions

---

## 🎯 Development Guidelines

### Performance Considerations

- **Memory Usage**: Be mindful of memory allocation
- **Goroutine Management**: Avoid goroutine leaks
- **Connection Pooling**: Reuse connections efficiently
- **Caching**: Implement appropriate caching strategies

```go
// ✅ Good: Resource cleanup
func (s *Service) processImage(ctx context.Context) error {
    buffer := s.getBuffer()
    defer s.putBuffer(buffer) // Always clean up
    
    // Process image...
}

// ✅ Good: Context handling
func (s *Service) longRunningOperation(ctx context.Context) error {
    select {
    case <-ctx.Done():
        return ctx.Err() // Respect cancellation
    default:
        // Continue processing
    }
}
```

### Security Guidelines

- **Input Validation**: Always validate user input
- **Error Messages**: Don't expose internal details
- **Logging**: Don't log sensitive information
- **Dependencies**: Keep dependencies updated

```go
// ✅ Good: Input validation
func validateImageID(id string) error {
    if !isValidUUID(id) {
        return models.ValidationError{
            Field:   "id",
            Message: "invalid UUID format",
        }
    }
    return nil
}

// ✅ Good: Safe error handling
func (h *Handler) handleError(c *gin.Context, err error) {
    logger.Error("Internal error", zap.Error(err))
    
    // Don't expose internal error details
    c.JSON(500, models.ErrorResponse{
        Error:   "Internal server error",
        Message: "An unexpected error occurred",
        Code:    500,
    })
}
```

---

## 🏁 Final Notes

### Getting Help

If you need help:
1. Check existing documentation
2. Search existing issues
3. Ask in GitHub Discussions
4. Create a detailed issue

### Thank You

Thank you for contributing to RESIZR! Your contributions help make this project better for everyone. Whether you're fixing a bug, adding a feature, or improving documentation, every contribution is valuable and appreciated.

### License

By contributing to RESIZR, you agree that your contributions will be licensed under the same license as the project (MIT License).

---

<div align="center">

**Happy Contributing! 🎉**

[⬆ Back to Top](#-contributing-to-resizr)

</div>
