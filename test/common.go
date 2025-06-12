package main

import (
	"fmt"
	"strings"
	"time"

	"quota-manager/internal/condition"
	"quota-manager/internal/config"
	"quota-manager/internal/database"
	"quota-manager/internal/models"
	"quota-manager/internal/services"
	"quota-manager/pkg/aigateway"
	"quota-manager/pkg/logger"

	"github.com/google/uuid"
)

// testClearData test clear data
func testClearData(ctx *TestContext) TestResult {
	// Clear quota-related tables from main database
	quotaTables := []string{"voucher_redemption", "quota_audit", "quota", "quota_execute", "quota_strategy"}
	for _, table := range quotaTables {
		if err := ctx.DB.DB.Exec("DELETE FROM " + table).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Clear table %s failed: %v", table, err)}
		}
	}

	// Clear auth_users table from auth database
	if err := ctx.DB.AuthDB.Exec("DELETE FROM auth_users").Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Clear table auth_users failed: %v", err)}
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
			fmt.Printf("✅ %s - PASSED (%.2fs)\n", result.TestName, result.Duration.Seconds())
		} else {
			failed++
			fmt.Printf("❌ %s - FAILED: %s (%.2fs)\n", result.TestName, result.Message, result.Duration.Seconds())
		}
	}
	fmt.Printf("\nTotal: %d tests, %d passed, %d failed\n", len(results), passed, failed)
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
	db, err := database.NewDB(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	// Auto migrate - ensure all tables exist in test environment
	if err := db.DB.AutoMigrate(&models.QuotaStrategy{}, &models.QuotaExecute{}, &models.Quota{}, &models.QuotaAudit{}, &models.VoucherRedemption{}); err != nil {
		return nil, fmt.Errorf("failed to migrate main tables: %w", err)
	}

	// Auto migrate auth tables
	if err := db.AuthDB.AutoMigrate(&models.UserInfo{}); err != nil {
		return nil, fmt.Errorf("failed to migrate auth tables: %w", err)
	}

	// Create successful mock server
	mockServer := createMockServer(false)

	// Create failure mock server
	failServer := createMockServer(true)

	// Create AiGateway client with mock server URL
	gateway := aigateway.NewClient(mockServer.URL, "/v1/chat/completions", "X-Auth-Key", "credential3")

	// Create mock AiGateway config for QuotaService
	mockAiGatewayConfig := &config.AiGatewayConfig{
		BaseURL:    mockServer.URL,
		AdminPath:  "/v1/chat/completions",
		AuthHeader: "X-Auth-Key",
		AuthValue:  "credential3",
	}

	// Create services
	voucherService := services.NewVoucherService("test-signing-key-at-least-32-bytes-long")
	quotaService := services.NewQuotaService(db.DB, mockAiGatewayConfig, voucherService)
	strategyService := services.NewStrategyService(db, gateway, quotaService)

	// Create quota querier
	quotaQuerier := condition.NewAiGatewayQuotaQuerier(gateway)

	return &TestContext{
		DB:              db,
		StrategyService: strategyService,
		QuotaService:    quotaService,
		VoucherService:  voucherService,
		Gateway:         gateway,
		MockServer:      mockServer,
		FailServer:      failServer,
		quotaQuerier:    quotaQuerier,
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

// verifyStrategyNameInAudit verifies that audit records contain the correct strategy name
func verifyStrategyNameInAudit(ctx *TestContext, userID, expectedStrategyName string, operationType string) error {
	var auditRecord models.QuotaAudit
	err := ctx.DB.DB.Where("user_id = ? AND operation = ? AND strategy_name = ?",
		userID, operationType, expectedStrategyName).
		Order("create_time DESC").
		First(&auditRecord).Error

	if err != nil {
		return fmt.Errorf("audit record with strategy name '%s' not found: %v", expectedStrategyName, err)
	}

	if auditRecord.StrategyName != expectedStrategyName {
		return fmt.Errorf("strategy name mismatch in audit record, expected: %s, actual: %s",
			expectedStrategyName, auditRecord.StrategyName)
	}

	return nil
}

// verifyNoStrategyNameInAudit verifies that audit records do not contain strategy name for non-recharge operations
func verifyNoStrategyNameInAudit(ctx *TestContext, userID, operationType string) error {
	var auditRecord models.QuotaAudit
	err := ctx.DB.DB.Where("user_id = ? AND operation = ?", userID, operationType).
		Order("create_time DESC").
		First(&auditRecord).Error

	if err != nil {
		return fmt.Errorf("audit record for %s operation not found: %v", operationType, err)
	}

	if auditRecord.StrategyName != "" {
		return fmt.Errorf("audit record for %s operation should not contain strategy name, but actual: %s",
			operationType, auditRecord.StrategyName)
	}

	return nil
}

// verifyAuditRecordCount verifies the total count of audit records for a user
func verifyAuditRecordCount(ctx *TestContext, userID string, expectedCount int64) error {
	var count int64
	if err := ctx.DB.DB.Model(&models.QuotaAudit{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to query audit record count: %v", err)
	}

	if count != expectedCount {
		return fmt.Errorf("audit record count mismatch, expected: %d, actual: %d", expectedCount, count)
	}

	return nil
}

// createTestUser creates a test user with new auth_users table structure
func createTestUser(id, name string, vip int) *models.UserInfo {
	// Generate a valid UUID for the user ID
	validUUID := uuid.NewString()

	// Create a unique github_id by combining the id parameter with a timestamp
	uniqueGithubID := fmt.Sprintf("%s_%d", strings.ToLower(id), time.Now().UnixNano())

	return &models.UserInfo{
		ID:               validUUID,
		CreatedAt:        time.Now().Add(-time.Hour * 24),
		UpdatedAt:        time.Now().Add(-time.Hour * 1),
		AccessTime:       time.Now().Add(-time.Hour * 1),
		Name:             name,
		GithubID:         uniqueGithubID,
		GithubName:       name,
		VIP:              vip,
		Phone:            "13800138000",
		Email:            fmt.Sprintf("%s@test.com", strings.ToLower(name)),
		Password:         "",
		Company:          "TestCompany",
		Location:         "TestCity",
		UserCode:         fmt.Sprintf("TC%s", id),
		ExternalAccounts: "",
		EmployeeNumber:   fmt.Sprintf("EMP%s", id),
		GithubStar:       "zgsm-ai.zgsm,openai.gpt-4",
		Devices:          "{}",
	}
}
