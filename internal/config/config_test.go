package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoad_DefaultValues(t *testing.T) {
	// Clear environment to test defaults
	clearEnv()

	// Set required values
	os.Setenv("S3_BUCKET", "test-bucket")
	os.Setenv("S3_ACCESS_KEY", "test-key")
	os.Setenv("S3_SECRET_KEY", "test-secret")
	defer clearEnv()

	config, err := Load()

	assert.NoError(t, err)
	assert.NotNil(t, config)

	// Test default values
	assert.Equal(t, "8080", config.Server.Port)
	assert.Equal(t, "release", config.Server.GinMode)
	assert.Equal(t, "redis://localhost:6379", config.Redis.URL)
	assert.Equal(t, "", config.Redis.Password)
	assert.Equal(t, 0, config.Redis.DB)
	assert.Equal(t, 10, config.Redis.PoolSize)
	assert.Equal(t, 5*time.Second, config.Redis.Timeout)
	assert.Equal(t, "redis", config.Cache.Type)
	assert.Equal(t, "./data/cache", config.Cache.Directory)
	assert.Equal(t, 3600*time.Second, config.Cache.TTL)
	assert.Equal(t, "https://s3.amazonaws.com", config.S3.Endpoint)
	assert.Equal(t, "test-bucket", config.S3.Bucket)
	assert.Equal(t, "us-east-1", config.S3.Region)
	assert.True(t, config.S3.UseSSL)
	assert.Equal(t, 3600*time.Second, config.S3.URLExpire)
	assert.Equal(t, int64(10485760), config.Image.MaxFileSize)
	assert.Equal(t, 85, config.Image.Quality)
	assert.True(t, config.Image.GenerateDefaultResolutions)
	assert.Equal(t, "smart_fit", config.Image.ResizeMode)
	assert.Equal(t, 4096, config.Image.MaxWidth)
	assert.Equal(t, 4096, config.Image.MaxHeight)
	assert.Equal(t, 10, config.RateLimit.Upload)
	assert.Equal(t, 100, config.RateLimit.Download)
	assert.Equal(t, 50, config.RateLimit.Info)
	assert.False(t, config.Auth.Enabled)
	assert.Empty(t, config.Auth.ReadWriteKeys)
	assert.Empty(t, config.Auth.ReadOnlyKeys)
	assert.Equal(t, "X-API-Key", config.Auth.KeyHeader)
	assert.Equal(t, "info", config.Logger.Level)
	assert.Equal(t, "json", config.Logger.Format)
	assert.True(t, config.CORS.Enabled)
	assert.False(t, config.CORS.AllowAllOrigins)
	assert.Equal(t, []string{"*"}, config.CORS.AllowedOrigins)
	assert.False(t, config.CORS.AllowCredentials)
}

