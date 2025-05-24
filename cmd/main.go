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
	"quota-manager/internal/config"
	"quota-manager/internal/database"
	"quota-manager/internal/handlers"
	"quota-manager/internal/services"
	"quota-manager/pkg/aigateway"
	"quota-manager/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	// Initialize logging
	if err := logger.Init(); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	// Load configuration
	configFile := "config.yaml"
	if _, err := os.Stat("config_local.yaml"); err == nil {
		configFile = "config_local.yaml"
		fmt.Println("Using local config: config_local.yaml")
	}

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		logger.Error("Failed to load config", zap.Error(err))
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	db, err := database.NewDB(&cfg.Database)
	if err != nil {
		logger.Error("Failed to connect database", zap.Error(err))
		log.Fatalf("Failed to connect database: %v", err)
	}
	defer db.Close()

	// Initialize AiGateway client
	gateway := aigateway.NewClient(
		cfg.AiGateway.BaseURL(),
		cfg.AiGateway.AdminPath,
		cfg.AiGateway.Credential,
	)

	// Initialize services
	strategyService := services.NewStrategyService(db, gateway)

	// Start strategy scan service
	if err := strategyService.Start(); err != nil {
		logger.Error("Failed to start strategy service", zap.Error(err))
		log.Fatalf("Failed to start strategy service: %v", err)
	}
	defer strategyService.Stop()

	// Initialize HTTP handlers
	strategyHandler := handlers.NewStrategyHandler(strategyService)

	// Set Gin mode
	gin.SetMode(cfg.Server.Mode)

	// Create routes
	router := gin.Default()

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Strategy management API
	v1 := router.Group("/api/v1")
	{
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

	logger.Info("Starting server", zap.Int("port", cfg.Server.Port))
	fmt.Printf("Server starting on port %d\n", cfg.Server.Port)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("Failed to start server", zap.Error(err))
		log.Fatalf("Failed to start server: %v", err)
	}
}
