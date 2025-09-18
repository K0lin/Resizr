package api

import (
	"resizr/internal/api/handlers"
	"resizr/internal/api/middleware"
	"resizr/internal/config"
	"resizr/internal/service"

	"github.com/gin-gonic/gin"
)

// Router holds the HTTP router and dependencies
type Router struct {
	engine        *gin.Engine
	config        *config.Config
	imageHandler  *handlers.ImageHandler
	healthHandler *handlers.HealthHandler
	authHandler   *handlers.AuthHandler
}

// NewRouter creates a new HTTP router with all routes configured
func NewRouter(cfg *config.Config, imageService service.ImageService, healthService service.HealthService) *Router {
	// Set Gin mode based on config
	if cfg.IsDevelopment() {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()

	// Create handlers
	imageHandler := handlers.NewImageHandler(imageService, cfg)
	healthHandler := handlers.NewHealthHandler(healthService)
	authHandler := handlers.NewAuthHandler(cfg)

	router := &Router{
		engine:        engine,
		config:        cfg,
		imageHandler:  imageHandler,
		healthHandler: healthHandler,
		authHandler:   authHandler,
	}

	// Setup middleware and routes
	router.setupMiddleware()
	router.setupRoutes()

	return router
}

// setupMiddleware configures all middleware
func (r *Router) setupMiddleware() {
	// Basic middleware
	r.engine.Use(gin.Logger())
	r.engine.Use(gin.Recovery())

	// Request ID middleware for tracing
	r.engine.Use(middleware.RequestID())

	// CORS middleware
	r.engine.Use(middleware.CORS(r.config))

	// Rate limiting middleware
	r.engine.Use(middleware.RateLimit(r.config))

	// Request size limit middleware
	r.engine.Use(middleware.RequestSizeLimit(r.config.Image.MaxFileSize))
}

// setupRoutes configures all API routes
func (r *Router) setupRoutes() {
	// Health check endpoint (no prefix, no auth)
	r.engine.GET("/health", r.healthHandler.Health)

	// API v1 routes
	v1 := r.engine.Group("/api/v1")
	{
		// Authentication endpoints (no auth required)
		auth := v1.Group("/auth")
		{
			auth.GET("/generate-key", r.authHandler.GenerateAPIKey)
			auth.GET("/status", r.authHandler.GetAuthStatus)
		}

		// Image endpoints (with authentication)
		images := v1.Group("/images")
		images.Use(middleware.APIKeyAuth(r.config))
		{
			// Write operations (require read-write permission)
			images.POST("", middleware.RequirePermission(middleware.PermissionReadWrite), r.imageHandler.Upload)

			// Read operations (require read permission - both read-only and read-write keys work)
			images.GET("/:id/info", middleware.RequirePermission(middleware.PermissionRead), r.imageHandler.Info)
			images.GET("/:id/original", middleware.RequirePermission(middleware.PermissionRead), r.imageHandler.DownloadOriginal)
			images.GET("/:id/thumbnail", middleware.RequirePermission(middleware.PermissionRead), r.imageHandler.DownloadThumbnail)
			images.GET("/:id/:resolution", middleware.RequirePermission(middleware.PermissionRead), r.imageHandler.DownloadCustomResolution)

			// Presigned URL generation (require read permission)
			images.GET("/:id/original/presigned-url", middleware.RequirePermission(middleware.PermissionRead), r.imageHandler.GeneratePresignedURL)
			images.GET("/:id/thumbnail/presigned-url", middleware.RequirePermission(middleware.PermissionRead), r.imageHandler.GeneratePresignedURL)
			images.GET("/:id/:resolution/presigned-url", middleware.RequirePermission(middleware.PermissionRead), r.imageHandler.GeneratePresignedURL)

			// Future: Delete image (require read-write permission)
			// images.DELETE("/:id", middleware.RequirePermission(middleware.PermissionReadWrite), r.imageHandler.Delete)
		}
	}

	// Optional: Metrics endpoint for monitoring
	if r.config.IsDevelopment() {
		r.engine.GET("/debug/vars", r.healthHandler.Metrics)
	}
}

// GetEngine returns the Gin engine
func (r *Router) GetEngine() *gin.Engine {
	return r.engine
}

// PrintRoutes prints all registered routes (useful for debugging)
func (r *Router) PrintRoutes() {
	for _, route := range r.engine.Routes() {
		println(route.Method, route.Path)
	}
}