func TestLoad_CustomValues(t *testing.T) {
	clearEnv()

	// Set custom environment variables
	envVars := map[string]string{
		"PORT":                         "9090",
		"GIN_MODE":                     "debug",
		"REDIS_URL":                    "redis://custom:6379",
		"REDIS_PASSWORD":               "secret",
		"REDIS_DB":                     "5",
		"REDIS_POOL_SIZE":              "20",
		"REDIS_TIMEOUT":                "10",
		"CACHE_TYPE":                   "badger",
		"CACHE_DIRECTORY":              "/tmp/cache",
		"CACHE_TTL":                    "7200",
		"S3_ENDPOINT":                  "http://localhost:9000",
		"S3_ACCESS_KEY":                "custom-key",
		"S3_SECRET_KEY":                "custom-secret",
		"S3_BUCKET":                    "custom-bucket",
		"S3_REGION":                    "eu-west-1",
		"S3_USE_SSL":                   "false",
		"S3_URL_EXPIRE":                "1800",
		"MAX_FILE_SIZE":                "20971520", // 20MB
		"IMAGE_QUALITY":                "95",
		"GENERATE_DEFAULT_RESOLUTIONS": "false",
		"RESIZE_MODE":                  "crop",
		"IMAGE_MAX_WIDTH":              "8192",
		"IMAGE_MAX_HEIGHT":             "8192",
		"RATE_LIMIT_UPLOAD":            "5",
		"RATE_LIMIT_DOWNLOAD":          "200",
		"RATE_LIMIT_INFO":              "25",
		"LOG_LEVEL":                    "debug",
		"LOG_FORMAT":                   "console",
		"CORS_ENABLED":                 "false",
		"CORS_ALLOW_ALL_ORIGINS":       "true",
		"CORS_ALLOWED_ORIGINS":         "https://example.com,https://test.com",
		"CORS_ALLOW_CREDENTIALS":       "true",
	}

	for key, value := range envVars {
		os.Setenv(key, value)
	}
	defer clearEnv()

	config, err := Load()

	assert.NoError(t, err)
	assert.NotNil(t, config)

	// Verify custom values
	assert.Equal(t, "9090", config.Server.Port)
	assert.Equal(t, "debug", config.Server.GinMode)
	assert.Equal(t, "redis://custom:6379", config.Redis.URL)
	assert.Equal(t, "secret", config.Redis.Password)
	assert.Equal(t, 5, config.Redis.DB)
	assert.Equal(t, 20, config.Redis.PoolSize)
	assert.Equal(t, 10*time.Second, config.Redis.Timeout)
	assert.Equal(t, "badger", config.Cache.Type)
	assert.Equal(t, "/tmp/cache", config.Cache.Directory)
	assert.Equal(t, 7200*time.Second, config.Cache.TTL)
	assert.Equal(t, "http://localhost:9000", config.S3.Endpoint)
	assert.Equal(t, "custom-key", config.S3.AccessKey)
	assert.Equal(t, "custom-secret", config.S3.SecretKey)
	assert.Equal(t, "custom-bucket", config.S3.Bucket)
	assert.Equal(t, "eu-west-1", config.S3.Region)
	assert.False(t, config.S3.UseSSL)
	assert.Equal(t, 1800*time.Second, config.S3.URLExpire)
	assert.Equal(t, int64(20971520), config.Image.MaxFileSize)
	assert.Equal(t, 95, config.Image.Quality)
	assert.False(t, config.Image.GenerateDefaultResolutions)
	assert.Equal(t, "crop", config.Image.ResizeMode)
	assert.Equal(t, 8192, config.Image.MaxWidth)
	assert.Equal(t, 8192, config.Image.MaxHeight)
	assert.Equal(t, 5, config.RateLimit.Upload)
	assert.Equal(t, 200, config.RateLimit.Download)
	assert.Equal(t, 25, config.RateLimit.Info)
	assert.Equal(t, "debug", config.Logger.Level)
	assert.Equal(t, "console", config.Logger.Format)
	assert.False(t, config.CORS.Enabled)
	assert.True(t, config.CORS.AllowAllOrigins)
	assert.Equal(t, []string{"https://example.com", "https://test.com"}, config.CORS.AllowedOrigins)
	assert.True(t, config.CORS.AllowCredentials)
}

func TestValidate_Success(t *testing.T) {
	config := &Config{
		Server: ServerConfig{
			Port:    "8080",
			GinMode: "release",
		},
		Cache: CacheConfig{
			Type: "redis",
		},
		Redis: RedisConfig{
			URL: "redis://localhost:6379",
		},
		S3: S3Config{
			AccessKey: "key",
			SecretKey: "secret",
			Bucket:    "bucket",
		},
		Image: ImageConfig{
			MaxFileSize: 10485760,
			Quality:     85,
			ResizeMode:  "smart_fit",
			MaxWidth:    4096,
			MaxHeight:   4096,
		},
		RateLimit: RateLimitConfig{
			Upload:   10,
			Download: 100,
			Info:     50,
		},
		Logger: LoggerConfig{
			Level:  "info",
			Format: "json",
		},
	}

	err := config.Validate()
	assert.NoError(t, err)
}

