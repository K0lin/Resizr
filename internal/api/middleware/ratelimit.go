package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"resizr/internal/config"
	"resizr/internal/models"
	"resizr/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// RateLimiter holds rate limiting configuration and state
type RateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	config   *config.Config

	// Cleanup ticker for removing old limiters
	cleanup     *time.Ticker
	stopCleanup chan struct{}
}

// ClientLimiter holds rate limiter info for a client
type ClientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	globalRateLimiter *RateLimiter
	once              sync.Once
)

// RateLimit middleware applies rate limiting per IP address and endpoint
func RateLimit(cfg *config.Config) gin.HandlerFunc {
	// Initialize global rate limiter (singleton)
	once.Do(func() {
		globalRateLimiter = &RateLimiter{
			limiters:    make(map[string]*rate.Limiter),
			config:      cfg,
			cleanup:     time.NewTicker(10 * time.Minute),
			stopCleanup: make(chan struct{}),
		}

		// Start cleanup goroutine
		go globalRateLimiter.startCleanup()
	})

	return globalRateLimiter.middleware
}

// middleware is the actual Gin middleware function
func (rl *RateLimiter) middleware(c *gin.Context) {
	clientIP := c.ClientIP()
	endpoint := c.Request.Method + " " + c.FullPath()
	key := fmt.Sprintf("%s:%s", clientIP, endpoint)

	// Get rate limit for this endpoint
	limit := rl.getRateLimit(c.Request.Method, c.FullPath())
	if limit <= 0 {
		// No rate limiting for this endpoint
		c.Next()
		return
	}

	// Get or create limiter for this client+endpoint
	limiter := rl.getLimiter(key, limit)

	// Check if request is allowed
	if !limiter.Allow() {
		rl.handleRateLimitExceeded(c, clientIP, endpoint, limit)
		return
	}

	// Set rate limit headers
	rl.setRateLimitHeaders(c, limiter, limit)

	c.Next()
}

// getRateLimit returns the rate limit for a specific endpoint
func (rl *RateLimiter) getRateLimit(method, path string) int {
	// Upload endpoints (more restrictive)
	if method == "POST" && strings.Contains(path, "/images") {
		return rl.config.RateLimit.Upload
	}

	// Download endpoints (less restrictive)
	if method == "GET" && strings.Contains(path, "/images/") && !strings.HasSuffix(path, "/info") {
		return rl.config.RateLimit.Download
	}

	// Info endpoints
	if method == "GET" && (strings.HasSuffix(path, "/info") || path == "/health") {
		return rl.config.RateLimit.Info
	}

	// Default: no limit
	return 0
}

// getLimiter gets or creates a rate limiter for a client+endpoint
func (rl *RateLimiter) getLimiter(key string, limit int) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.limiters[key]
	if !exists {
		// Create new limiter: burst = 2x rate, refill every minute
		ratePerSecond := rate.Limit(float64(limit) / 60.0) // Convert per-minute to per-second
		limiter = rate.NewLimiter(ratePerSecond, limit*2)
		rl.limiters[key] = limiter
	}

	return limiter
}

// setRateLimitHeaders sets rate limiting headers for client information
func (rl *RateLimiter) setRateLimitHeaders(c *gin.Context, limiter *rate.Limiter, limit int) {
	// Get current token count (approximate)
	burst := limiter.Burst()

	c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
	c.Header("X-RateLimit-Burst", fmt.Sprintf("%d", burst))
	c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Minute).Unix()))
}

// handleRateLimitExceeded handles rate limit exceeded scenarios
func (rl *RateLimiter) handleRateLimitExceeded(c *gin.Context, clientIP, endpoint string, limit int) {
	ctx := c.Request.Context()
	requestID := c.GetString("request_id")

	logger.WarnWithContext(ctx, "Rate limit exceeded",
		zap.String("client_ip", clientIP),
		zap.String("endpoint", endpoint),
		zap.Int("limit", limit),
		zap.String("request_id", requestID))

	// Set rate limit headers
	c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
	c.Header("X-RateLimit-Remaining", "0")
	c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Minute).Unix()))
	c.Header("Retry-After", "60")

	c.JSON(http.StatusTooManyRequests, models.ErrorResponse{
		Error:   "Rate limit exceeded",
		Message: fmt.Sprintf("Too many requests. Limit: %d requests per minute", limit),
		Code:    http.StatusTooManyRequests,
	})

	c.Abort()
}

// startCleanup starts the background cleanup goroutine
func (rl *RateLimiter) startCleanup() {
	for {
		select {
		case <-rl.cleanup.C:
			rl.cleanupOldLimiters()
		case <-rl.stopCleanup:
			return
		}
	}
}

// cleanupOldLimiters removes limiters that haven't been used recently
func (rl *RateLimiter) cleanupOldLimiters() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	toDelete := make([]string, 0)

	// Mark old limiters for deletion
	// Note: This is a simplified cleanup - in production, you'd track lastUsed time
	if len(rl.limiters) > 10000 { // Prevent memory bloat
		// Keep only recent limiters (this is simplified logic)
		newLimiters := make(map[string]*rate.Limiter)
		count := 0
		for key, limiter := range rl.limiters {
			if count < 5000 { // Keep most recent 5000
				newLimiters[key] = limiter
				count++
			} else {
				toDelete = append(toDelete, key)
			}
		}
		rl.limiters = newLimiters
	}

	if len(toDelete) > 0 {
		logger.Debug("Cleaned up old rate limiters",
			zap.Int("deleted_count", len(toDelete)),
			zap.Int("remaining_count", len(rl.limiters)))
	}
}

// Stop stops the rate limiter cleanup
func (rl *RateLimiter) Stop() {
	if rl.cleanup != nil {
		rl.cleanup.Stop()
	}
	if rl.stopCleanup != nil {
		close(rl.stopCleanup)
	}
}
