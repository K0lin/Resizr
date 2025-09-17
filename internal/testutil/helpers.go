package testutil

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"resizr/internal/config"
	"resizr/internal/models"

	"github.com/gin-gonic/gin"
)

// SetupTestGin creates a test Gin engine with test mode
func SetupTestGin() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

// CreateTestRequest creates a test HTTP request
func CreateTestRequest(method, path string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, path, body)
	return req
}

// CreateJSONRequest creates a test HTTP request with JSON body
func CreateJSONRequest(method, path string, data interface{}) *http.Request {
	jsonData, _ := json.Marshal(data)
	req := httptest.NewRequest(method, path, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// CreateMultipartRequest creates a multipart form request
func CreateMultipartRequest(method, path string, formData map[string]string, fileField, filename string, fileContent []byte) *http.Request {
	var body bytes.Buffer
	boundary := "test-boundary"

	// Add form fields
	for key, value := range formData {
		body.WriteString("--" + boundary + "\r\n")
		body.WriteString("Content-Disposition: form-data; name=\"" + key + "\"\r\n\r\n")
		body.WriteString(value + "\r\n")
	}

	// Add file
	if fileField != "" && filename != "" {
		body.WriteString("--" + boundary + "\r\n")
		body.WriteString("Content-Disposition: form-data; name=\"" + fileField + "\"; filename=\"" + filename + "\"\r\n")
		body.WriteString("Content-Type: application/octet-stream\r\n\r\n")
		body.Write(fileContent)
		body.WriteString("\r\n")
	}

	body.WriteString("--" + boundary + "--\r\n")

	req := httptest.NewRequest(method, path, &body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	return req
}

// ParseJSONResponse parses JSON response into a map
func ParseJSONResponse(resp *httptest.ResponseRecorder, target interface{}) error {
	return json.Unmarshal(resp.Body.Bytes(), target)
}

// AssertJSONResponse asserts that the response contains expected JSON
func AssertJSONResponse(resp *httptest.ResponseRecorder, expected interface{}) bool {
	var actual interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &actual); err != nil {
		return false
	}

	expectedJSON, _ := json.Marshal(expected)
	actualJSON, _ := json.Marshal(actual)

	return string(expectedJSON) == string(actualJSON)
}

// TestConfig returns a test configuration
func TestConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Port:    "8080",
			GinMode: "test",
		},
		Redis: config.RedisConfig{
			URL:      "redis://localhost:6379",
			Password: "",
			DB:       0,
			PoolSize: 10,
			Timeout:  5000,
		},
		Cache: config.CacheConfig{
			Type:      "redis",
			Directory: "/tmp/test",
		},
		S3: config.S3Config{
			Endpoint:  "http://localhost:9000",
			AccessKey: "test",
			SecretKey: "test",
			Bucket:    "test-bucket",
			Region:    "us-east-1",
			UseSSL:    false,
		},
		Image: config.ImageConfig{
			MaxFileSize:                10485760, // 10MB
			Quality:                    85,
			GenerateDefaultResolutions: true,
			ResizeMode:                 "smart_fit",
			MaxWidth:                   4096,
			MaxHeight:                  4096,
		},
		RateLimit: config.RateLimitConfig{
			Upload:   10,
			Download: 100,
			Info:     50,
		},
		CORS: config.CORSConfig{
			Enabled:          true,
			AllowAllOrigins:  true,
			AllowedOrigins:   []string{"*"},
			AllowCredentials: false,
		},
		Logger: config.LoggerConfig{
			Level:  "debug",
			Format: "console",
		},
	}
}

// CreateTestImageMetadata creates test image metadata
func CreateTestImageMetadata() *models.ImageMetadata {
	return &models.ImageMetadata{
		ID:          "f47ac10b-58cc-4372-a567-0e02b2c3d479",
		OriginalKey: "images/f47ac10b-58cc-4372-a567-0e02b2c3d479/original.jpg",
		Filename:    "test.jpg",
		MimeType:    "image/jpeg",
		Size:        102400, // 100KB
		Width:       1920,
		Height:      1080,
		Resolutions: []string{"thumbnail", "800x600"},
		CreatedAt:   time.Now().Add(-time.Hour),
		UpdatedAt:   time.Now(),
	}
}

// CreateTestImageData creates test image data (minimal JPEG header)
func CreateTestImageData() []byte {
	// Minimal JPEG file header for testing
	return []byte{
		0xFF, 0xD8, 0xFF, 0xE0, // JPEG SOI marker
		0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01, // JFIF header
		0x01, 0x01, 0x00, 0x48, 0x00, 0x48, 0x00, 0x00,
		0xFF, 0xD9, // JPEG EOI marker
	}
}

// CreateLargeTestImageData creates test image data that exceeds size limits
func CreateLargeTestImageData(size int) []byte {
	data := make([]byte, size)
	// Add JPEG header
	copy(data[:2], []byte{0xFF, 0xD8})
	// Add JPEG end marker
	copy(data[size-2:], []byte{0xFF, 0xD9})
	return data
}

// MockReadCloser implements io.ReadCloser for testing
type MockReadCloser struct {
	*bytes.Reader
}

func (m *MockReadCloser) Close() error {
	return nil
}

// NewMockReadCloser creates a new MockReadCloser
func NewMockReadCloser(data []byte) *MockReadCloser {
	return &MockReadCloser{bytes.NewReader(data)}
}

// ErrorReader implements io.Reader that always returns an error
type ErrorReader struct{}

func (e ErrorReader) Read(p []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}

// ErrorReadCloser implements io.ReadCloser that always returns an error
type ErrorReadCloser struct{}

func (e ErrorReadCloser) Read(p []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}

func (e ErrorReadCloser) Close() error {
	return nil
}

// SetupTestContext creates a test Gin context with request ID
func SetupTestContext(req *http.Request) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Set("request_id", "test-request-id")
	return c, w
}

// Contains checks if a string slice contains a specific string
func Contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// AssertError checks if error matches expected type and message
func AssertError(err error, expectedType string, expectedMessage string) bool {
	if err == nil {
		return expectedType == "" && expectedMessage == ""
	}

	errStr := err.Error()
	return strings.Contains(errStr, expectedMessage)
}

// ValidUUID is a valid UUID for testing
const ValidUUID = "f47ac10b-58cc-4372-a567-0e02b2c3d479"

// InvalidUUID is an invalid UUID for testing
const InvalidUUID = "invalid-uuid"

// TestUploadRequest creates a test upload request
func TestUploadRequest() models.UploadRequest {
	return models.UploadRequest{
		Resolutions: []string{"800x600", "1200x900"},
	}
}

// TestHealthResponse creates a test health response
func TestHealthResponse() models.HealthResponse {
	return models.HealthResponse{
		Status: "healthy",
		Services: map[string]string{
			"redis":       "connected",
			"s3":          "connected",
			"application": "healthy",
		},
		Timestamp: time.Now(),
	}
}
