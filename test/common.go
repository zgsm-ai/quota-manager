package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
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

// testClearData test clear data - unified data clearing for all test modules
func testClearData(ctx *TestContext) TestResult {
	// Clear quota-related tables from main database
	quotaTables := []string{"voucher_redemption", "quota_audit", "quota", "quota_execute", "quota_strategy"}
	for _, table := range quotaTables {
		if err := ctx.DB.DB.Exec("DELETE FROM " + table).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Clear table %s failed: %v", table, err)}
		}
		// Reset sequence for tables with SERIAL PRIMARY KEY
		if err := ctx.DB.DB.Exec("SELECT setval('" + table + "_id_seq', 1, false)").Error; err != nil {
			// Ignore error if sequence doesn't exist (for tables without SERIAL PRIMARY KEY)
			fmt.Printf("Warning: Failed to reset sequence for table %s: %v\n", table, err)
		}
	}

	// Clear permission-related tables from main database
	permissionTables := []string{"permission_audit", "effective_quota_check_settings", "quota_check_settings", "effective_star_check_settings", "star_check_settings", "effective_permissions", "model_whitelist", "employee_department"}
	for _, table := range permissionTables {
		if err := ctx.DB.DB.Exec("DELETE FROM " + table).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Clear table %s failed: %v", table, err)}
		}
		// Reset sequence for tables with SERIAL PRIMARY KEY
		if err := ctx.DB.DB.Exec("SELECT setval('" + table + "_id_seq', 1, false)").Error; err != nil {
			// Ignore error if sequence doesn't exist (for tables without SERIAL PRIMARY KEY)
			fmt.Printf("Warning: Failed to reset sequence for table %s: %v\n", table, err)
		}
	}

	// Clear auth_users table from auth database
	if err := ctx.DB.AuthDB.Exec("DELETE FROM auth_users").Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Clear table auth_users failed: %v", err)}
	}

	// Log mock store state before clearing
	fmt.Printf("[DEBUG] testClearData: Before clearing - used delta calls count: %d\n", len(mockStore.usedDeltaCalls))
	for i, call := range mockStore.usedDeltaCalls {
		fmt.Printf("[DEBUG] testClearData: Before clearing - call %d: EmployeeNumber=%s, Delta=%f\n", i, call.EmployeeNumber, call.Delta)
	}

	// Reset mock storage
	mockStore.data = make(map[string]float64)
	mockStore.usedData = make(map[string]float64)
	mockStore.starData = make(map[string]bool)
	mockStore.ClearSetStarProjectsCalls()
	mockStore.ClearAllPermissions()
	mockStore.ClearPermissionCalls()
	mockStore.ClearStarCheckCalls()
	mockStore.ClearQuotaCheckCalls()
	mockStore.ClearUsedDeltaCalls()

	// Log mock store state after clearing
	fmt.Printf("[DEBUG] testClearData: After clearing - used delta calls count: %d\n", len(mockStore.usedDeltaCalls))

	return TestResult{Passed: true, Message: "All data cleared successfully (quota + permission + auth)"}
}

