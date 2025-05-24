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
)

func main() {
	// 初始化日志
	if err := logger.Init(); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	// 加载配置
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		logger.Error("Failed to load config", nil)
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化数据库
	db, err := database.NewDB(&cfg.Database)
	if err != nil {
		logger.Error("Failed to connect database", nil)
		log.Fatalf("Failed to connect database: %v", err)
	}
	defer db.Close()

	// 初始化AiGateway客户端
	gateway := aigateway.NewClient(
		cfg.AiGateway.BaseURL(),
		cfg.AiGateway.AdminPath,
		cfg.AiGateway.Credential,
	)

	// 初始化服务
	strategyService := services.NewStrategyService(db, gateway)

	// 启动策略扫描服务
	if err := strategyService.Start(); err != nil {
		logger.Error("Failed to start strategy service", nil)
		log.Fatalf("Failed to start strategy service: %v", err)
	}
	defer strategyService.Stop()

	// 初始化HTTP处理器
	strategyHandler := handlers.NewStrategyHandler(strategyService)

	// 设置Gin模式
	gin.SetMode(cfg.Server.Mode)

	// 创建路由
	router := gin.Default()

	// 健康检查
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 策略管理API
	v1 := router.Group("/api/v1")
	{
		strategies := v1.Group("/strategies")
		{
			strategies.POST("", strategyHandler.CreateStrategy)
			strategies.GET("", strategyHandler.GetStrategies)
			strategies.GET("/:id", strategyHandler.GetStrategy)
			strategies.PUT("/:id", strategyHandler.UpdateStrategy)
			strategies.DELETE("/:id", strategyHandler.DeleteStrategy)
			strategies.POST("/scan", strategyHandler.TriggerScan)
		}
	}

	// 启动HTTP服务器
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: router,
	}

	// 优雅关闭
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		logger.Info("Shutting down server...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			logger.Error("Server forced to shutdown", nil)
		}
	}()

	logger.Info("Starting server", nil)
	fmt.Printf("Server starting on port %d\n", cfg.Server.Port)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("Failed to start server", nil)
		log.Fatalf("Failed to start server: %v", err)
	}
}
