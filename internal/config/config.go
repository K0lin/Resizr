package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all application configuration
type Config struct {
	Server    ServerConfig
	Redis     RedisConfig
	Cache     CacheConfig
	S3        S3Config
	Image     ImageConfig
	RateLimit RateLimitConfig
	Logger    LoggerConfig
	CORS      CORSConfig
	Canvas    CanvasConfig
	Health    HealthConfig
	Auth      AuthConfig
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port    string
	GinMode string
}

// RedisConfig holds Redis database configuration
type RedisConfig struct {
	URL      string
	Password string
	DB       int
	PoolSize int
	Timeout  time.Duration
}

// S3Config holds S3 storage configuration
type S3Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
	Region    string
	UseSSL    bool
	URLExpire time.Duration
}

// ImageConfig holds image processing configuration
type ImageConfig struct {
	MaxFileSize                int64
	Quality                    int
	CacheTTL                   time.Duration
	GenerateDefaultResolutions bool
	ResizeMode                 string
	SupportedFormats           []string
	DefaultResolutions         map[string]ResolutionConfig
	MaxWidth                   int
	MaxHeight                  int
}

// ResolutionConfig defines image resolution parameters
type ResolutionConfig struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Upload   int // requests per minute
	Download int // requests per minute
	Info     int // requests per minute
}

// LoggerConfig holds logging configuration
type LoggerConfig struct {
	Level  string // "debug", "info", "warn", "error"
	Format string // "json", "console"
}

// CacheConfig holds cache configuration
// Supports two backend types:
// - "redis": Uses Redis for both metadata and caching (requires Redis server)
// - "badger": Uses BadgerDB for both metadata and caching (embedded, no external dependencies)
type CacheConfig struct {
	Type      string        // Cache type: "redis" or "badger"
	Directory string        // Directory for BadgerDB files (only used when type=badger)
	TTL       time.Duration // Default TTL for cache entries
}

// CORSConfig holds CORS configuration
type CORSConfig struct {
	Enabled          bool     // Enable/disable CORS
	AllowAllOrigins  bool     // Allow all origins (*)
	AllowedOrigins   []string // List of allowed origins
	AllowCredentials bool     // Allow credentials in CORS requests
}

// CanvasConfig holds canvas configuration
type CanvasConfig struct {
	BackgroundColor string
}

