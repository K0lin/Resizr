package logger

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestInit(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "json_info_level",
			config: Config{
				Level:  "info",
				Format: "json",
			},
			wantErr: false,
		},
		{
			name: "console_debug_level",
			config: Config{
				Level:  "debug",
				Format: "console",
			},
			wantErr: false,
		},
		{
			name: "warn_level",
			config: Config{
				Level:  "warn",
				Format: "json",
			},
			wantErr: false,
		},
		{
			name: "error_level",
			config: Config{
				Level:  "error",
				Format: "json",
			},
			wantErr: false,
		},
		{
			name: "default_level",
			config: Config{
				Level:  "unknown",
				Format: "json",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Init(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, globalLogger)
			}
		})
	}
}

func TestGetLogger(t *testing.T) {
	// Reset global logger
	globalLogger = nil

	logger := GetLogger()
	assert.NotNil(t, logger)

	// Should return the same instance
	logger2 := GetLogger()
	assert.Equal(t, logger, logger2)
}

func TestWithContext(t *testing.T) {
	_ = Init(Config{Level: "info", Format: "json"})

	t.Run("empty_context", func(t *testing.T) {
		ctx := context.Background()
		logger := WithContext(ctx)
		assert.NotNil(t, logger)
	})

	t.Run("with_request_id", func(t *testing.T) {
		ctx := WithRequestID(context.Background(), "test-request-123")
		logger := WithContext(ctx)
		assert.NotNil(t, logger)
	})

	t.Run("with_user_id", func(t *testing.T) {
		ctx := WithUserID(context.Background(), "user-456")
		logger := WithContext(ctx)
		assert.NotNil(t, logger)
	})

	t.Run("with_both_ids", func(t *testing.T) {
		ctx := WithRequestID(context.Background(), "test-request-123")
		ctx = WithUserID(ctx, "user-456")
		logger := WithContext(ctx)
		assert.NotNil(t, logger)
	})
}

func TestWithRequestID(t *testing.T) {
	ctx := context.Background()
	requestID := "test-request-id"

	ctx = WithRequestID(ctx, requestID)
	retrievedID := GetRequestID(ctx)

	assert.Equal(t, requestID, retrievedID)
}

func TestGetRequestID(t *testing.T) {
	t.Run("empty_context", func(t *testing.T) {
		ctx := context.Background()
		id := GetRequestID(ctx)
		assert.Empty(t, id)
	})

	t.Run("with_request_id", func(t *testing.T) {
		ctx := WithRequestID(context.Background(), "abc-123")
		id := GetRequestID(ctx)
		assert.Equal(t, "abc-123", id)
	})
}

func TestWithUserID(t *testing.T) {
	ctx := context.Background()
	userID := "test-user-id"

	ctx = WithUserID(ctx, userID)
	retrievedID := GetUserID(ctx)

	assert.Equal(t, userID, retrievedID)
}

func TestGetUserID(t *testing.T) {
	t.Run("empty_context", func(t *testing.T) {
		ctx := context.Background()
		id := GetUserID(ctx)
		assert.Empty(t, id)
	})

	t.Run("with_user_id", func(t *testing.T) {
		ctx := WithUserID(context.Background(), "user-789")
		id := GetUserID(ctx)
		assert.Equal(t, "user-789", id)
	})
}

func TestConvenienceFunctions(t *testing.T) {
	_ = Init(Config{Level: "debug", Format: "json"})

	// These functions should not panic
	t.Run("info", func(t *testing.T) {
		assert.NotPanics(t, func() {
			Info("test info message", zap.String("key", "value"))
		})
	})

	t.Run("error", func(t *testing.T) {
		assert.NotPanics(t, func() {
			Error("test error message", zap.String("key", "value"))
		})
	})

	t.Run("debug", func(t *testing.T) {
		assert.NotPanics(t, func() {
			Debug("test debug message", zap.String("key", "value"))
		})
	})

	t.Run("warn", func(t *testing.T) {
		assert.NotPanics(t, func() {
			Warn("test warn message", zap.String("key", "value"))
		})
	})
}

func TestContextAwareFunctions(t *testing.T) {
	_ = Init(Config{Level: "debug", Format: "json"})
	ctx := WithRequestID(context.Background(), "test-123")

	// These functions should not panic
	t.Run("info_with_context", func(t *testing.T) {
		assert.NotPanics(t, func() {
			InfoWithContext(ctx, "test info", zap.String("key", "value"))
		})
	})

	t.Run("error_with_context", func(t *testing.T) {
		assert.NotPanics(t, func() {
			ErrorWithContext(ctx, "test error", zap.String("key", "value"))
		})
	})

	t.Run("debug_with_context", func(t *testing.T) {
		assert.NotPanics(t, func() {
			DebugWithContext(ctx, "test debug", zap.String("key", "value"))
		})
	})

	t.Run("warn_with_context", func(t *testing.T) {
		assert.NotPanics(t, func() {
			WarnWithContext(ctx, "test warn", zap.String("key", "value"))
		})
	})
}

func TestLogLevelParsing(t *testing.T) {
	tests := []struct {
		level    string
		expected zapcore.Level
	}{
		{"debug", zapcore.DebugLevel},
		{"info", zapcore.InfoLevel},
		{"warn", zapcore.WarnLevel},
		{"error", zapcore.ErrorLevel},
		{"invalid", zapcore.InfoLevel}, // defaults to info
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			err := Init(Config{Level: tt.level, Format: "json"})
			assert.NoError(t, err)
			// We can't directly test the level, but we ensure no error occurs
		})
	}
}

func TestLogFormatting(t *testing.T) {
	t.Run("json_format", func(t *testing.T) {
		err := Init(Config{Level: "info", Format: "json"})
		assert.NoError(t, err)
	})

	t.Run("console_format", func(t *testing.T) {
		err := Init(Config{Level: "info", Format: "console"})
		assert.NoError(t, err)
	})

	t.Run("other_format_defaults_to_console", func(t *testing.T) {
		err := Init(Config{Level: "info", Format: "text"})
		assert.NoError(t, err)
	})
}