func TestValidate_MissingS3Config(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		errMsg string
	}{
		{
			name: "missing bucket",
			config: &Config{
				S3: S3Config{
					AccessKey: "key",
					SecretKey: "secret",
					Bucket:    "", // Missing
				},
			},
			errMsg: "S3_BUCKET is required",
		},
		{
			name: "missing access key",
			config: &Config{
				S3: S3Config{
					AccessKey: "", // Missing
					SecretKey: "secret",
					Bucket:    "bucket",
				},
			},
			errMsg: "S3_ACCESS_KEY is required",
		},
		{
			name: "missing secret key",
			config: &Config{
				S3: S3Config{
					AccessKey: "key",
					SecretKey: "", // Missing
					Bucket:    "bucket",
				},
			},
			errMsg: "S3_SECRET_KEY is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set other required fields
			tt.config.Server.Port = "8080"
			tt.config.Cache.Type = "redis"
			tt.config.Redis.URL = "redis://localhost:6379"
			tt.config.Image.MaxFileSize = 10485760
			tt.config.Image.Quality = 85
			tt.config.Image.ResizeMode = "smart_fit"
			tt.config.Image.MaxWidth = 4096
			tt.config.Image.MaxHeight = 4096
			tt.config.RateLimit.Upload = 10
			tt.config.RateLimit.Download = 100
			tt.config.RateLimit.Info = 50
			tt.config.Logger.Level = "info"
			tt.config.Logger.Format = "json"

			err := tt.config.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestValidate_InvalidCacheConfig(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*Config)
		errMsg string
	}{
		{
			name: "invalid cache type",
			modify: func(c *Config) {
				c.Cache.Type = "invalid"
			},
			errMsg: "CACHE_TYPE must be one of",
		},
		{
			name: "missing redis url when cache type is redis",
			modify: func(c *Config) {
				c.Cache.Type = "redis"
				c.Redis.URL = ""
			},
			errMsg: "REDIS_URL is required when CACHE_TYPE=redis",
		},
		{
			name: "missing cache directory when cache type is badger",
			modify: func(c *Config) {
				c.Cache.Type = "badger"
				c.Cache.Directory = ""
			},
			errMsg: "CACHE_DIRECTORY is required when CACHE_TYPE=badger",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createValidConfig()
			tt.modify(config)

			err := config.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestValidate_ImageConfig(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*Config)
		errMsg string
	}{
		{
			name: "zero max file size",
			modify: func(c *Config) {
				c.Image.MaxFileSize = 0
			},
			errMsg: "MAX_FILE_SIZE must be positive",
		},
		{
			name: "negative max file size",
			modify: func(c *Config) {
				c.Image.MaxFileSize = -1
			},
			errMsg: "MAX_FILE_SIZE must be positive",
		},
		{
			name: "quality too low",
			modify: func(c *Config) {
				c.Image.Quality = 0
			},
			errMsg: "IMAGE_QUALITY must be between 1 and 100",
		},
		{
			name: "quality too high",
			modify: func(c *Config) {
				c.Image.Quality = 101
			},
			errMsg: "IMAGE_QUALITY must be between 1 and 100",
		},
		{
			name: "invalid resize mode",
			modify: func(c *Config) {
				c.Image.ResizeMode = "invalid"
			},
			errMsg: "RESIZE_MODE must be one of",
		},
		{
			name: "zero max width",
			modify: func(c *Config) {
				c.Image.MaxWidth = 0
			},
			errMsg: "IMAGE_MAX_WIDTH must be a positive integer",
		},
		{
			name: "negative max width",
			modify: func(c *Config) {
				c.Image.MaxWidth = -1
			},
			errMsg: "IMAGE_MAX_WIDTH must be a positive integer",
		},
		{
			name: "zero max height",
			modify: func(c *Config) {
				c.Image.MaxHeight = 0
			},
			errMsg: "IMAGE_MAX_HEIGHT must be a positive integer",
		},
		{
			name: "negative max height",
			modify: func(c *Config) {
				c.Image.MaxHeight = -1
			},
			errMsg: "IMAGE_MAX_HEIGHT must be a positive integer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createValidConfig()
			tt.modify(config)

			err := config.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestValidate_RateLimitConfig(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*Config)
		errMsg string
	}{
		{
			name: "zero upload rate",
			modify: func(c *Config) {
				c.RateLimit.Upload = 0
			},
			errMsg: "rate limits must be positive integers",
		},
		{
			name: "negative upload rate",
			modify: func(c *Config) {
				c.RateLimit.Upload = -1
			},
			errMsg: "rate limits must be positive integers",
		},
		{
			name: "zero download rate",
			modify: func(c *Config) {
				c.RateLimit.Download = 0
			},
			errMsg: "rate limits must be positive integers",
		},
		{
			name: "zero info rate",
			modify: func(c *Config) {
				c.RateLimit.Info = 0
			},
			errMsg: "rate limits must be positive integers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createValidConfig()
			tt.modify(config)

			err := config.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestValidate_LoggerConfig(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*Config)
		errMsg string
	}{
		{
			name: "invalid log level",
			modify: func(c *Config) {
				c.Logger.Level = "invalid"
			},
			errMsg: "LOG_LEVEL must be one of",
		},
		{
			name: "invalid log format",
			modify: func(c *Config) {
				c.Logger.Format = "invalid"
			},
			errMsg: "LOG_FORMAT must be one of",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := createValidConfig()
			tt.modify(config)

			err := config.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestValidate_EmptyPort(t *testing.T) {
	config := createValidConfig()
	config.Server.Port = ""

	err := config.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "PORT cannot be empty")
}

func TestIsDevelopment(t *testing.T) {
	tests := []struct {
		name     string
		ginMode  string
		format   string
		expected bool
	}{
		{"debug gin mode", "debug", "json", true},
		{"console format", "release", "console", true},
		{"both dev indicators", "debug", "console", true},
		{"production", "release", "json", false},
		{"test mode", "test", "json", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Server: ServerConfig{GinMode: tt.ginMode},
				Logger: LoggerConfig{Format: tt.format},
			}
			assert.Equal(t, tt.expected, config.IsDevelopment())
		})
	}
}

func TestIsProduction(t *testing.T) {
	tests := []struct {
		name     string
		ginMode  string
		format   string
		expected bool
	}{
		{"production", "release", "json", true},
		{"debug mode", "debug", "json", false},
		{"console format", "release", "console", false},
		{"both non-prod", "debug", "console", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				Server: ServerConfig{GinMode: tt.ginMode},
				Logger: LoggerConfig{Format: tt.format},
			}
			assert.Equal(t, tt.expected, config.IsProduction())
		})
	}
}

func TestGetResolution(t *testing.T) {
	config := &Config{
		Image: ImageConfig{
			DefaultResolutions: map[string]ResolutionConfig{
				"thumbnail": {Width: 150, Height: 150},
			},
		},
	}

	// Test existing resolution
	resolution, exists := config.GetResolution("thumbnail")
	assert.True(t, exists)
	assert.Equal(t, 150, resolution.Width)
	assert.Equal(t, 150, resolution.Height)

	// Test non-existing resolution
	_, exists = config.GetResolution("nonexistent")
	assert.False(t, exists)
}

func TestIsSupportedFormat(t *testing.T) {
	config := &Config{
		Image: ImageConfig{
			SupportedFormats: []string{"image/jpeg", "image/png", "image/gif"},
		},
	}

	assert.True(t, config.IsSupportedFormat("image/jpeg"))
	assert.True(t, config.IsSupportedFormat("image/png"))
	assert.True(t, config.IsSupportedFormat("image/gif"))
	assert.False(t, config.IsSupportedFormat("image/webp"))
	assert.False(t, config.IsSupportedFormat("text/plain"))
}

func TestGetEnvHelpers(t *testing.T) {
	t.Run("getEnv", func(t *testing.T) {
		os.Setenv("TEST_STRING", "test_value")
		defer os.Unsetenv("TEST_STRING")

		assert.Equal(t, "test_value", getEnv("TEST_STRING", "default"))
		assert.Equal(t, "default", getEnv("NONEXISTENT", "default"))
	})

	t.Run("getEnvInt", func(t *testing.T) {
		os.Setenv("TEST_INT", "123")
		os.Setenv("TEST_INT_INVALID", "not_a_number")
		defer func() {
			os.Unsetenv("TEST_INT")
			os.Unsetenv("TEST_INT_INVALID")
		}()

		assert.Equal(t, 123, getEnvInt("TEST_INT", 456))
		assert.Equal(t, 456, getEnvInt("TEST_INT_INVALID", 456))
		assert.Equal(t, 456, getEnvInt("NONEXISTENT", 456))
	})

	t.Run("getEnvBool", func(t *testing.T) {
		os.Setenv("TEST_BOOL_TRUE", "true")
		os.Setenv("TEST_BOOL_FALSE", "false")
		os.Setenv("TEST_BOOL_INVALID", "not_a_bool")
		defer func() {
			os.Unsetenv("TEST_BOOL_TRUE")
			os.Unsetenv("TEST_BOOL_FALSE")
			os.Unsetenv("TEST_BOOL_INVALID")
		}()

		assert.True(t, getEnvBool("TEST_BOOL_TRUE", false))
		assert.False(t, getEnvBool("TEST_BOOL_FALSE", true))
		assert.True(t, getEnvBool("TEST_BOOL_INVALID", true))
		assert.False(t, getEnvBool("NONEXISTENT", false))
	})

	t.Run("getEnvFloat", func(t *testing.T) {
		os.Setenv("TEST_FLOAT", "123.45")
		os.Setenv("TEST_FLOAT_INVALID", "not_a_float")
		defer func() {
			os.Unsetenv("TEST_FLOAT")
			os.Unsetenv("TEST_FLOAT_INVALID")
		}()

		assert.Equal(t, 123.45, getEnvFloat("TEST_FLOAT", 678.90))
		assert.Equal(t, 678.90, getEnvFloat("TEST_FLOAT_INVALID", 678.90))
		assert.Equal(t, 678.90, getEnvFloat("NONEXISTENT", 678.90))
	})

	t.Run("getEnvDuration", func(t *testing.T) {
		os.Setenv("TEST_DURATION", "5m")
		os.Setenv("TEST_DURATION_INVALID", "not_a_duration")
		defer func() {
			os.Unsetenv("TEST_DURATION")
			os.Unsetenv("TEST_DURATION_INVALID")
		}()

		assert.Equal(t, 5*time.Minute, getEnvDuration("TEST_DURATION", time.Hour))
		assert.Equal(t, time.Hour, getEnvDuration("TEST_DURATION_INVALID", time.Hour))
		assert.Equal(t, time.Hour, getEnvDuration("NONEXISTENT", time.Hour))
	})

	t.Run("getEnvStringSlice", func(t *testing.T) {
		os.Setenv("TEST_SLICE", "a,b,c")
		os.Setenv("TEST_SLICE_SPACES", " a , b , c ")
		os.Setenv("TEST_SLICE_EMPTY", "")
		defer func() {
			os.Unsetenv("TEST_SLICE")
			os.Unsetenv("TEST_SLICE_SPACES")
			os.Unsetenv("TEST_SLICE_EMPTY")
		}()

		assert.Equal(t, []string{"a", "b", "c"}, getEnvStringSlice("TEST_SLICE", []string{"default"}))
		assert.Equal(t, []string{"a", "b", "c"}, getEnvStringSlice("TEST_SLICE_SPACES", []string{"default"}))
		assert.Equal(t, []string{"default"}, getEnvStringSlice("TEST_SLICE_EMPTY", []string{"default"}))
		assert.Equal(t, []string{"default"}, getEnvStringSlice("NONEXISTENT", []string{"default"}))
	})
}

func TestContains(t *testing.T) {
	slice := []string{"a", "b", "c"}

	assert.True(t, contains(slice, "a"))
	assert.True(t, contains(slice, "b"))
	assert.True(t, contains(slice, "c"))
	assert.False(t, contains(slice, "d"))
	assert.False(t, contains(slice, ""))
	assert.False(t, contains([]string{}, "a"))
}

func TestLoad_ValidationError(t *testing.T) {
	clearEnv()

	// Set incomplete configuration (missing required S3 fields)
	os.Setenv("S3_BUCKET", "test-bucket")
	// Missing S3_ACCESS_KEY and S3_SECRET_KEY
	defer clearEnv()

	_, err := Load()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "configuration validation failed")
}