// HealthConfig holds health check configuration
type HealthConfig struct {
	S3ChecksDisabled bool          // Disable S3 health checks to reduce API calls
	S3ChecksInterval time.Duration // Interval for caching S3 health check results
	CheckInterval    time.Duration // Docker health check interval (minimum 10s)
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Enabled       bool     // Enable/disable authentication
	ReadWriteKeys []string // API keys with read-write permissions
	ReadOnlyKeys  []string // API keys with read-only permissions
	KeyHeader     string   // HTTP header name for API key
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Load .env file if it exists (for development)
	_ = godotenv.Load()

	config := &Config{
		Server: ServerConfig{
			Port:    getEnv("PORT", "8080"),
			GinMode: getEnv("GIN_MODE", "release"),
		},
		Redis: RedisConfig{
			URL:      getEnv("REDIS_URL", "redis://localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
			PoolSize: getEnvInt("REDIS_POOL_SIZE", 10),
			Timeout:  time.Duration(getEnvInt("REDIS_TIMEOUT", 5)) * time.Second,
		},
		Cache: CacheConfig{
			Type:      getEnv("CACHE_TYPE", "redis"),
			Directory: getEnv("CACHE_DIRECTORY", "./data/cache"),
			TTL:       time.Duration(getEnvInt("CACHE_TTL", 3600)) * time.Second,
		},
		S3: S3Config{
			Endpoint:  getEnv("S3_ENDPOINT", "https://s3.amazonaws.com"),
			AccessKey: getEnv("S3_ACCESS_KEY", ""),
			SecretKey: getEnv("S3_SECRET_KEY", ""),
			Bucket:    getEnv("S3_BUCKET", ""),
			Region:    getEnv("S3_REGION", "us-east-1"),
			UseSSL:    getEnvBool("S3_USE_SSL", true),
			URLExpire: time.Duration(getEnvInt("S3_URL_EXPIRE", 3600)) * time.Second,
		},
		Image: ImageConfig{
			MaxFileSize:                int64(getEnvInt("MAX_FILE_SIZE", 10485760)), // 10MB default
			Quality:                    getEnvInt("IMAGE_QUALITY", 85),
			CacheTTL:                   time.Duration(getEnvInt("CACHE_TTL", 3600)) * time.Second,
			GenerateDefaultResolutions: getEnvBool("GENERATE_DEFAULT_RESOLUTIONS", true),
			ResizeMode:                 getEnv("RESIZE_MODE", "smart_fit"),
			SupportedFormats:           []string{"image/jpeg", "image/png", "image/gif", "image/webp"},
			DefaultResolutions: map[string]ResolutionConfig{
				"thumbnail": {Width: 150, Height: 150},
			},
			MaxWidth:  getEnvInt("IMAGE_MAX_WIDTH", 4096),
			MaxHeight: getEnvInt("IMAGE_MAX_HEIGHT", 4096),
		},
		RateLimit: RateLimitConfig{
			Upload:   getEnvInt("RATE_LIMIT_UPLOAD", 10),
			Download: getEnvInt("RATE_LIMIT_DOWNLOAD", 100),
			Info:     getEnvInt("RATE_LIMIT_INFO", 50),
		},
		Logger: LoggerConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
		},
		CORS: CORSConfig{
			Enabled:          getEnvBool("CORS_ENABLED", true),
			AllowAllOrigins:  getEnvBool("CORS_ALLOW_ALL_ORIGINS", false),
			AllowedOrigins:   getEnvStringSlice("CORS_ALLOWED_ORIGINS", []string{"*"}),
			AllowCredentials: getEnvBool("CORS_ALLOW_CREDENTIALS", false),
		},
		Canvas: CanvasConfig{
			BackgroundColor: getEnv("BACKGROUND_COLOR", "#000000"),
		},
		Health: HealthConfig{
			S3ChecksDisabled: getEnvBool("S3_HEALTHCHECKS_DISABLE", false),
			S3ChecksInterval: getS3HealthCheckInterval(),
			CheckInterval:    getHealthCheckInterval(),
		},
		Auth: AuthConfig{
			Enabled:       getEnvBool("AUTH_ENABLED", false),
			ReadWriteKeys: getEnvStringSlice("AUTH_READWRITE_KEYS", []string{}),
			ReadOnlyKeys:  getEnvStringSlice("AUTH_READONLY_KEYS", []string{}),
			KeyHeader:     getEnv("AUTH_KEY_HEADER", "X-API-Key"),
		},
	}

	// Validate required configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate S3 configuration
	if c.S3.Bucket == "" {
		return fmt.Errorf("S3_BUCKET is required")
	}
	if c.S3.AccessKey == "" {
		return fmt.Errorf("S3_ACCESS_KEY is required")
	}
	if c.S3.SecretKey == "" {
		return fmt.Errorf("S3_SECRET_KEY is required")
	}

	// Validate server configuration
	if c.Server.Port == "" {
		return fmt.Errorf("PORT cannot be empty")
	}

	// Validate cache configuration
	validCacheTypes := []string{"redis", "badger"}
	if !contains(validCacheTypes, c.Cache.Type) {
		return fmt.Errorf("CACHE_TYPE must be one of: %s", strings.Join(validCacheTypes, ", "))
	}

	// Validate Redis configuration (only if using Redis cache)
	if c.Cache.Type == "redis" {
		if c.Redis.URL == "" {
			return fmt.Errorf("REDIS_URL is required when CACHE_TYPE=redis")
		}
	}

	// Validate BadgerDB configuration (only if using BadgerDB cache)
	if c.Cache.Type == "badger" && c.Cache.Directory == "" {
		return fmt.Errorf("CACHE_DIRECTORY is required when CACHE_TYPE=badger")
	}

	// Validate image configuration
	if c.Image.MaxFileSize <= 0 {
		return fmt.Errorf("MAX_FILE_SIZE must be positive")
	}
	if c.Image.Quality < 1 || c.Image.Quality > 100 {
		return fmt.Errorf("IMAGE_QUALITY must be between 1 and 100")
	}

	// Validate rate limit configuration
	if c.RateLimit.Upload <= 0 || c.RateLimit.Download <= 0 || c.RateLimit.Info <= 0 {
		return fmt.Errorf("rate limits must be positive integers")
	}

	// Validate resize mode configuration
	validResizeModes := []string{"smart_fit", "crop", "stretch"}
	if !contains(validResizeModes, c.Image.ResizeMode) {
		return fmt.Errorf("RESIZE_MODE must be one of: %s", strings.Join(validResizeModes, ", "))
	}

	// Validate logger configuration
	validLogLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLogLevels, c.Logger.Level) {
		return fmt.Errorf("LOG_LEVEL must be one of: %s", strings.Join(validLogLevels, ", "))
	}

	validLogFormats := []string{"json", "console"}
	if !contains(validLogFormats, c.Logger.Format) {
		return fmt.Errorf("LOG_FORMAT must be one of: %s", strings.Join(validLogFormats, ", "))
	}

	// Validate image max dimensions (must be positive)
	if c.Image.MaxWidth <= 0 {
		return fmt.Errorf("IMAGE_MAX_WIDTH must be a positive integer")
	}
	if c.Image.MaxHeight <= 0 {
		return fmt.Errorf("IMAGE_MAX_HEIGHT must be a positive integer")
	}

	return nil
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return c.Server.GinMode == "debug" || c.Logger.Format == "console"
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	return c.Server.GinMode == "release" && c.Logger.Format == "json"
}

// GetResolution returns resolution config by name
func (c *Config) GetResolution(name string) (ResolutionConfig, bool) {
	resolution, exists := c.Image.DefaultResolutions[name]
	return resolution, exists
}

// IsSupportedFormat checks if the MIME type is supported
func (c *Config) IsSupportedFormat(mimeType string) bool {
	return contains(c.Image.SupportedFormats, mimeType)
}

// Helper functions for environment variable parsing

// getEnv returns environment variable value or default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt returns environment variable as integer or default
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvBool returns environment variable as boolean or default
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

// getEnvFloat returns environment variable as float64 or default
func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

// getEnvDuration returns environment variable as duration or default
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// getEnvStringSlice returns environment variable as string slice or default
func getEnvStringSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		// Split by comma and trim spaces
		parts := strings.Split(value, ",")
		result := make([]string, 0, len(parts))
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultValue
}

// getHealthCheckInterval returns health check interval with minimum 10s limit
func getHealthCheckInterval() time.Duration {
	interval := getEnvInt("HEALTHCHECK_INTERVAL", 30)
	if interval < 10 {
		interval = 10 // Minimum 10 seconds for health check interval
	}
	return time.Duration(interval) * time.Second
}

// getS3HealthCheckInterval returns S3 health check interval with minimum 10s limit
func getS3HealthCheckInterval() time.Duration {
	interval := getEnvInt("S3_HEALTHCHECKS_INTERVAL", 30)
	if interval < 10 {
		interval = 10 // Minimum 10 seconds for S3 health check interval
	}
	return time.Duration(interval) * time.Second
}

// contains checks if slice contains value
func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}
