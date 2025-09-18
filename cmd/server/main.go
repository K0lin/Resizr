package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"resizr/internal/api"
	"resizr/internal/config"
	"resizr/internal/repository"
	"resizr/internal/service"
	"resizr/internal/storage"
	"resizr/pkg/logger"

	"go.uber.org/zap"
)

const (
	// Application information
	AppName    = "Resizr"
	AppVersion = "0.0.1"

	// Graceful shutdown timeout
	ShutdownTimeout = 30 * time.Second
)

func main() {
	// Initialize application
	if err := run(); err != nil {
		log.Fatalf("Application failed to start: %v", err)
	}
}

func run() error {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Initialize logger first
	if err := logger.Init(logger.Config{
		Level:  cfg.Logger.Level,
		Format: cfg.Logger.Format,
	}); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	logger.Info("Starting RESIZR application",
		zap.String("version", AppVersion),
		zap.String("port", cfg.Server.Port),
		zap.Bool("development", cfg.IsDevelopment()))

	// Initialize repository (composite: Redis + configurable cache)
	logger.Info("Initializing image repository...")
	repo, err := repository.NewImageRepository(cfg)
	if err != nil {
		logger.Fatal("Failed to initialize image repository", zap.Error(err))
		return fmt.Errorf("failed to initialize repository: %w", err)
	}
	defer func() {
		if err := repo.Close(); err != nil {
			logger.Error("Failed to close repository", zap.Error(err))
		}
	}()

	// Initialize storage (S3)
	logger.Info("Initializing S3 storage...")
	store, err := storage.NewS3Storage(&cfg.S3)
	if err != nil {
		logger.Fatal("Failed to initialize S3 storage", zap.Error(err))
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Initialize image processor
	logger.Info("Initializing image processor...")
	// Allow configuration via env (IMAGE_MAX_WIDTH/IMAGE_MAX_HEIGHT) with sensible defaults
	maxW := cfg.Image.MaxWidth
	maxH := cfg.Image.MaxHeight
	// Hard cap at 8192 to prevent excessive memory usage even if misconfigured
	if maxW <= 0 || maxW > 8192 {
		maxW = 8192
	}
	if maxH <= 0 || maxH > 8192 {
		maxH = 8192
	}
	processor := service.NewProcessorService(maxW, maxH)

	// Initialize services
	logger.Info("Initializing services...")
	imageService := service.NewImageService(repo, store, processor, cfg)
	healthService := service.NewHealthService(repo, store, cfg, AppVersion)

	// Initialize API router
	logger.Info("Initializing API router...")
	router := api.NewRouter(cfg, imageService, healthService)

	// Create HTTP server
	server := &http.Server{
		Addr:           ":" + cfg.Server.Port,
		Handler:        router.GetEngine(),
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}

	// Start server in a goroutine
	serverErrChan := make(chan error, 1)
	go func() {
		logger.Info("Starting HTTP server",
			zap.String("addr", server.Addr),
			zap.String("mode", cfg.Server.GinMode))

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrChan <- fmt.Errorf("server failed to start: %w", err)
		}
	}()

	// Print routes in development mode
	if cfg.IsDevelopment() {
		logger.Info("Available routes:")
		router.PrintRoutes()
	}

	logger.Info(AppName+" application started successfully",
		zap.String("version", AppVersion),
		zap.String("port", cfg.Server.Port))

	// Wait for interrupt signal or server error
	return waitForShutdown(server, serverErrChan)
}

// waitForShutdown waits for shutdown signal and gracefully shuts down the server
func waitForShutdown(server *http.Server, serverErrChan chan error) error {
	// Channel to listen for interrupt signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrChan:
		return err
	case sig := <-quit:
		logger.Info("Received shutdown signal, starting graceful shutdown...",
			zap.String("signal", sig.String()))

		return gracefulShutdown(server)
	}
}

// gracefulShutdown performs graceful shutdown of the server
func gracefulShutdown(server *http.Server) error {
	// Create context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
	defer cancel()

	// Attempt graceful shutdown
	logger.Info("Shutting down HTTP server...",
		zap.Duration("timeout", ShutdownTimeout))

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Failed to gracefully shutdown server", zap.Error(err))
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	logger.Info("Server shut down successfully")
	return nil
}

// Health check endpoint information for monitoring
func init() {
	// Register application info that can be used by monitoring systems
	log.Printf("RESIZR %s initializing...", AppVersion)
}