// clearPermissionData clears permission-related data for test isolation
func clearPermissionData(ctx *TestContext) error {
	// Clear permission-related tables in the correct order (to avoid foreign key constraints)
	permissionTables := []string{"permission_audit", "effective_quota_check_settings", "quota_check_settings", "effective_star_check_settings", "star_check_settings", "effective_permissions", "model_whitelist", "employee_department"}
	for _, table := range permissionTables {
		if err := ctx.DB.DB.Exec("DELETE FROM " + table).Error; err != nil {
			return fmt.Errorf("failed to clear table %s: %w", table, err)
		}
		// Reset sequence for tables with SERIAL PRIMARY KEY
		if err := ctx.DB.DB.Exec("ALTER SEQUENCE " + table + "_id_seq RESTART WITH 1").Error; err != nil {
			// Ignore error if sequence doesn't exist (for tables without SERIAL PRIMARY KEY)
			fmt.Printf("Warning: Failed to reset sequence for table %s: %v\n", table, err)
		}
	}

	// Clear mock permission calls
	mockStore.ClearPermissionCalls()
	mockStore.ClearStarCheckCalls()
	mockStore.ClearQuotaCheckCalls()

	return nil
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
	if err := db.DB.AutoMigrate(&models.QuotaStrategy{}, &models.QuotaExecute{}, &models.Quota{}, &models.QuotaAudit{}, &models.VoucherRedemption{}, &models.MonthlyQuotaUsage{}); err != nil {
		return nil, fmt.Errorf("failed to migrate main tables: %w", err)
	}

	// Drop and recreate permission tables to avoid type migration issues
	// permissionTables := []string{"permission_audit", "effective_permissions", "model_whitelist", "employee_department"}
	// for _, table := range permissionTables {
	// 	if err := db.DB.Exec("DROP TABLE IF EXISTS " + table + " CASCADE").Error; err != nil {
	// 		return nil, fmt.Errorf("failed to drop table %s: %w", table, err)
	// 	}
	// }

	// Auto migrate permission tables (will create them fresh)
	if err := db.DB.AutoMigrate(&models.EmployeeDepartment{}, &models.ModelWhitelist{}, &models.EffectivePermission{}, &models.PermissionAudit{}, &models.StarCheckSetting{}, &models.EffectiveStarCheckSetting{}, &models.QuotaCheckSetting{}, &models.EffectiveQuotaCheckSetting{}); err != nil {
		return nil, fmt.Errorf("failed to migrate permission tables: %w", err)
	}

	// Auto migrate auth tables
	if err := db.AuthDB.AutoMigrate(&models.UserInfo{}); err != nil {
		return nil, fmt.Errorf("failed to migrate auth tables: %w", err)
	}

	// Create successful mock server
	mockServer := createMockServer(false)

	// Create failure mock server
	failServer := createMockServer(true)

	// Parse mock server URL to get host and port
	mockURL, err := url.Parse(mockServer.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse mock server URL: %w", err)
	}

	// Extract host and port from the URL
	mockHost := mockURL.Hostname()
	mockPort := mockURL.Port()
	if mockPort == "" {
		mockPort = "80" // Default HTTP port
	}

	mockPortInt := 80
	if port, err := strconv.Atoi(mockPort); err == nil {
		mockPortInt = port
	}

	// Create AiGateway client with mock server URL
	gateway := aigateway.NewClient(mockServer.URL, "/v1/chat/completions/quota", "x-admin-key", "12345678")

	// Create mock AiGateway config for QuotaService using actual mock server host/port
	mockAiGatewayConfig := &config.AiGatewayConfig{
		Host:       mockHost,
		Port:       mockPortInt,
		AdminPath:  "/v1/chat/completions/quota",
		AuthHeader: "x-admin-key",
		AuthValue:  "12345678",
	}

	// Create services
	voucherService := services.NewVoucherService("test-signing-key-at-least-32-bytes-long")
	cfg.AiGateway = *mockAiGatewayConfig
	// Create a config manager for the quota service
	configManager := config.NewManager(cfg)
	quotaService := services.NewQuotaService(db, configManager, gateway, voucherService)
	// Default employee sync config for testing (disabled by default)
	defaultEmployeeSyncConfig := &config.EmployeeSyncConfig{
		Enabled: false,
	}
	strategyService := services.NewStrategyService(db, gateway, quotaService, defaultEmployeeSyncConfig)
	quotaQuerier := condition.NewAiGatewayQuotaQuerier(gateway)

	// Initialize permission-related services and set config manager for mapping logic in tests if needed
	// Note: Many tests call services directly; mapping is only active when EmployeeSync.Enabled is true via configManager
	return &TestContext{
		DB:              db,
		StrategyService: strategyService,
		QuotaService:    quotaService,
		VoucherService:  voucherService,
		Gateway:         gateway,
		MockServer:      mockServer,
		FailServer:      failServer,
		quotaQuerier:    quotaQuerier,
		MockQuotaStore:  mockStore,
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

// createAuthUserForEmployee 在 auth 库中创建一条用户记录，建立 UUID -> 员工号 的映射
// 返回生成的 user_id (UUID)
func createAuthUserForEmployee(ctx *TestContext, employeeNumber, name string) (string, error) {
	userID := uuid.NewString()
	authUser := &models.UserInfo{
		ID:             userID,
		Name:           name,
		EmployeeNumber: employeeNumber,
		GithubID:       fmt.Sprintf("test_%s_%d", employeeNumber, time.Now().UnixNano()),
		GithubName:     name,
		Devices:        "{}",
	}
	if err := ctx.DB.AuthDB.Create(authUser).Error; err != nil {
		return "", fmt.Errorf("failed to create auth user for %s: %w", employeeNumber, err)
	}
	return userID, nil
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

// UseFailServer 切换到 failure server 并返回恢复函数
// 使用方法：
// restoreFunc := ctx.UseFailServer()
// defer restoreFunc()
func (ctx *TestContext) UseFailServer() (restoreFunc func()) {
	// 保存原始状态
	originalBaseURL := ctx.Gateway.BaseURL
	originalConfig := ctx.QuotaService.GetConfigManager().Get()

	// 切换到 failure server
	ctx.Gateway.BaseURL = ctx.FailServer.URL

	// 自动解析和更新配置
	failURL, err := url.Parse(ctx.FailServer.URL)
	if err != nil {
		// 如果解析失败，直接返回空恢复函数
		return func() {}
	}

	failHost := failURL.Hostname()
	failPort := failURL.Port()
	if failPort == "" {
		failPort = "80"
	}

	failPortInt := 80
	if port, err := strconv.Atoi(failPort); err == nil {
		failPortInt = port
	}

	// 更新配置
	ctx.QuotaService.GetConfigManager().Update(func(cfg *config.Config) {
		cfg.AiGateway.Host = failHost
		cfg.AiGateway.Port = failPortInt
	})

	// 创建新的 HTTP 客户端
	ctx.Gateway.HTTPClient = &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}

	// 返回恢复函数
	return func() {
		ctx.Gateway.BaseURL = originalBaseURL
		ctx.QuotaService.GetConfigManager().Update(func(cfg *config.Config) {
			cfg.AiGateway.Host = originalConfig.AiGateway.Host
			cfg.AiGateway.Port = originalConfig.AiGateway.Port
		})
	}
}

// createStrategyServiceWithEmployeeSync creates a StrategyService with custom employee sync configuration for testing
func (ctx *TestContext) createStrategyServiceWithEmployeeSync(employeeSyncConfig *config.EmployeeSyncConfig) *services.StrategyService {
	return services.NewStrategyService(ctx.DB, ctx.Gateway, ctx.QuotaService, employeeSyncConfig)
}

// createTestInviterUser creates a test user with new auth_users table structure
func createTestInviterUser(id, name string, vip int, inviterID string) *models.UserInfo {
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
		InviterID:        inviterID,
	}
}