func TestResolutionConfig(t *testing.T) {
	config := ResolutionConfig{Width: 800, Height: 600}

	assert.Equal(t, 800, config.Width)
	assert.Equal(t, 600, config.Height)
}

// Helper functions

func createValidConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:    "8080",
			GinMode: "release",
		},
		Cache: CacheConfig{
			Type: "redis",
		},
		Redis: RedisConfig{
			URL: "redis://localhost:6379",
		},
		S3: S3Config{
			AccessKey: "key",
			SecretKey: "secret",
			Bucket:    "bucket",
		},
		Image: ImageConfig{
			MaxFileSize: 10485760,
			Quality:     85,
			ResizeMode:  "smart_fit",
			MaxWidth:    4096,
			MaxHeight:   4096,
		},
		RateLimit: RateLimitConfig{
			Upload:   10,
			Download: 100,
			Info:     50,
		},
		Logger: LoggerConfig{
			Level:  "info",
			Format: "json",
		},
	}
}

func clearEnv() {
	envVars := []string{
		"PORT", "GIN_MODE", "REDIS_URL", "REDIS_PASSWORD", "REDIS_DB", "REDIS_POOL_SIZE", "REDIS_TIMEOUT",
		"CACHE_TYPE", "CACHE_DIRECTORY", "CACHE_TTL", "S3_ENDPOINT", "S3_ACCESS_KEY", "S3_SECRET_KEY",
		"S3_BUCKET", "S3_REGION", "S3_USE_SSL", "S3_URL_EXPIRE", "MAX_FILE_SIZE", "IMAGE_QUALITY",
		"GENERATE_DEFAULT_RESOLUTIONS", "RESIZE_MODE", "IMAGE_MAX_WIDTH", "IMAGE_MAX_HEIGHT",
		"RATE_LIMIT_UPLOAD", "RATE_LIMIT_DOWNLOAD", "RATE_LIMIT_INFO", "LOG_LEVEL", "LOG_FORMAT",
		"CORS_ENABLED", "CORS_ALLOW_ALL_ORIGINS", "CORS_ALLOWED_ORIGINS", "CORS_ALLOW_CREDENTIALS",
		"S3_HEALTHCHECKS_DISABLE", "S3_HEALTHCHECKS_INTERVAL", "HEALTHCHECK_INTERVAL",
		"AUTH_ENABLED", "AUTH_READWRITE_KEYS", "AUTH_READONLY_KEYS", "AUTH_KEY_HEADER",
	}

	for _, env := range envVars {
		os.Unsetenv(env)
	}
}

