package main

import (
	"fmt"
	"net"
	"net/url"
	"strconv"

	"quota-manager/internal/config"
	"quota-manager/internal/database"
	"quota-manager/internal/models"
	"quota-manager/internal/services"
	"quota-manager/pkg/aigateway"
	"quota-manager/pkg/logger"
)

// testClearData test clear data
func testClearData(ctx *TestContext) TestResult {
	// Clear all tables
	tables := []string{"voucher_redemption", "quota_audit", "quota", "quota_execute", "quota_strategy", "user_info"}
	for _, table := range tables {
		if err := ctx.DB.Exec("DELETE FROM " + table).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Clear table %s failed: %v", table, err)}
		}
	}

	// Reset mock storage
	mockStore.data = make(map[string]int)

	return TestResult{Passed: true, Message: "Data cleared successfully"}
}

// printTestResults print test results
func printTestResults(results []TestResult) {
	fmt.Println("\n=== Test Results ===")
	passed := 0
	failed := 0
	for _, result := range results {
		if result.Passed {
			passed++
			fmt.Printf("✅ %s - 通过 (%.2fs)\n", result.TestName, result.Duration.Seconds())
		} else {
			failed++
			fmt.Printf("❌ %s - 失败: %s (%.2fs)\n", result.TestName, result.Message, result.Duration.Seconds())
		}
	}
	fmt.Printf("\n总计: %d 个测试, %d 通过, %d 失败\n", len(results), passed, failed)
}

// setupTestEnvironment setup test environment
func setupTestEnvironment() (*TestContext, error) {
	// Initialize logger
	logger.Init()

	// Load configuration
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Connect to database
	db, err := database.NewDB(&cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	// Auto migrate
	if err := models.AutoMigrate(db.DB); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	// Create successful mock server
	mockServer := createMockServer(false)

	// Create failure mock server
	failServer := createMockServer(true)

	// Create AiGateway client with mock server URL
	gateway := aigateway.NewClient(mockServer.URL, "/v1/chat/completions", "credential3")

	// Create mock AiGateway config for QuotaService
	mockAiGatewayConfig := &config.AiGatewayConfig{
		Host:       "127.0.0.1", // This will be overridden by the URL parsing
		Port:       8080,        // This will be overridden by the URL parsing
		AdminPath:  "/v1/chat/completions",
		Credential: "credential3",
	}

	// Override the BaseURL method behavior by setting the host and port from mockServer URL
	// Parse the mock server URL to get host and port
	parsedURL, err := url.Parse(mockServer.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse mock server URL: %w", err)
	}
	host, portStr, err := net.SplitHostPort(parsedURL.Host)
	if err != nil {
		return nil, fmt.Errorf("failed to split host and port: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse port: %w", err)
	}
	mockAiGatewayConfig.Host = host
	mockAiGatewayConfig.Port = port

	// Create services
	voucherService := services.NewVoucherService("test-signing-key-at-least-32-bytes-long")
	quotaService := services.NewQuotaService(db.DB, mockAiGatewayConfig, voucherService)
	strategyService := services.NewStrategyService(db, gateway, quotaService)

	return &TestContext{
		DB:              db,
		StrategyService: strategyService,
		QuotaService:    quotaService,
		VoucherService:  voucherService,
		Gateway:         gateway,
		MockServer:      mockServer,
		FailServer:      failServer,
	}, nil
}

// cleanupTestEnvironment cleanup test environment
func cleanupTestEnvironment(ctx *TestContext) {
	if ctx.MockServer != nil {
		ctx.MockServer.Close()
	}
	if ctx.FailServer != nil {
		ctx.FailServer.Close()
	}
}
