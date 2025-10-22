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
	"resizr/internal/queue"
	"resizr/internal/repository"
	"resizr/internal/service"
	"resizr/internal/storage"
	"resizr/internal/worker"
	"resizr/pkg/logger"

	"github.com/dgraph-io/badger/v4"
	"github.com/go-redis/redis/v8"
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

	// Initialize storage
	logger.Info("Initializing storage...")
	var store storage.ImageStorage
	switch cfg.Storage.Provider {
	case "s3":
		store, err = storage.NewS3Storage(&cfg.S3)
	case "local":
		store, err = storage.NewLocalStorage(cfg.Storage.Directory)
	default:
		return fmt.Errorf("unsupported storage provider: %s", cfg.Storage.Provider)
	}
	if err != nil {
		logger.Fatal("Failed to initialize storage", zap.Error(err))
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

	// Cast repository to deduplication repository interface
	dedupRepo, ok := repo.(repository.DeduplicationRepository)
	if !ok {
		logger.Fatal("Repository does not support deduplication")
		return fmt.Errorf("repository does not implement DeduplicationRepository interface")
	}

	imageService := service.NewImageService(repo, dedupRepo, store, processor, cfg)
	healthService := service.NewHealthService(repo, store, cfg, AppVersion)
	statisticsService := service.NewStatisticsService(repo, dedupRepo, store, cfg)

	// Initialize deletion queue and workers if async mode is enabled
	var workerPool *worker.WorkerPool
	if cfg.Deletion.AsyncMode {
		logger.Info("Initializing async deletion queue and workers",
			zap.Int("worker_count", cfg.Deletion.WorkerCount))

		// Get underlying Redis/Badger clients from repository
		var deletionQueue queue.DeletionQueue
		var intentMgr worker.IntentManager

		// For simplicity, create new Redis/Badger clients for the queue
		// (they share the same connection configuration)
		switch cfg.Cache.Type {
		case "redis":
			// Create Redis client for queue
			redisClient := redis.NewClient(&redis.Options{
				Addr:     cfg.Redis.URL,
				Password: cfg.Redis.Password,
				DB:       cfg.Redis.DB,
			})

			deletionQueue, err = queue.NewDeletionQueue(cfg, redisClient, nil)
			if err != nil {
				logger.Fatal("Failed to initialize deletion queue", zap.Error(err))
				return fmt.Errorf("failed to initialize deletion queue: %w", err)
			}

			intentMgr, err = worker.NewIntentManager(cfg.Cache.Type, redisClient, nil)
			if err != nil {
				logger.Fatal("Failed to initialize intent manager", zap.Error(err))
				return fmt.Errorf("failed to initialize intent manager: %w", err)
			}

		case "badger":
			// Open Badger DB for queue (use separate directory to avoid lock conflicts)
			queueDir := cfg.Cache.Directory + "_queue"
			badgerOpts := badger.DefaultOptions(queueDir).
				WithLogger(nil) // Disable badger's internal logging

			badgerDB, err := badger.Open(badgerOpts)
			if err != nil {
				logger.Fatal("Failed to open Badger DB for queue", zap.Error(err))
				return fmt.Errorf("failed to open badger DB: %w", err)
			}
			defer badgerDB.Close()

			deletionQueue, err = queue.NewDeletionQueue(cfg, nil, badgerDB)
			if err != nil {
				logger.Fatal("Failed to initialize deletion queue", zap.Error(err))
				return fmt.Errorf("failed to initialize deletion queue: %w", err)
			}

			intentMgr, err = worker.NewIntentManager(cfg.Cache.Type, nil, badgerDB)
			if err != nil {
				logger.Fatal("Failed to initialize intent manager", zap.Error(err))
				return fmt.Errorf("failed to initialize intent manager: %w", err)
			}

		default:
			return fmt.Errorf("unsupported cache type: %s", cfg.Cache.Type)
		}

		// Set deletion queue and intent manager on image service (cast to implementation)
		if imgSvc, ok := imageService.(*service.ImageServiceImpl); ok {
			imgSvc.SetDeletionQueue(deletionQueue, intentMgr)
		} else {
			return fmt.Errorf("failed to cast image service to implementation type")
		}

		// Create worker pool config
		poolConfig := &worker.WorkerPoolConfig{
			WorkerCount: cfg.Deletion.WorkerCount,
			WorkerConfig: worker.WorkerConfig{
				MaxRetries:      cfg.Deletion.MaxRetries,
				RetryBackoff:    cfg.Deletion.RetryBackoff,
				DeletionTimeout: cfg.Deletion.DeletionTimeout,
			},
			HealthCheckInterval: cfg.Deletion.HealthCheck,
			ShutdownTimeout:     cfg.Deletion.ShutdownTimeout,
		}

		// Create worker pool
		workerPool = worker.NewWorkerPool(
			deletionQueue,
			store,
			intentMgr,
			poolConfig,
		)

		// Start worker pool
		workerCtx := context.Background()
		if err := workerPool.Start(workerCtx); err != nil {
			logger.Fatal("Failed to start worker pool", zap.Error(err))
			return fmt.Errorf("failed to start worker pool: %w", err)
		}

		logger.Info("Deletion workers started successfully",
			zap.Int("worker_count", cfg.Deletion.WorkerCount))
	} else {
		logger.Info("Async deletion disabled - using synchronous deletion")
	}

	// Initialize API router
	logger.Info("Initializing API router...")
	router := api.NewRouter(cfg, imageService, healthService, statisticsService)

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
	return waitForShutdown(server, workerPool, serverErrChan)
}

// waitForShutdown waits for shutdown signal and gracefully shuts down the server
func waitForShutdown(server *http.Server, workerPool *worker.WorkerPool, serverErrChan chan error) error {
	// Channel to listen for interrupt signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrChan:
		return err
	case sig := <-quit:
		logger.Info("Received shutdown signal, starting graceful shutdown...",
			zap.String("signal", sig.String()))

		return gracefulShutdown(server, workerPool)
	}
}

// gracefulShutdown performs graceful shutdown of the server
func gracefulShutdown(server *http.Server, workerPool *worker.WorkerPool) error {
	// Create context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
	defer cancel()

	// Shutdown worker pool first (if exists) to finish processing deletion tasks
	if workerPool != nil {
		logger.Info("Shutting down deletion worker pool...",
			zap.Duration("timeout", ShutdownTimeout))

		if err := workerPool.Shutdown(ShutdownTimeout); err != nil {
			logger.Warn("Worker pool shutdown encountered errors", zap.Error(err))
			// Continue with server shutdown even if workers had issues
		} else {
			logger.Info("Worker pool shut down successfully")
		}
	}

	// Attempt graceful shutdown of HTTP server
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
