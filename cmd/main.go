package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"quota-manager/internal/config"
	"quota-manager/internal/database"
	"quota-manager/internal/handlers"
	"quota-manager/internal/services"
	"quota-manager/pkg/aigateway"
	"quota-manager/pkg/logger"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	// Parse command line flags FIRST - before any other initialization
	var configFile string
	var showHelp bool

	flag.StringVar(&configFile, "config", "", "Path to the configuration file")
	flag.StringVar(&configFile, "c", "", "Path to the configuration file (shorthand)")
	flag.BoolVar(&showHelp, "help", false, "Show help message")
	flag.BoolVar(&showHelp, "h", false, "Show help message (shorthand)")

	flag.Parse()

	// Show help message
	if showHelp {
		fmt.Println("Quota Manager - Quota Management Service")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Printf("  %s [options]\n", os.Args[0])
		fmt.Println()
		fmt.Println("Options:")
		flag.PrintDefaults()
		return
	}

	// Determine config file to use
	if configFile == "" {
		// Default config file selection logic
		if _, err := os.Stat("config_local.yaml"); err == nil {
			configFile = "config_local.yaml"
			fmt.Println("Using local config: config_local.yaml")
		} else {
			configFile = "config.yaml"
			fmt.Println("Using default config: config.yaml")
		}
	} else {
		// Check if the specified configuration file exists
		if _, err := os.Stat(configFile); os.IsNotExist(err) {
			log.Fatalf("Specified configuration file not found: %s", configFile)
		}
		fmt.Printf("Using specified config: %s\n", configFile)
	}

	// Initialize logging AFTER config file is determined
	if err := logger.Init(); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	// Load configuration
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		logger.Error("Failed to load config", zap.Error(err))
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	db, err := database.NewDB(cfg)
	if err != nil {
		logger.Error("Failed to connect database", zap.Error(err))
		log.Fatalf("Failed to connect database: %v", err)
	}
	defer db.Close()

	// Initialize AiGateway client
	gateway := aigateway.NewClient(
		cfg.AiGateway.GetBaseURL(),
		cfg.AiGateway.AdminPath,
		cfg.AiGateway.AuthHeader,
		cfg.AiGateway.AuthValue,
	)

	// Initialize services
	voucherService := services.NewVoucherService(cfg.Voucher.SigningKey)
	quotaService := services.NewQuotaService(db.DB, &cfg.AiGateway, voucherService)
	strategyService := services.NewStrategyService(db, gateway, quotaService)
	schedulerService := services.NewSchedulerService(quotaService, strategyService, cfg)

	// Start scheduler service (includes strategy scan)
	if err := schedulerService.Start(); err != nil {
		logger.Error("Failed to start scheduler service", zap.Error(err))
		log.Fatalf("Failed to start scheduler service: %v", err)
	}
	defer schedulerService.Stop()

	// Initialize HTTP handlers
	strategyHandler := handlers.NewStrategyHandler(strategyService)
	quotaHandler := handlers.NewQuotaHandler(quotaService, &cfg.Server)

	// Set Gin mode
	gin.SetMode(cfg.Server.Mode)

	// Create routes
	router := gin.Default()

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// API routes
	v1 := router.Group("/api/v1")
	{
		// Strategy management API
		strategies := v1.Group("/strategies")
		{
			strategies.POST("", strategyHandler.CreateStrategy)
			strategies.GET("", strategyHandler.GetStrategies)
			strategies.GET("/:id", strategyHandler.GetStrategy)
			strategies.PUT("/:id", strategyHandler.UpdateStrategy)
			strategies.DELETE("/:id", strategyHandler.DeleteStrategy)

			// Strategy status management
			strategies.POST("/:id/enable", strategyHandler.EnableStrategy)
			strategies.POST("/:id/disable", strategyHandler.DisableStrategy)

			// Manually trigger strategy scan
			strategies.POST("/scan", strategyHandler.TriggerScan)
		}

		// Quota management API
		handlers.RegisterQuotaRoutes(v1, quotaHandler)
	}

	// Start HTTP server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: router,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		logger.Info("Shutting down server...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			logger.Error("Server forced to shutdown", zap.Error(err))
		}
	}()

	logger.Info("Starting server", zap.Int("port", cfg.Server.Port), zap.String("config", configFile))
	fmt.Printf("Server starting on port %d with config: %s\n", cfg.Server.Port, configFile)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("Failed to start server", zap.Error(err))
		log.Fatalf("Failed to start server: %v", err)
	}
}