func TestS3HealthCheckInterval_MinimumLimit(t *testing.T) {
	tests := []struct {
		name           string
		envValue       string
		expectedResult time.Duration
		description    string
	}{
		{
			name:           "Below minimum",
			envValue:       "2",
			expectedResult: 10 * time.Second,
			description:    "Values below 10 seconds should be adjusted to 10 seconds",
		},
		{
			name:           "At minimum",
			envValue:       "10",
			expectedResult: 10 * time.Second,
			description:    "Minimum value of 10 seconds should be preserved",
		},
		{
			name:           "Above minimum",
			envValue:       "60",
			expectedResult: 60 * time.Second,
			description:    "Values above 10 seconds should be preserved",
		},
		{
			name:           "Default value",
			envValue:       "",
			expectedResult: 30 * time.Second,
			description:    "Default value should be 30 seconds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			clearEnv()

			// Set test value if provided
			if tt.envValue != "" {
				os.Setenv("S3_HEALTHCHECKS_INTERVAL", tt.envValue)
			}

			// Set required config values
			os.Setenv("S3_BUCKET", "test-bucket")
			os.Setenv("S3_ACCESS_KEY", "test-key")
			os.Setenv("S3_SECRET_KEY", "test-secret")

			defer clearEnv()

			config, err := Load()
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedResult, config.Health.S3ChecksInterval, tt.description)
		})
	}
}

