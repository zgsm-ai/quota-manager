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
	"quota-manager/internal/response"
	"quota-manager/internal/services"
	"quota-manager/pkg/aigateway"
	"quota-manager/pkg/logger"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func setTimezone(timezone string) {
	if timezone == "" {
		timezone = "Asia/Shanghai" // Default to Beijing Time
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		log.Printf("Warning: Failed to load timezone %s: %v, using UTC", timezone, err)
	} else {
		time.Local = loc
		log.Printf("Timezone set to %s", timezone)
	}
}

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
			fmt.Printf("Specified configuration file not found: %s\n", configFile)
			os.Exit(1)
		}
		fmt.Printf("Using specified config: %s\n", configFile)
	}

	// Load configuration
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Set timezone from configuration
	setTimezone(cfg.Server.Timezone)

	// Initialize logging with configured level
	logLevel := cfg.Log.Level
	if logLevel == "" {
		logLevel = "warn" // Default level if not configured
	}
	if err := logger.InitWithLevel(logLevel); err != nil {
		fmt.Printf("Failed to initialize logger with level %s: %v\n", logLevel, err)
		os.Exit(1)
	}

	// Initialize database
	db, err := database.NewDB(cfg)
	if err != nil {
		logger.Error("Failed to connect database", zap.Error(err))
		os.Exit(1)
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
	quotaService := services.NewQuotaService(db, cfg, gateway, voucherService)
	strategyService := services.NewStrategyService(db, gateway, quotaService, &cfg.EmployeeSync)

	// Initialize permission management services
	permissionService := services.NewPermissionService(db, &cfg.AiGateway, &cfg.EmployeeSync, gateway)
	starCheckPermissionService := services.NewStarCheckPermissionService(db, &cfg.AiGateway, &cfg.EmployeeSync, gateway)
	quotaCheckPermissionService := services.NewQuotaCheckPermissionService(db, &cfg.AiGateway, &cfg.EmployeeSync, gateway)
	unifiedPermissionService := services.NewUnifiedPermissionService(permissionService, starCheckPermissionService, quotaCheckPermissionService, nil) // employeeSyncService will be set later
	employeeSyncService := services.NewEmployeeSyncService(db, &cfg.EmployeeSync, permissionService, starCheckPermissionService, quotaCheckPermissionService)

	// Update unified permission service with employee sync service
	unifiedPermissionService = services.NewUnifiedPermissionService(permissionService, starCheckPermissionService, quotaCheckPermissionService, employeeSyncService)

	schedulerService := services.NewSchedulerService(quotaService, strategyService, employeeSyncService, cfg)

	// Start scheduler service (includes strategy scan and employee sync)
	if err := schedulerService.Start(); err != nil {
		logger.Error("Failed to start scheduler service", zap.Error(err))
		os.Exit(1)
	}
	defer schedulerService.Stop()

	// Trigger initial employee sync if employee_department table is empty
	if err := employeeSyncService.TriggerInitialSyncIfNeeded(); err != nil {
		logger.Error("Failed to trigger initial employee sync", zap.Error(err))
		// Log error but don't exit, let the service continue running
	}

	// Initialize HTTP handlers
	strategyHandler := handlers.NewStrategyHandler(strategyService)
	quotaHandler := handlers.NewQuotaHandler(quotaService, &cfg.Server)
	modelPermissionHandler := handlers.NewModelPermissionHandler(permissionService)
	starCheckPermissionHandler := handlers.NewStarCheckPermissionHandler(starCheckPermissionService)
	quotaCheckPermissionHandler := handlers.NewQuotaCheckPermissionHandler(quotaCheckPermissionService)
	unifiedPermissionHandler := handlers.NewUnifiedPermissionHandler(unifiedPermissionService)

	// Set Gin mode
	gin.SetMode(cfg.Server.Mode)

	// Create routes
	router := gin.Default()

	// Configure CORS
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = []string{"*"}
	corsConfig.AllowCredentials = false
	corsConfig.AllowHeaders = []string{
		"Origin",
		"Content-Length",
		"Content-Type",
		"Authorization",
		"Accept",
		"Cache-Control",
		"X-Requested-With",
	}
	corsConfig.AllowMethods = []string{
		"GET",
		"POST",
		"PUT",
		"DELETE",
		"OPTIONS",
		"PATCH",
	}
	router.Use(cors.New(corsConfig))

	// Wrap all routes under /quota-manager prefix
	quotaManager := router.Group("/quota-manager")
	{
		// Health check
		quotaManager.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, response.NewSuccessResponse(gin.H{"status": "ok"}, "Service is running"))
		})

		// API routes
		v1 := quotaManager.Group("/api/v1")
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

				// Strategy execution records
				strategies.GET("/:id/executions", strategyHandler.GetStrategyExecuteRecords)
			}

			// Quota management API
			handlers.RegisterQuotaRoutes(v1, quotaHandler)

			// Model permissions management
			modelPermissions := v1.Group("/model-permissions")
			{
				modelPermissions.POST("/user", modelPermissionHandler.SetUserWhitelist)
				modelPermissions.POST("/department", modelPermissionHandler.SetDepartmentWhitelist)
			}

			// Star check permissions management
			starCheckPermissions := v1.Group("/star-check-permissions")
			{
				starCheckPermissions.POST("/user", starCheckPermissionHandler.SetUserStarCheckSetting)
				starCheckPermissions.POST("/department", starCheckPermissionHandler.SetDepartmentStarCheckSetting)
			}

			// Quota check permissions management
			quotaCheckPermissions := v1.Group("/quota-check-permissions")
			{
				quotaCheckPermissions.POST("/user", quotaCheckPermissionHandler.SetUserQuotaCheckSetting)
				quotaCheckPermissions.POST("/department", quotaCheckPermissionHandler.SetDepartmentQuotaCheckSetting)
			}

			// Unified query and sync interfaces
			v1.GET("/effective-permissions", unifiedPermissionHandler.GetEffectivePermissions)
			v1.POST("/employee-sync", unifiedPermissionHandler.TriggerEmployeeSync)
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

	// Start server with single console message
	logger.Info("Starting server", zap.Int("port", cfg.Server.Port), zap.String("config", configFile))
	fmt.Printf("Server starting on port %d\n", cfg.Server.Port)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("Failed to start server", zap.Error(err))
		os.Exit(1)
	}
}