func TestHealthCheckInterval_MinimumLimit(t *testing.T) {
	tests := []struct {
		name           string
		envValue       string
		expectedResult time.Duration
		description    string
	}{
		{
			name:           "Below minimum",
			envValue:       "5",
			expectedResult: 10 * time.Second,
			description:    "Values below 10 seconds should be adjusted to 10 seconds",
		},
		{
			name:           "At minimum",
			envValue:       "10",
			expectedResult: 10 * time.Second,
			description:    "Minimum value of 10 seconds should be preserved",
		},
		{
			name:           "Above minimum",
			envValue:       "45",
			expectedResult: 45 * time.Second,
			description:    "Values above 10 seconds should be preserved",
		},
		{
			name:           "Default value",
			envValue:       "",
			expectedResult: 30 * time.Second,
			description:    "Default value should be 30 seconds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			clearEnv()

			// Set test value if provided
			if tt.envValue != "" {
				os.Setenv("HEALTHCHECK_INTERVAL", tt.envValue)
			}

			// Set required config values
			os.Setenv("S3_BUCKET", "test-bucket")
			os.Setenv("S3_ACCESS_KEY", "test-key")
			os.Setenv("S3_SECRET_KEY", "test-secret")

			defer clearEnv()

			config, err := Load()
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedResult, config.Health.CheckInterval, tt.description)
		})
	}
}

func TestLoad_AuthConfiguration(t *testing.T) {
	tests := []struct {
		name              string
		envVars           map[string]string
		expectedEnabled   bool
		expectedRWKeys    []string
		expectedROKeys    []string
		expectedKeyHeader string
	}{
		{
			name: "auth disabled by default",
			envVars: map[string]string{
				"S3_BUCKET":     "test-bucket",
				"S3_ACCESS_KEY": "test-key",
				"S3_SECRET_KEY": "test-secret",
			},
			expectedEnabled:   false,
			expectedRWKeys:    []string{},
			expectedROKeys:    []string{},
			expectedKeyHeader: "X-API-Key",
		},
		{
			name: "auth enabled with keys",
			envVars: map[string]string{
				"S3_BUCKET":           "test-bucket",
				"S3_ACCESS_KEY":       "test-key",
				"S3_SECRET_KEY":       "test-secret",
				"AUTH_ENABLED":        "true",
				"AUTH_READWRITE_KEYS": "rw-key-1,rw-key-2",
				"AUTH_READONLY_KEYS":  "ro-key-1,ro-key-2,ro-key-3",
				"AUTH_KEY_HEADER":     "Authorization",
			},
			expectedEnabled:   true,
			expectedRWKeys:    []string{"rw-key-1", "rw-key-2"},
			expectedROKeys:    []string{"ro-key-1", "ro-key-2", "ro-key-3"},
			expectedKeyHeader: "Authorization",
		},
		{
			name: "auth enabled without keys",
			envVars: map[string]string{
				"S3_BUCKET":     "test-bucket",
				"S3_ACCESS_KEY": "test-key",
				"S3_SECRET_KEY": "test-secret",
				"AUTH_ENABLED":  "true",
			},
			expectedEnabled:   true,
			expectedRWKeys:    []string{},
			expectedROKeys:    []string{},
			expectedKeyHeader: "X-API-Key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv()

			// Set environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}
			defer clearEnv()

			config, err := Load()
			assert.NoError(t, err)
			assert.NotNil(t, config)

			// Verify auth configuration
			assert.Equal(t, tt.expectedEnabled, config.Auth.Enabled)
			assert.Equal(t, tt.expectedRWKeys, config.Auth.ReadWriteKeys)
			assert.Equal(t, tt.expectedROKeys, config.Auth.ReadOnlyKeys)
			assert.Equal(t, tt.expectedKeyHeader, config.Auth.KeyHeader)
		})
	}
}
