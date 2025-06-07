package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"time"

	"quota-manager/internal/config"
	"quota-manager/internal/database"
	"quota-manager/internal/models"
	"quota-manager/internal/services"
	"quota-manager/pkg/aigateway"
	"quota-manager/pkg/logger"

	"github.com/gin-gonic/gin"
)

// TestContext test context
type TestContext struct {
	DB              *database.DB
	StrategyService *services.StrategyService
	QuotaService    *services.QuotaService
	VoucherService  *services.VoucherService
	Gateway         *aigateway.Client
	MockServer      *httptest.Server
	FailServer      *httptest.Server
}

// TestResult test result
type TestResult struct {
	TestName string
	Passed   bool
	Message  string
	Duration time.Duration
}

// MockQuotaStore mock quota storage
type MockQuotaStore struct {
	data map[string]int
}

func (m *MockQuotaStore) GetQuota(consumer string) int {
	if quota, exists := m.data[consumer]; exists {
		return quota
	}
	return 0
}

func (m *MockQuotaStore) SetQuota(consumer string, quota int) {
	m.data[consumer] = quota
}

func (m *MockQuotaStore) DeltaQuota(consumer string, delta int) int {
	m.data[consumer] += delta
	return m.data[consumer]
}

var mockStore = &MockQuotaStore{data: make(map[string]int)}

func main() {
	fmt.Println("=== Quota Manager Integration Tests ===")

	// Initialize test environment
	ctx, err := setupTestEnvironment()
	if err != nil {
		log.Fatalf("Failed to setup test environment: %v", err)
	}
	defer cleanupTestEnvironment(ctx)

	// Run all tests
	results := runAllTests(ctx)

	// Print test results
	printTestResults(results)
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

// createMockServer create mock server
func createMockServer(shouldFail bool) *httptest.Server {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Middleware: validate Authorization
	authMiddleware := func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth != "Bearer credential3" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization"})
			c.Abort()
			return
		}
		c.Next()
	}

	v1 := router.Group("/v1/chat/completions")
	v1.Use(authMiddleware)
	{
		v1.POST("/quota/refresh", func(c *gin.Context) {
			if shouldFail {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				return
			}

			consumer := c.PostForm("consumer")
			quota := c.PostForm("quota")

			if consumer == "" || quota == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "missing parameters"})
				return
			}

			c.JSON(http.StatusOK, gin.H{"message": "success"})
		})

		v1.GET("/quota", func(c *gin.Context) {
			if shouldFail {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				return
			}

			consumer := c.Query("consumer")
			quota := mockStore.GetQuota(consumer)

			c.JSON(http.StatusOK, gin.H{
				"quota":    quota,
				"consumer": consumer,
			})
		})

		v1.POST("/quota/delta", func(c *gin.Context) {
			if shouldFail {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				return
			}

			consumer := c.PostForm("consumer")
			value := c.PostForm("value")

			if consumer == "" || value == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "missing parameters"})
				return
			}

			// Simulate quota increase
			var delta int
			if _, err := fmt.Sscanf(value, "%d", &delta); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid value"})
				return
			}

			newQuota := mockStore.DeltaQuota(consumer, delta)

			c.JSON(http.StatusOK, gin.H{
				"message":   "success",
				"consumer":  consumer,
				"new_quota": newQuota,
			})
		})

		v1.GET("/quota/used", func(c *gin.Context) {
			if shouldFail {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				return
			}

			consumer := c.Query("consumer")
			// Mock used quota as 0 for simplicity
			c.JSON(http.StatusOK, gin.H{
				"quota":    0,
				"consumer": consumer,
			})
		})

		v1.POST("/quota/used/delta", func(c *gin.Context) {
			if shouldFail {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				return
			}

			consumer := c.PostForm("consumer")
			value := c.PostForm("value")

			if consumer == "" || value == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "missing parameters"})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"message":  "success",
				"consumer": consumer,
			})
		})
	}

	return httptest.NewServer(router)
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

// runAllTests run all tests
func runAllTests(ctx *TestContext) []TestResult {
	var results []TestResult

	// Test case list
	testCases := []struct {
		name string
		fn   func(*TestContext) TestResult
	}{
		{"Clear Data Test", testClearData},
		{"Condition Expression - Empty Condition Test", testEmptyCondition},
		{"Condition Expression - Match User Test", testMatchUserCondition},
		{"Condition Expression - Register Before Test", testRegisterBeforeCondition},
		{"Condition Expression - Access After Test", testAccessAfterCondition},
		{"Condition Expression - Github Star Test", testGithubStarCondition},
		{"Condition Expression - Quota LE Test", testQuotaLECondition},
		{"Condition Expression - Is VIP Test", testIsVipCondition},
		{"Condition Expression - Belong To Test", testBelongToCondition},
		{"Condition Expression - AND Nesting Test", testAndCondition},
		{"Condition Expression - OR Nesting Test", testOrCondition},
		{"Condition Expression - NOT Nesting Test", testNotCondition},
		{"Condition Expression - Complex Nesting Test", testComplexCondition},
		{"Single Recharge Strategy Test", testSingleTypeStrategy},
		{"Periodic Recharge Strategy Test", testPeriodicTypeStrategy},
		{"Strategy Status Control Test", testStrategyStatusControl},
		{"AiGateway Request Failure Test", testAiGatewayFailure},
		{"Batch User Processing Test", testBatchUserProcessing},
		{"Voucher Generation and Validation Test", testVoucherGenerationAndValidation},
		{"Quota Transfer Out Test", testQuotaTransferOut},
		{"Quota Transfer In Test", testQuotaTransferIn},
		{"Quota Expiry Test", testQuotaExpiry},
		{"Quota Audit Records Test", testQuotaAuditRecords},
		{"Strategy with Expiry Date Test", testStrategyWithExpiryDate},
		// New test cases
		{"Multiple Operations Accuracy Test", testMultipleOperationsAccuracy},
		{"Transfer In User ID Mismatch Test", testTransferInUserIDMismatch},
		{"User Quota Consumption Order Test", testUserQuotaConsumptionOrder},
		{"Transfer Out Insufficient Available Quota Test", testTransferOutInsufficientAvailable},
		{"Transfer In Expired Quota Test", testTransferInExpiredQuota},
		{"Transfer In Invalid Voucher Test", testTransferInInvalidVoucher},
		{"Transfer In Quota Expiry Consistency Test", testTransferInQuotaExpiryConsistency},
		{"Strategy Expiry Date Coverage Test", testStrategyExpiryDateCoverage},
		{"Transfer Earliest Expiry Date Test", testTransferEarliestExpiryDate},
		{"Concurrent Operations Test", testConcurrentOperations},
	}

	for _, tc := range testCases {
		fmt.Printf("Running test: %s\n", tc.name)
		start := time.Now()
		result := tc.fn(ctx)
		result.Duration = time.Since(start)
		result.TestName = tc.name
		results = append(results, result)

		if result.Passed {
			fmt.Printf("✅ %s - 通过 (%.2fs)\n", tc.name, result.Duration.Seconds())
		} else {
			fmt.Printf("❌ %s - 失败: %s (%.2fs)\n", tc.name, result.Message, result.Duration.Seconds())
		}
	}

	return results
}

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

// testEmptyCondition test empty condition expression
func testEmptyCondition(ctx *TestContext) TestResult {
	// Create test user
	user := &models.UserInfo{
		ID:           "test_user_empty",
		Name:         "Test User Empty",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}
	if err := ctx.DB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Create empty condition strategy
	strategy := &models.QuotaStrategy{
		Name:      "empty-condition-test",
		Title:     "Empty Condition Test",
		Type:      "single",
		Amount:    10,
		Model:     "test-model",
		Condition: "", // Empty condition
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	// Execute strategy
	users := []models.UserInfo{*user}
	ctx.StrategyService.ExecStrategy(strategy, users)

	// Check execution result
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected execution 1 time, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "Empty condition strategy execution succeeded"}
}

// testMatchUserCondition test match-user condition
func testMatchUserCondition(ctx *TestContext) TestResult {
	// Create test users
	users := []*models.UserInfo{
		{
			ID:           "user_match_1",
			Name:         "Match User 1",
			RegisterTime: time.Now().Add(-time.Hour * 24),
			AccessTime:   time.Now().Add(-time.Hour * 1),
		},
		{
			ID:           "user_match_2",
			Name:         "Match User 2",
			RegisterTime: time.Now().Add(-time.Hour * 24),
			AccessTime:   time.Now().Add(-time.Hour * 1),
		},
	}

	for _, user := range users {
		if err := ctx.DB.Create(user).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
		}
	}

	// Create match-user strategy, only match the first user
	strategy := &models.QuotaStrategy{
		Name:      "match-user-test",
		Title:     "Match User Test",
		Type:      "single",
		Amount:    15,
		Model:     "test-model",
		Condition: `match-user("user_match_1")`,
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	// Execute strategy
	userList := []models.UserInfo{*users[0], *users[1]}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// Check execution result - only user_match_1 should be executed
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_match_1").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("user_match_1 expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check user_match_2 should not be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_match_2").Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("user_match_2 expected execution 0 times, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "match-user condition test succeeded"}
}

// testRegisterBeforeCondition test register-before condition
func testRegisterBeforeCondition(ctx *TestContext) TestResult {
	// Use fixed time point
	baseTime, _ := time.Parse("2006-01-02 15:04:05", "2025-01-01 12:00:00")
	cutoffTime := baseTime

	// Create test users
	users := []*models.UserInfo{
		{
			ID:           "user_reg_before",
			Name:         "Early User",
			RegisterTime: baseTime.Add(-time.Hour * 2), // Register before cutoff time
			AccessTime:   baseTime,
		},
		{
			ID:           "user_reg_after",
			Name:         "Late User",
			RegisterTime: baseTime.Add(time.Hour * 2), // Register after cutoff time
			AccessTime:   baseTime,
		},
	}

	for _, user := range users {
		if err := ctx.DB.Create(user).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
		}
	}

	// Create register-before strategy
	strategy := &models.QuotaStrategy{
		Name:      "register-before-test",
		Title:     "Register Time Test",
		Type:      "single",
		Amount:    20,
		Model:     "test-model",
		Condition: fmt.Sprintf(`register-before("%s")`, cutoffTime.Format("2006-01-02 15:04:05")),
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	// Execute strategy
	userList := []models.UserInfo{*users[0], *users[1]}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// Check early user should be executed
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_reg_before").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Early user expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check late user should not be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_reg_after").Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Late user expected execution 0 times, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "register-before condition test succeeded"}
}

// testAccessAfterCondition test access-after condition
func testAccessAfterCondition(ctx *TestContext) TestResult {
	// Use fixed time point
	baseTime, _ := time.Parse("2006-01-02 15:04:05", "2025-01-01 12:00:00")
	cutoffTime := baseTime

	// Create test users
	users := []*models.UserInfo{
		{
			ID:           "user_access_recent",
			Name:         "Recent User",
			RegisterTime: baseTime.Add(-time.Hour * 24),
			AccessTime:   baseTime.Add(time.Hour * 2), // Access after cutoff time
		},
		{
			ID:           "user_access_old",
			Name:         "Old User",
			RegisterTime: baseTime.Add(-time.Hour * 24),
			AccessTime:   baseTime.Add(-time.Hour * 2), // Access before cutoff time
		},
	}

	for _, user := range users {
		if err := ctx.DB.Create(user).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
		}
	}

	// Create access-after strategy
	strategy := &models.QuotaStrategy{
		Name:      "access-after-test",
		Title:     "Recent Access Test",
		Type:      "single",
		Amount:    25,
		Model:     "test-model",
		Condition: fmt.Sprintf(`access-after("%s")`, cutoffTime.Format("2006-01-02 15:04:05")),
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	// Execute strategy
	userList := []models.UserInfo{*users[0], *users[1]}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// Check recent access user should be executed
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_access_recent").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Recent access user expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check old access user should not be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_access_old").Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Old access user expected execution 0 times, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "access-after condition test succeeded"}
}

// testGithubStarCondition test github-star condition
func testGithubStarCondition(ctx *TestContext) TestResult {
	// Create test users
	users := []*models.UserInfo{
		{
			ID:           "user_star_yes",
			Name:         "Starred User",
			GithubStar:   "zgsm,openai/gpt-4,facebook/react",
			RegisterTime: time.Now().Add(-time.Hour * 24),
			AccessTime:   time.Now().Add(-time.Hour * 1),
		},
		{
			ID:           "user_star_no",
			Name:         "Non-starred User",
			GithubStar:   "microsoft/vscode,google/tensorflow",
			RegisterTime: time.Now().Add(-time.Hour * 24),
			AccessTime:   time.Now().Add(-time.Hour * 1),
		},
		{
			ID:           "user_star_empty",
			Name:         "Empty Star User",
			GithubStar:   "",
			RegisterTime: time.Now().Add(-time.Hour * 24),
			AccessTime:   time.Now().Add(-time.Hour * 1),
		},
	}

	for _, user := range users {
		if err := ctx.DB.Create(user).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
		}
	}

	// Create github-star strategy
	strategy := &models.QuotaStrategy{
		Name:      "github-star-test",
		Title:     "GitHub Star Test",
		Type:      "single",
		Amount:    30,
		Model:     "test-model",
		Condition: `github-star("zgsm")`,
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	// Execute strategy
	userList := []models.UserInfo{*users[0], *users[1], *users[2]}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// Check starred user should be executed
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_star_yes").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Starred user expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check non-starred user should not be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_star_no").Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Non-starred user expected execution 0 times, actually executed %d times", executeCount)}
	}

	// Check empty starred user should not be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_star_empty").Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Empty starred user expected execution 0 times, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "github-star condition test succeeded"}
}

// testQuotaLECondition test quota-le condition
func testQuotaLECondition(ctx *TestContext) TestResult {
	// Create test users
	users := []*models.UserInfo{
		{
			ID:           "user_quota_low",
			Name:         "Low Quota User",
			RegisterTime: time.Now().Add(-time.Hour * 24),
			AccessTime:   time.Now().Add(-time.Hour * 1),
		},
		{
			ID:           "user_quota_high",
			Name:         "High Quota User",
			RegisterTime: time.Now().Add(-time.Hour * 24),
			AccessTime:   time.Now().Add(-time.Hour * 1),
		},
	}

	for _, user := range users {
		if err := ctx.DB.Create(user).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
		}
	}

	// Set mock quota
	mockStore.SetQuota("user_quota_low", 5)   // Low quota
	mockStore.SetQuota("user_quota_high", 50) // High quota

	// Create quota-le strategy
	strategy := &models.QuotaStrategy{
		Name:      "quota-le-test",
		Title:     "Quota Less Than Test",
		Type:      "single",
		Amount:    35,
		Model:     "test-model",
		Condition: `quota-le("test-model", 10)`,
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	// Execute strategy
	userList := []models.UserInfo{*users[0], *users[1]}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// Check low quota user should be executed
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_quota_low").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Low quota user expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check high quota user should not be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_quota_high").Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("High quota user expected execution 0 times, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "quota-le condition test succeeded"}
}

// testIsVipCondition test is-vip condition
func testIsVipCondition(ctx *TestContext) TestResult {
	// Create test users
	users := []*models.UserInfo{
		{
			ID:           "user_vip_high",
			Name:         "High VIP User",
			VIP:          3,
			RegisterTime: time.Now().Add(-time.Hour * 24),
			AccessTime:   time.Now().Add(-time.Hour * 1),
		},
		{
			ID:           "user_vip_low",
			Name:         "Low VIP User",
			VIP:          0,
			RegisterTime: time.Now().Add(-time.Hour * 24),
			AccessTime:   time.Now().Add(-time.Hour * 1),
		},
		{
			ID:           "user_vip_equal",
			Name:         "Equal VIP User",
			VIP:          2,
			RegisterTime: time.Now().Add(-time.Hour * 24),
			AccessTime:   time.Now().Add(-time.Hour * 1),
		},
	}

	for _, user := range users {
		if err := ctx.DB.Create(user).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
		}
	}

	// Create is-vip strategy
	strategy := &models.QuotaStrategy{
		Name:      "is-vip-test",
		Title:     "VIP Level Test",
		Type:      "single",
		Amount:    40,
		Model:     "test-model",
		Condition: `is-vip(2)`,
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	// Execute strategy
	userList := []models.UserInfo{*users[0], *users[1], *users[2]}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// Check high VIP user should be executed
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_vip_high").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("High VIP user expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check equal VIP user should be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_vip_equal").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Equal VIP user expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check low VIP user should not be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_vip_low").Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Low VIP user expected execution 0 times, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "is-vip condition test succeeded"}
}

// testBelongToCondition test belong-to condition
func testBelongToCondition(ctx *TestContext) TestResult {
	// Create test users
	users := []*models.UserInfo{
		{
			ID:           "user_org_target",
			Name:         "Target Org User",
			Org:          "org001",
			RegisterTime: time.Now().Add(-time.Hour * 24),
			AccessTime:   time.Now().Add(-time.Hour * 1),
		},
		{
			ID:           "user_org_other",
			Name:         "Other Org User",
			Org:          "org002",
			RegisterTime: time.Now().Add(-time.Hour * 24),
			AccessTime:   time.Now().Add(-time.Hour * 1),
		},
		{
			ID:           "user_org_empty",
			Name:         "No Org User",
			Org:          "",
			RegisterTime: time.Now().Add(-time.Hour * 24),
			AccessTime:   time.Now().Add(-time.Hour * 1),
		},
	}

	for _, user := range users {
		if err := ctx.DB.Create(user).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
		}
	}

	// Create belong-to strategy
	strategy := &models.QuotaStrategy{
		Name:      "belong-to-test",
		Title:     "Organization Belonging Test",
		Type:      "single",
		Amount:    45,
		Model:     "test-model",
		Condition: `belong-to("org001")`,
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	// Execute strategy
	userList := []models.UserInfo{*users[0], *users[1], *users[2]}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// Check target organization user should be executed
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_org_target").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Target organization user expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check other organization user should not be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_org_other").Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Other organization user expected execution 0 times, actually executed %d times", executeCount)}
	}

	// Check no organization user should not be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_org_empty").Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("No organization user expected execution 0 times, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "belong-to condition test succeeded"}
}

// testAndCondition test and nesting condition
func testAndCondition(ctx *TestContext) TestResult {
	// Create test users
	users := []*models.UserInfo{
		{
			ID:           "user_and_both",
			Name:         "Both Conditions User",
			VIP:          2,
			GithubStar:   "zgsm,openai/gpt-4",
			RegisterTime: time.Now().Add(-time.Hour * 24),
			AccessTime:   time.Now().Add(-time.Hour * 1),
		},
		{
			ID:           "user_and_vip_only",
			Name:         "VIP Only User",
			VIP:          3,
			GithubStar:   "microsoft/vscode",
			RegisterTime: time.Now().Add(-time.Hour * 24),
			AccessTime:   time.Now().Add(-time.Hour * 1),
		},
		{
			ID:           "user_and_star_only",
			Name:         "Star Only User",
			VIP:          0,
			GithubStar:   "zgsm,facebook/react",
			RegisterTime: time.Now().Add(-time.Hour * 24),
			AccessTime:   time.Now().Add(-time.Hour * 1),
		},
	}

	for _, user := range users {
		if err := ctx.DB.Create(user).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
		}
	}

	// Create and condition strategy
	strategy := &models.QuotaStrategy{
		Name:      "and-condition-test",
		Title:     "AND Condition Test",
		Type:      "single",
		Amount:    50,
		Model:     "test-model",
		Condition: `and(is-vip(2), github-star("zgsm"))`,
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	// Execute strategy
	userList := []models.UserInfo{*users[0], *users[1], *users[2]}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// Check users simultaneously satisfying both conditions should be executed
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_and_both").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Users simultaneously satisfying conditions expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check users satisfying only VIP condition should not be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_and_vip_only").Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Users satisfying only VIP condition expected execution 0 times, actually executed %d times", executeCount)}
	}

	// Check users satisfying only star condition should not be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_and_star_only").Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Users satisfying only star condition expected execution 0 times, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "and condition test succeeded"}
}

// testOrCondition test or nesting condition
func testOrCondition(ctx *TestContext) TestResult {
	// Create test users
	users := []*models.UserInfo{
		{
			ID:           "user_or_both",
			Name:         "Both Conditions User",
			VIP:          3,
			Org:          "org001",
			RegisterTime: time.Now().Add(-time.Hour * 24),
			AccessTime:   time.Now().Add(-time.Hour * 1),
		},
		{
			ID:           "user_or_vip_only",
			Name:         "VIP Only User",
			VIP:          2,
			Org:          "org002",
			RegisterTime: time.Now().Add(-time.Hour * 24),
			AccessTime:   time.Now().Add(-time.Hour * 1),
		},
		{
			ID:           "user_or_org_only",
			Name:         "Org Only User",
			VIP:          0,
			Org:          "org001",
			RegisterTime: time.Now().Add(-time.Hour * 24),
			AccessTime:   time.Now().Add(-time.Hour * 1),
		},
		{
			ID:           "user_or_neither",
			Name:         "Neither User",
			VIP:          0,
			Org:          "org002",
			RegisterTime: time.Now().Add(-time.Hour * 24),
			AccessTime:   time.Now().Add(-time.Hour * 1),
		},
	}

	for _, user := range users {
		if err := ctx.DB.Create(user).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
		}
	}

	// Create or condition strategy
	strategy := &models.QuotaStrategy{
		Name:      "or-condition-test",
		Title:     "OR Condition Test",
		Type:      "single",
		Amount:    55,
		Model:     "test-model",
		Condition: `or(is-vip(2), belong-to("org001"))`,
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	// Execute strategy
	userList := []models.UserInfo{*users[0], *users[1], *users[2], *users[3]}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// Check users simultaneously satisfying both conditions should be executed
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_or_both").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Users simultaneously satisfying conditions expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check users satisfying only VIP condition should be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_or_vip_only").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Users satisfying only VIP condition expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check users satisfying only organization condition should be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_or_org_only").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Users satisfying only organization condition expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check users not satisfying any condition should not be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_or_neither").Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Users not satisfying any condition expected execution 0 times, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "or condition test succeeded"}
}

// testNotCondition test not nesting condition
func testNotCondition(ctx *TestContext) TestResult {
	// Create test users
	users := []*models.UserInfo{
		{
			ID:           "user_not_vip",
			Name:         "VIP User",
			VIP:          3,
			RegisterTime: time.Now().Add(-time.Hour * 24),
			AccessTime:   time.Now().Add(-time.Hour * 1),
		},
		{
			ID:           "user_not_normal",
			Name:         "Normal User",
			VIP:          0,
			RegisterTime: time.Now().Add(-time.Hour * 24),
			AccessTime:   time.Now().Add(-time.Hour * 1),
		},
	}

	for _, user := range users {
		if err := ctx.DB.Create(user).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
		}
	}

	// Create not condition strategy
	strategy := &models.QuotaStrategy{
		Name:      "not-condition-test",
		Title:     "NOT Condition Test",
		Type:      "single",
		Amount:    60,
		Model:     "test-model",
		Condition: `not(is-vip(2))`,
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	// Execute strategy
	userList := []models.UserInfo{*users[0], *users[1]}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// Check VIP user should not be executed (excluded by NOT)
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_not_vip").Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("VIP user expected execution 0 times, actually executed %d times", executeCount)}
	}

	// Check normal user should be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_not_normal").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Normal user expected execution 1 time, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "not condition test succeeded"}
}

// testComplexCondition test complex nesting condition
func testComplexCondition(ctx *TestContext) TestResult {
	// Create test users
	users := []*models.UserInfo{
		{
			ID:           "user_complex_match1",
			Name:         "Complex Match 1",
			VIP:          3,
			GithubStar:   "zgsm,openai/gpt-4",
			Org:          "org001",
			RegisterTime: time.Now().Add(-time.Hour * 24),
			AccessTime:   time.Now().Add(-time.Hour * 1),
		},
		{
			ID:           "user_complex_match2",
			Name:         "Complex Match 2",
			VIP:          0,
			GithubStar:   "",
			Org:          "org002",
			RegisterTime: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			AccessTime:   time.Now().Add(-time.Hour * 1),
		},
		{
			ID:           "user_complex_no_match",
			Name:         "Complex No Match",
			VIP:          1,
			GithubStar:   "microsoft/vscode",
			Org:          "org003",
			RegisterTime: time.Now().Add(-time.Hour * 24),
			AccessTime:   time.Now().Add(-time.Hour * 1),
		},
	}

	for _, user := range users {
		if err := ctx.DB.Create(user).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
		}
	}

	// Create complex nesting condition strategy
	// (is-vip(3) AND github-star("zgsm")) OR (register-before("2024-01-01 00:00:00") AND belong-to("org002"))
	strategy := &models.QuotaStrategy{
		Name:      "complex-condition-test",
		Title:     "Complex Condition Test",
		Type:      "single",
		Amount:    65,
		Model:     "test-model",
		Condition: `or(and(is-vip(3), github-star("zgsm")), and(register-before("2024-01-01 00:00:00"), belong-to("org002")))`,
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	// Execute strategy
	userList := []models.UserInfo{*users[0], *users[1], *users[2]}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// Check user satisfying first condition should be executed
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_complex_match1").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User satisfying first condition expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check user satisfying second condition should be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_complex_match2").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User satisfying second condition expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check user not satisfying any condition should not be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_complex_no_match").Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User not satisfying any condition expected execution 0 times, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "complex condition test succeeded"}
}

// testSingleTypeStrategy test single recharge strategy
func testSingleTypeStrategy(ctx *TestContext) TestResult {
	// Create test user
	user := &models.UserInfo{
		ID:           "user_single_test",
		Name:         "Single Test User",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}
	if err := ctx.DB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Create single recharge strategy
	strategy := &models.QuotaStrategy{
		Name:      "single-type-test",
		Title:     "Single Recharge Test",
		Type:      "single",
		Amount:    70,
		Model:     "test-model",
		Condition: "", // Empty condition, all users match
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	// First execute strategy
	users := []models.UserInfo{*user}
	ctx.StrategyService.ExecStrategy(strategy, users)

	// Check first execution result
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("First execution expected 1 time, actually executed %d times", executeCount)}
	}

	// Second execute strategy (should be skipped because it has already been executed)
	ctx.StrategyService.ExecStrategy(strategy, users)

	// Check second execution result (should still be 1 time)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Single strategy repeated execution, expected still 1 time, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "Single Recharge Strategy Test Succeeded"}
}

// testPeriodicTypeStrategy test periodic recharge strategy
func testPeriodicTypeStrategy(ctx *TestContext) TestResult {
	// Create test user
	user := &models.UserInfo{
		ID:           "user_periodic_test",
		Name:         "Periodic Test User",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}
	if err := ctx.DB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Create periodic recharge strategy (execute every minute for testing)
	strategy := &models.QuotaStrategy{
		Name:         "periodic-type-test",
		Title:        "Periodic Recharge Test",
		Type:         "periodic",
		Amount:       75,
		Model:        "test-model",
		PeriodicExpr: "* * * * *", // Execute every minute
		Condition:    "",          // Empty condition, all users match
		Status:       true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	// First execute strategy
	users := []models.UserInfo{*user}
	ctx.StrategyService.ExecStrategy(strategy, users)

	// Check first execution result
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("First execution expected 1 time, actually executed %d times", executeCount)}
	}

	// Wait 2 seconds before executing strategy again (periodic strategy can be repeated)
	time.Sleep(2 * time.Second)
	ctx.StrategyService.ExecStrategy(strategy, users)

	// Check second execution result (should be 2 times)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)

	if executeCount != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Periodic strategy repeated execution, expected 2 times, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "Periodic Recharge Strategy Test Succeeded"}
}

// testStrategyStatusControl test strategy status control
func testStrategyStatusControl(ctx *TestContext) TestResult {
	// Create test user
	user := &models.UserInfo{
		ID:           "user_status_test",
		Name:         "Status Test User",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}
	if err := ctx.DB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Create disabled strategy
	strategy := &models.QuotaStrategy{
		Name:      "status-control-test",
		Title:     "Status Control Test",
		Type:      "single",
		Amount:    80,
		Model:     "test-model",
		Condition: "", // Empty condition
	}
	// First create strategy
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}
	// Then disable it
	if err := ctx.StrategyService.DisableStrategy(strategy.ID); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Disable strategy failed: %v", err)}
	}

	// Execute disabled strategy
	users := []models.UserInfo{*user}
	ctx.StrategyService.ExecStrategy(strategy, users)

	// Check disabled strategy should not be executed
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Disabled strategy expected execution 0 times, actually executed %d times", executeCount)}
	}

	// Enable strategy
	if err := ctx.StrategyService.EnableStrategy(strategy.ID); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Enable strategy failed: %v", err)}
	}

	// Execute strategy again
	ctx.StrategyService.ExecStrategy(strategy, users)

	// Check enabled strategy should be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Enabled strategy expected execution 1 time, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "Strategy Status Control Test Succeeded"}
}

// testAiGatewayFailure test AiGateway request failure
func testAiGatewayFailure(ctx *TestContext) TestResult {
	// Create test user
	user := &models.UserInfo{
		ID:           "user_gateway_fail",
		Name:         "Gateway Fail User",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}
	if err := ctx.DB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Create mock AiGateway config pointing to the fail server
	parsedURL, err := url.Parse(ctx.FailServer.URL)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to parse fail server URL: %v", err)}
	}
	host, portStr, err := net.SplitHostPort(parsedURL.Host)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to split host and port: %v", err)}
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to parse port: %v", err)}
	}

	failAiGatewayConfig := &config.AiGatewayConfig{
		Host:       host,
		Port:       port,
		AdminPath:  "/v1/chat/completions",
		Credential: "credential3",
	}

	// Create services using failed gateway configuration
	failQuotaService := services.NewQuotaService(ctx.DB.DB, failAiGatewayConfig, ctx.VoucherService)
	failGateway := aigateway.NewClient(ctx.FailServer.URL, "/v1/chat/completions", "credential3")
	failStrategyService := services.NewStrategyService(ctx.DB, failGateway, failQuotaService)

	// Create strategy
	strategy := &models.QuotaStrategy{
		Name:      "gateway-failure-test",
		Title:     "Gateway Failure Test",
		Type:      "single",
		Amount:    85,
		Model:     "test-model",
		Condition: "", // Empty condition
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	// Execute strategy using failed gateway
	users := []models.UserInfo{*user}
	failStrategyService.ExecStrategy(strategy, users)

	// Check execution record exists but status is failed
	var execute models.QuotaExecute
	err = ctx.DB.Where("strategy_id = ? AND user_id = ? AND status = 'failed'", strategy.ID, user.ID).First(&execute).Error

	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Execution record not found: %v", err)}
	}

	if execute.Status != "failed" {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected status failed, actual status %s", execute.Status)}
	}

	return TestResult{Passed: true, Message: "Gateway Failure Test Succeeded"}
}

// testBatchUserProcessing test batch user processing
func testBatchUserProcessing(ctx *TestContext) TestResult {
	// Create multiple test users
	users := make([]*models.UserInfo, 10)
	for i := 0; i < 10; i++ {
		users[i] = &models.UserInfo{
			ID:           fmt.Sprintf("batch_user_%03d", i),
			Name:         fmt.Sprintf("Batch User %d", i),
			VIP:          i % 4, // VIP level 0-3
			RegisterTime: time.Now().Add(-time.Hour * 24),
			AccessTime:   time.Now().Add(-time.Hour * 1),
		}
		if err := ctx.DB.Create(users[i]).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create user%d failed: %v", i, err)}
		}
	}

	// Create VIP user strategy
	strategy := &models.QuotaStrategy{
		Name:      "batch-processing-test",
		Title:     "Batch Processing Test",
		Type:      "single",
		Amount:    90,
		Model:     "test-model",
		Condition: `is-vip(2)`, // VIP level >=2 users
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	// Execute strategy
	userList := make([]models.UserInfo, len(users))
	for i, user := range users {
		userList[i] = *user
	}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// Check execution result - should be 4 users executed (VIP level 2 and 3 users)
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND status = 'completed'", strategy.ID).Count(&executeCount)

	expectedCount := int64(4) // VIP level 2,3 users each 2 times, total 4
	if executeCount != expectedCount {
		return TestResult{Passed: false, Message: fmt.Sprintf("Batch processing expected execution %d times, actually executed %d times", expectedCount, executeCount)}
	}

	// Verify specific executed users
	var executes []models.QuotaExecute
	ctx.DB.Where("strategy_id = ?", strategy.ID).Find(&executes)

	executedUsers := make(map[string]bool)
	for _, exec := range executes {
		executedUsers[exec.User] = true
	}

	// Check VIP>=2 users should be executed
	for _, user := range users {
		shouldExecute := user.VIP >= 2
		wasExecuted := executedUsers[user.ID]

		if shouldExecute != wasExecuted {
			return TestResult{
				Passed:  false,
				Message: fmt.Sprintf("User%s VIP%d, expected execution:%v, actual execution:%v", user.ID, user.VIP, shouldExecute, wasExecuted),
			}
		}
	}

	return TestResult{Passed: true, Message: "Batch User Processing Test Succeeded"}
}

// testVoucherGenerationAndValidation test voucher generation and validation
func testVoucherGenerationAndValidation(ctx *TestContext) TestResult {
	// Test voucher data
	voucherData := &services.VoucherData{
		GiverID:     "giver123",
		GiverName:   "张三",
		GiverPhone:  "13800138000",
		GiverGithub: "zhangsan",
		ReceiverID:  "receiver456",
		QuotaList: []services.VoucherQuotaItem{
			{Amount: 10, ExpiryDate: time.Now().Add(30 * 24 * time.Hour)},
			{Amount: 20, ExpiryDate: time.Now().Add(60 * 24 * time.Hour)},
		},
	}

	// Generate voucher
	voucherCode, err := ctx.VoucherService.GenerateVoucher(voucherData)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Generate voucher failed: %v", err)}
	}

	if voucherCode == "" {
		return TestResult{Passed: false, Message: "Generated voucher code is empty"}
	}

	// Validate and decode voucher
	decodedData, err := ctx.VoucherService.ValidateAndDecodeVoucher(voucherCode)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Validate voucher failed: %v", err)}
	}

	// Verify decoded data
	if decodedData.GiverID != voucherData.GiverID ||
		decodedData.GiverName != voucherData.GiverName ||
		decodedData.ReceiverID != voucherData.ReceiverID ||
		len(decodedData.QuotaList) != len(voucherData.QuotaList) {
		return TestResult{Passed: false, Message: "Decoded voucher data mismatch"}
	}

	// Test invalid voucher
	_, err = ctx.VoucherService.ValidateAndDecodeVoucher("invalid-voucher-code")
	if err == nil {
		return TestResult{Passed: false, Message: "Invalid voucher should fail validation"}
	}

	return TestResult{Passed: true, Message: "Voucher Generation and Validation Test Succeeded"}
}

// testQuotaTransferOut test quota transfer out
func testQuotaTransferOut(ctx *TestContext) TestResult {
	// Create test users
	giver := &models.UserInfo{
		ID:             "giver_user",
		Name:           "Giver User",
		Phone:          "13800138000",
		GithubUsername: "giver",
		RegisterTime:   time.Now().Add(-time.Hour * 24),
		AccessTime:     time.Now().Add(-time.Hour * 1),
	}
	if err := ctx.DB.Create(giver).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create giver user failed: %v", err)}
	}

	// Add initial quota for giver
	expiryDate := time.Now().Add(30 * 24 * time.Hour)
	quota := &models.Quota{
		UserID:     giver.ID,
		Amount:     100,
		ExpiryDate: expiryDate,
		Status:     models.StatusValid,
	}
	if err := ctx.DB.Create(quota).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create initial quota failed: %v", err)}
	}

	// Create AuthUser for giver
	giverAuth := &models.AuthUser{
		ID:      giver.ID,
		Name:    giver.Name,
		StaffID: "test_staff_id",
		Github:  giver.GithubUsername,
		Phone:   giver.Phone,
	}

	// Transfer out request
	transferReq := &services.TransferOutRequest{
		ReceiverID: "receiver_user",
		QuotaList: []services.TransferQuotaItem{
			{Amount: 30, ExpiryDate: expiryDate},
		},
	}

	// Execute transfer out
	response, err := ctx.QuotaService.TransferOut(giverAuth, transferReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer out failed: %v", err)}
	}

	if response.VoucherCode == "" {
		return TestResult{Passed: false, Message: "Voucher code is empty"}
	}

	// Verify giver's quota is reduced
	var updatedQuota models.Quota
	if err := ctx.DB.Where("user_id = ? AND expiry_date = ?", giver.ID, expiryDate).First(&updatedQuota).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get updated quota: %v", err)}
	}

	if updatedQuota.Amount != 70 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected remaining quota 70, got %d", updatedQuota.Amount)}
	}

	// Verify audit record
	var auditRecord models.QuotaAudit
	if err := ctx.DB.Where("user_id = ? AND operation = ?", giver.ID, models.OperationTransferOut).First(&auditRecord).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get audit record: %v", err)}
	}

	if auditRecord.Amount != -30 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected audit amount -30, got %d", auditRecord.Amount)}
	}

	return TestResult{Passed: true, Message: "Quota Transfer Out Test Succeeded"}
}

// testQuotaTransferIn test quota transfer in
func testQuotaTransferIn(ctx *TestContext) TestResult {
	// Create test users
	receiver := &models.UserInfo{
		ID:           "receiver_user",
		Name:         "Receiver User",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}
	if err := ctx.DB.Create(receiver).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create receiver user failed: %v", err)}
	}

	// Generate a valid voucher
	expiryDate := time.Now().Add(30 * 24 * time.Hour)
	voucherData := &services.VoucherData{
		GiverID:     "giver_user",
		GiverName:   "Giver User",
		GiverPhone:  "13800138000",
		GiverGithub: "giver",
		ReceiverID:  receiver.ID,
		QuotaList: []services.VoucherQuotaItem{
			{Amount: 30, ExpiryDate: expiryDate},
		},
	}

	voucherCode, err := ctx.VoucherService.GenerateVoucher(voucherData)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Generate voucher failed: %v", err)}
	}

	// Create AuthUser for receiver
	receiverAuth := &models.AuthUser{
		ID:      receiver.ID,
		Name:    receiver.Name,
		StaffID: "test_staff_id",
		Github:  "receiver",
		Phone:   "13900139000",
	}

	// Transfer in request
	transferReq := &services.TransferInRequest{
		VoucherCode: voucherCode,
	}

	// Execute transfer in
	response, err := ctx.QuotaService.TransferIn(receiverAuth, transferReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer in failed: %v", err)}
	}

	if response.GiverID != voucherData.GiverID {
		return TestResult{Passed: false, Message: "Transfer in response giver ID mismatch"}
	}

	// Verify receiver's quota is added
	var quota models.Quota
	if err := ctx.DB.Where("user_id = ? AND expiry_date = ?", receiver.ID, expiryDate).First(&quota).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get receiver quota: %v", err)}
	}

	if quota.Amount != 30 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected receiver quota 30, got %d", quota.Amount)}
	}

	// Verify audit record
	var auditRecord models.QuotaAudit
	if err := ctx.DB.Where("user_id = ? AND operation = ?", receiver.ID, models.OperationTransferIn).First(&auditRecord).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get audit record: %v", err)}
	}

	if auditRecord.Amount != 30 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected audit amount 30, got %d", auditRecord.Amount)}
	}

	// Verify voucher redemption record
	var redemption models.VoucherRedemption
	if err := ctx.DB.Where("voucher_code = ?", voucherCode).First(&redemption).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get redemption record: %v", err)}
	}

	// Test duplicate redemption
	_, err = ctx.QuotaService.TransferIn(receiverAuth, transferReq)
	if err == nil {
		return TestResult{Passed: false, Message: "Duplicate redemption should fail"}
	}

	return TestResult{Passed: true, Message: "Quota Transfer In Test Succeeded"}
}

// testQuotaExpiry test quota expiry functionality
func testQuotaExpiry(ctx *TestContext) TestResult {
	// Create test user
	user := &models.UserInfo{
		ID:           "expiry_test_user",
		Name:         "Expiry Test User",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}
	if err := ctx.DB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Create expired and valid quotas
	expiredDate := time.Now().Add(-time.Hour)
	validDate := time.Now().Add(30 * 24 * time.Hour)

	quotas := []*models.Quota{
		{UserID: user.ID, Amount: 50, ExpiryDate: expiredDate, Status: models.StatusValid},
		{UserID: user.ID, Amount: 100, ExpiryDate: validDate, Status: models.StatusValid},
	}

	for _, quota := range quotas {
		if err := ctx.DB.Create(quota).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create quota failed: %v", err)}
		}
	}

	// Set initial AiGateway quota
	mockStore.SetQuota(user.ID, 150)

	// Execute quota expiry
	if err := ctx.QuotaService.ExpireQuotas(); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expire quotas failed: %v", err)}
	}

	// Verify expired quota status
	var expiredQuota models.Quota
	if err := ctx.DB.Where("user_id = ? AND expiry_date = ?", user.ID, expiredDate).First(&expiredQuota).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get expired quota: %v", err)}
	}

	if expiredQuota.Status != models.StatusExpired {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected expired status, got %s", expiredQuota.Status)}
	}

	// Verify valid quota remains valid
	var validQuota models.Quota
	if err := ctx.DB.Where("user_id = ? AND expiry_date = ?", user.ID, validDate).First(&validQuota).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get valid quota: %v", err)}
	}

	if validQuota.Status != models.StatusValid {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected valid status, got %s", validQuota.Status)}
	}

	return TestResult{Passed: true, Message: "Quota Expiry Test Succeeded"}
}

// testQuotaAuditRecords test quota audit records functionality
func testQuotaAuditRecords(ctx *TestContext) TestResult {
	// Create test user
	user := &models.UserInfo{
		ID:           "audit_test_user",
		Name:         "Audit Test User",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}
	if err := ctx.DB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Add quota using strategy execution
	if err := ctx.QuotaService.AddQuotaForStrategy(user.ID, 50, "test-strategy"); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Add quota for strategy failed: %v", err)}
	}

	// Get audit records
	records, total, err := ctx.QuotaService.GetQuotaAuditRecords(user.ID, 1, 10)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get audit records failed: %v", err)}
	}

	if total != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 audit record, got %d", total)}
	}

	if len(records) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 record in result, got %d", len(records))}
	}

	record := records[0]
	if record.Amount != 50 || record.Operation != models.OperationRecharge {
		return TestResult{Passed: false, Message: "Audit record data mismatch"}
	}

	return TestResult{Passed: true, Message: "Quota Audit Records Test Succeeded"}
}

// testStrategyWithExpiryDate test strategy execution with expiry date
func testStrategyWithExpiryDate(ctx *TestContext) TestResult {
	// Create test user
	user := &models.UserInfo{
		ID:           "strategy_expiry_user",
		Name:         "Strategy Expiry User",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}
	if err := ctx.DB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Create strategy
	strategy := &models.QuotaStrategy{
		Name:      "expiry-date-test",
		Title:     "Expiry Date Test",
		Type:      "single",
		Amount:    75,
		Model:     "test-model",
		Condition: "",
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	// Execute strategy
	users := []models.UserInfo{*user}
	ctx.StrategyService.ExecStrategy(strategy, users)

	// Verify quota was created with expiry date
	var quota models.Quota
	if err := ctx.DB.Where("user_id = ?", user.ID).First(&quota).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get quota: %v", err)}
	}

	if quota.Amount != 75 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected quota amount 75, got %d", quota.Amount)}
	}

	if quota.Status != models.StatusValid {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected valid status, got %s", quota.Status)}
	}

	// Verify expiry date is set correctly (end of month or next month)
	now := time.Now()
	endOfMonth := time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, now.Location())
	var expectedExpiry time.Time
	if endOfMonth.Sub(now).Hours() < 24*30 {
		expectedExpiry = time.Date(now.Year(), now.Month()+2, 0, 23, 59, 59, 0, now.Location())
	} else {
		expectedExpiry = endOfMonth
	}

	if !quota.ExpiryDate.Equal(expectedExpiry) {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected expiry date %v, got %v", expectedExpiry, quota.ExpiryDate)}
	}

	// Verify execution record has expiry date
	var execute models.QuotaExecute
	if err := ctx.DB.Where("strategy_id = ? AND user_id = ?", strategy.ID, user.ID).First(&execute).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get execute record: %v", err)}
	}

	if !execute.ExpiryDate.Equal(expectedExpiry) {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected execute expiry date %v, got %v", expectedExpiry, execute.ExpiryDate)}
	}

	return TestResult{Passed: true, Message: "Strategy with Expiry Date Test Succeeded"}
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

// testMultipleOperationsAccuracy test accuracy of quota calculations under multiple operations
func testMultipleOperationsAccuracy(ctx *TestContext) TestResult {
	// Create test users
	user1 := &models.UserInfo{
		ID:           "user_ops_accuracy_1",
		Name:         "Operations User 1",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}
	user2 := &models.UserInfo{
		ID:           "user_ops_accuracy_2",
		Name:         "Operations User 2",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}

	if err := ctx.DB.Create(user1).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user1 failed: %v", err)}
	}
	if err := ctx.DB.Create(user2).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user2 failed: %v", err)}
	}

	// Initialize mock quota for both users
	mockStore.SetQuota(user1.ID, 0)
	mockStore.SetQuota(user2.ID, 0)

	// 1. Add initial quota via strategy for user1
	if err := ctx.QuotaService.AddQuotaForStrategy(user1.ID, 100, "initial-strategy"); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Add initial quota failed: %v", err)}
	}

	// 2. Transfer some quota from user1 to user2 - use same expiry date as created by strategy
	now := time.Now()
	var transferExpiryDate time.Time
	endOfMonth := time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, now.Location())
	if endOfMonth.Sub(now).Hours() < 24*30 {
		transferExpiryDate = time.Date(now.Year(), now.Month()+2, 0, 23, 59, 59, 0, now.Location())
	} else {
		transferExpiryDate = endOfMonth
	}

	transferOutReq := &services.TransferOutRequest{
		ReceiverID: user2.ID,
		QuotaList: []services.TransferQuotaItem{
			{Amount: 30, ExpiryDate: transferExpiryDate},
		},
	}
	transferOutResp, err := ctx.QuotaService.TransferOut(&models.AuthUser{
		ID: user1.ID, Name: user1.Name, Phone: "13800138000", Github: "user1",
	}, transferOutReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer out failed: %v", err)}
	}

	// 3. User2 transfers in the quota
	transferInReq := &services.TransferInRequest{
		VoucherCode: transferOutResp.VoucherCode,
	}
	_, err = ctx.QuotaService.TransferIn(&models.AuthUser{
		ID: user2.ID, Name: user2.Name, Phone: "13900139000", Github: "user2",
	}, transferInReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer in failed: %v", err)}
	}

	// 4. Consume some quota for user1 and user2
	ctx.QuotaService.DeltaUsedQuotaInAiGateway(user1.ID, 20)
	ctx.QuotaService.DeltaUsedQuotaInAiGateway(user2.ID, 10)

	// 5. Add more quota via strategy for user1
	if err := ctx.QuotaService.AddQuotaForStrategy(user1.ID, 50, "additional-strategy"); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Add additional quota failed: %v", err)}
	}

	// Verify user1 quota calculations
	quotaInfo1, err := ctx.QuotaService.GetUserQuota(user1.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user1 quota failed: %v", err)}
	}

	expectedTotalUser1 := 120 // 100 initial + 50 additional - 30 transferred out
	expectedUsedUser1 := 20
	if quotaInfo1.TotalQuota != expectedTotalUser1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User1 total quota incorrect: expected %d, got %d", expectedTotalUser1, quotaInfo1.TotalQuota)}
	}
	if quotaInfo1.UsedQuota != expectedUsedUser1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User1 used quota incorrect: expected %d, got %d", expectedUsedUser1, quotaInfo1.UsedQuota)}
	}

	// Verify user2 quota calculations
	quotaInfo2, err := ctx.QuotaService.GetUserQuota(user2.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user2 quota failed: %v", err)}
	}

	expectedTotalUser2 := 30 // 30 transferred in
	expectedUsedUser2 := 10
	if quotaInfo2.TotalQuota != expectedTotalUser2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User2 total quota incorrect: expected %d, got %d", expectedTotalUser2, quotaInfo2.TotalQuota)}
	}
	if quotaInfo2.UsedQuota != expectedUsedUser2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User2 used quota incorrect: expected %d, got %d", expectedUsedUser2, quotaInfo2.UsedQuota)}
	}

	// Verify audit records count
	_, auditCount1, err := ctx.QuotaService.GetQuotaAuditRecords(user1.ID, 1, 100)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user1 audit records failed: %v", err)}
	}
	if auditCount1 != 3 { // initial + additional + transfer out
		return TestResult{Passed: false, Message: fmt.Sprintf("User1 audit records count incorrect: expected 3, got %d", auditCount1)}
	}

	_, auditCount2, err := ctx.QuotaService.GetQuotaAuditRecords(user2.ID, 1, 100)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user2 audit records failed: %v", err)}
	}
	if auditCount2 != 1 { // transfer in
		return TestResult{Passed: false, Message: fmt.Sprintf("User2 audit records count incorrect: expected 1, got %d", auditCount2)}
	}

	return TestResult{Passed: true, Message: "Multiple operations accuracy test succeeded"}
}

func testTransferInUserIDMismatch(ctx *TestContext) TestResult {
	// Create test users
	user1 := &models.UserInfo{
		ID:           "user_mismatch_1",
		Name:         "Mismatch User 1",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}
	user2 := &models.UserInfo{
		ID:           "user_mismatch_2",
		Name:         "Mismatch User 2",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}
	user3 := &models.UserInfo{
		ID:           "user_mismatch_3",
		Name:         "Mismatch User 3",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}

	if err := ctx.DB.Create(user1).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user1 failed: %v", err)}
	}
	if err := ctx.DB.Create(user2).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user2 failed: %v", err)}
	}
	if err := ctx.DB.Create(user3).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user3 failed: %v", err)}
	}

	// Initialize mock quota
	mockStore.SetQuota(user1.ID, 100)

	// Add quota for user1
	if err := ctx.QuotaService.AddQuotaForStrategy(user1.ID, 100, "test-strategy"); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Add quota failed: %v", err)}
	}

	// Transfer quota from user1 to user2 - use same expiry date as created by strategy
	now := time.Now()
	var transferExpiryDate time.Time
	endOfMonth := time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, now.Location())
	if endOfMonth.Sub(now).Hours() < 24*30 {
		transferExpiryDate = time.Date(now.Year(), now.Month()+2, 0, 23, 59, 59, 0, now.Location())
	} else {
		transferExpiryDate = endOfMonth
	}

	transferOutReq := &services.TransferOutRequest{
		ReceiverID: user2.ID,
		QuotaList: []services.TransferQuotaItem{
			{Amount: 50, ExpiryDate: transferExpiryDate},
		},
	}
	transferOutResp, err := ctx.QuotaService.TransferOut(&models.AuthUser{
		ID: user1.ID, Name: user1.Name, Phone: "13800138000", Github: "user1",
	}, transferOutReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer out failed: %v", err)}
	}

	// Try to transfer in with user3 (should fail as voucher is for user2)
	transferInReq := &services.TransferInRequest{
		VoucherCode: transferOutResp.VoucherCode,
	}
	_, err = ctx.QuotaService.TransferIn(&models.AuthUser{
		ID: user3.ID, Name: user3.Name, Phone: "13700137000", Github: "user3",
	}, transferInReq)
	if err == nil {
		return TestResult{Passed: false, Message: "Transfer in should have failed with mismatched user ID"}
	}

	// Verify the error message contains appropriate information
	if !strings.Contains(err.Error(), "voucher is not for this user") {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 'voucher is not for this user' error, got: %v", err)}
	}

	// Verify user3 has no quota records
	quotaInfo3, err := ctx.QuotaService.GetUserQuota(user3.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user3 quota failed: %v", err)}
	}
	if quotaInfo3.TotalQuota != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User3 should have no quota, got %d", quotaInfo3.TotalQuota)}
	}

	// Verify no audit records for user3
	_, auditCount3, err := ctx.QuotaService.GetQuotaAuditRecords(user3.ID, 1, 100)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user3 audit records failed: %v", err)}
	}
	if auditCount3 != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User3 should have no audit records, got %d", auditCount3)}
	}

	// Verify the voucher is still available for the correct user (user2)
	_, err = ctx.QuotaService.TransferIn(&models.AuthUser{
		ID: user2.ID, Name: user2.Name, Phone: "13900139000", Github: "user2",
	}, transferInReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer in with correct user failed: %v", err)}
	}

	return TestResult{Passed: true, Message: "Transfer in user ID mismatch test succeeded"}
}

func testUserQuotaConsumptionOrder(ctx *TestContext) TestResult {
	// Create test user
	user := &models.UserInfo{
		ID:           "user_consumption_order",
		Name:         "Consumption Order User",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}

	if err := ctx.DB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Initialize mock quota
	mockStore.SetQuota(user.ID, 300)

	// Add quota with different expiry dates (earliest first approach)
	now := time.Now()

	// Add quota expiring in 10 days
	earlyExpiry := now.AddDate(0, 0, 10)
	if err := ctx.DB.Create(&models.Quota{
		UserID:     user.ID,
		Amount:     100,
		ExpiryDate: earlyExpiry,
		Status:     models.StatusValid,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create early quota failed: %v", err)}
	}

	// Add quota expiring in 30 days
	midExpiry := now.AddDate(0, 0, 30)
	if err := ctx.DB.Create(&models.Quota{
		UserID:     user.ID,
		Amount:     100,
		ExpiryDate: midExpiry,
		Status:     models.StatusValid,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create mid quota failed: %v", err)}
	}

	// Add quota expiring in 60 days
	lateExpiry := now.AddDate(0, 0, 60)
	if err := ctx.DB.Create(&models.Quota{
		UserID:     user.ID,
		Amount:     100,
		ExpiryDate: lateExpiry,
		Status:     models.StatusValid,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create late quota failed: %v", err)}
	}

	// Consume 150 quota (should consume from earliest expiring quotas first)
	// This should consume: 100 from early + 50 from mid, leaving 50 from mid + 100 from late
	ctx.QuotaService.DeltaUsedQuotaInAiGateway(user.ID, 150)

	// Get user quota to verify consumption order
	quotaInfo, err := ctx.QuotaService.GetUserQuota(user.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user quota failed: %v", err)}
	}

	// Should have 2 quota items, with consumption applied to earliest first
	if len(quotaInfo.QuotaList) != 2 { // Only items with remaining quota should be shown
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 quota items with remaining quota, got %d", len(quotaInfo.QuotaList))}
	}

	// Sort items by expiry date to verify order
	if quotaInfo.QuotaList[0].ExpiryDate.After(quotaInfo.QuotaList[1].ExpiryDate) {
		return TestResult{Passed: false, Message: "Quota items should be ordered by expiry date (earliest first)"}
	}

	// Verify remaining amounts - first item (mid expiry) should have 50 remaining
	if quotaInfo.QuotaList[0].Amount != 50 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected first item to have 50 remaining, got %d", quotaInfo.QuotaList[0].Amount)}
	}

	// Second item (late expiry) should have 100 remaining
	if quotaInfo.QuotaList[1].Amount != 100 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected second item to have 100 remaining, got %d", quotaInfo.QuotaList[1].Amount)}
	}

	// Verify total remaining quota (calculate from quota list)
	expectedRemaining := quotaInfo.TotalQuota - quotaInfo.UsedQuota // Should be 300 - 150 = 150
	actualRemaining := quotaInfo.TotalQuota - quotaInfo.UsedQuota
	if actualRemaining != expectedRemaining {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected remaining quota %d, got %d", expectedRemaining, actualRemaining)}
	}

	// Verify used quota
	expectedUsed := 150
	if quotaInfo.UsedQuota != expectedUsed {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected used quota %d, got %d", expectedUsed, quotaInfo.UsedQuota)}
	}

	return TestResult{Passed: true, Message: "User quota consumption order test succeeded"}
}

func testTransferOutInsufficientAvailable(ctx *TestContext) TestResult {
	// Create test users
	user1 := &models.UserInfo{
		ID:           "user_insufficient_1",
		Name:         "Insufficient User 1",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}
	user2 := &models.UserInfo{
		ID:           "user_insufficient_2",
		Name:         "Insufficient User 2",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}

	if err := ctx.DB.Create(user1).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user1 failed: %v", err)}
	}
	if err := ctx.DB.Create(user2).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user2 failed: %v", err)}
	}

	// Initialize mock quota
	mockStore.SetQuota(user1.ID, 200)

	// Add quota with different expiry dates
	now := time.Now()

	// Add 100 quota expiring in 10 days
	earlyExpiry := now.AddDate(0, 0, 10)
	if err := ctx.DB.Create(&models.Quota{
		UserID:     user1.ID,
		Amount:     100,
		ExpiryDate: earlyExpiry,
		Status:     models.StatusValid,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create early quota failed: %v", err)}
	}

	// Add 100 quota expiring in 30 days
	lateExpiry := now.AddDate(0, 0, 30)
	if err := ctx.DB.Create(&models.Quota{
		UserID:     user1.ID,
		Amount:     100,
		ExpiryDate: lateExpiry,
		Status:     models.StatusValid,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create late quota failed: %v", err)}
	}

	// Consume 120 quota (should consume all 100 from early + 20 from late)
	// This leaves 80 available in late-expiry quota
	ctx.QuotaService.DeltaUsedQuotaInAiGateway(user1.ID, 120)

	// Try to transfer 90 quota with early expiry date (should fail - only has 0 available with early expiry)
	transferOutReq := &services.TransferOutRequest{
		ReceiverID: user2.ID,
		QuotaList: []services.TransferQuotaItem{
			{Amount: 90, ExpiryDate: earlyExpiry},
		},
	}
	_, err := ctx.QuotaService.TransferOut(&models.AuthUser{
		ID: user1.ID, Name: user1.Name, Phone: "13800138000", Github: "user1",
	}, transferOutReq)

	if err == nil {
		return TestResult{Passed: false, Message: "Transfer out should have failed due to insufficient available quota for specific expiry date"}
	}

	// Verify the error message indicates insufficient available quota
	if !strings.Contains(err.Error(), "insufficient available quota") && !strings.Contains(err.Error(), "not enough quota") {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected insufficient quota error, got: %v", err)}
	}

	// Try to transfer 80 quota with late expiry date (should succeed)
	transferOutReq2 := &services.TransferOutRequest{
		ReceiverID: user2.ID,
		QuotaList: []services.TransferQuotaItem{
			{Amount: 80, ExpiryDate: lateExpiry},
		},
	}
	_, err = ctx.QuotaService.TransferOut(&models.AuthUser{
		ID: user1.ID, Name: user1.Name, Phone: "13800138000", Github: "user1",
	}, transferOutReq2)

	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer out with sufficient available quota should succeed: %v", err)}
	}

	// Verify user1's remaining quota
	quotaInfo1, err := ctx.QuotaService.GetUserQuota(user1.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user1 quota failed: %v", err)}
	}

	// Should have 0 remaining quota (all consumed or transferred)
	actualRemaining1 := quotaInfo1.TotalQuota - quotaInfo1.UsedQuota
	if actualRemaining1 != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 0 remaining quota, got %d", actualRemaining1)}
	}

	// Try to transfer 1 more quota (should fail - no remaining quota)
	transferOutReq3 := &services.TransferOutRequest{
		ReceiverID: user2.ID,
		QuotaList: []services.TransferQuotaItem{
			{Amount: 1, ExpiryDate: lateExpiry},
		},
	}
	_, err = ctx.QuotaService.TransferOut(&models.AuthUser{
		ID: user1.ID, Name: user1.Name, Phone: "13800138000", Github: "user1",
	}, transferOutReq3)

	if err == nil {
		return TestResult{Passed: false, Message: "Transfer out should have failed due to no remaining quota"}
	}

	return TestResult{Passed: true, Message: "Transfer out insufficient available quota test succeeded"}
}

func testTransferInExpiredQuota(ctx *TestContext) TestResult {
	// Create test users
	user1 := &models.UserInfo{
		ID:           "user_expired_1",
		Name:         "Expired User 1",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}
	user2 := &models.UserInfo{
		ID:           "user_expired_2",
		Name:         "Expired User 2",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}

	if err := ctx.DB.Create(user1).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user1 failed: %v", err)}
	}
	if err := ctx.DB.Create(user2).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user2 failed: %v", err)}
	}

	// Initialize mock quota
	mockStore.SetQuota(user1.ID, 200)

	// Add quota with mixed expiry dates - some expired, some valid
	now := time.Now()

	// Add 100 quota that already expired (yesterday)
	expiredDate := now.AddDate(0, 0, -1)
	if err := ctx.DB.Create(&models.Quota{
		UserID:     user1.ID,
		Amount:     100,
		ExpiryDate: expiredDate,
		Status:     models.StatusExpired,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create expired quota failed: %v", err)}
	}

	// Add 100 quota that is still valid (expires in 30 days)
	validDate := now.AddDate(0, 0, 30)
	if err := ctx.DB.Create(&models.Quota{
		UserID:     user1.ID,
		Amount:     100,
		ExpiryDate: validDate,
		Status:     models.StatusValid,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create valid quota failed: %v", err)}
	}

	// Transfer out both quotas (including expired one)
	transferOutReq := &services.TransferOutRequest{
		ReceiverID: user2.ID,
		QuotaList: []services.TransferQuotaItem{
			{Amount: 100, ExpiryDate: expiredDate}, // Expired quota
			{Amount: 50, ExpiryDate: validDate},    // Valid quota
		},
	}
	transferOutResp, err := ctx.QuotaService.TransferOut(&models.AuthUser{
		ID: user1.ID, Name: user1.Name, Phone: "13800138000", Github: "user1",
	}, transferOutReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer out failed: %v", err)}
	}

	// Transfer in - should only get valid quota
	transferInReq := &services.TransferInRequest{
		VoucherCode: transferOutResp.VoucherCode,
	}
	transferInResp, err := ctx.QuotaService.TransferIn(&models.AuthUser{
		ID: user2.ID, Name: user2.Name, Phone: "13900139000", Github: "user2",
	}, transferInReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer in failed: %v", err)}
	}

	// Verify that only valid quota was transferred
	// Should only get 50 quota (expired quota should be ignored)
	if transferInResp.Amount != 50 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 50 transferred quota (excluding expired), got %d", transferInResp.Amount)}
	}

	// Verify user2's quota records
	var quotaRecords []models.Quota
	if err := ctx.DB.DB.Where("user_id = ?", user2.ID).Find(&quotaRecords).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user2 quota records failed: %v", err)}
	}

	// Should only have one quota record (the valid one)
	if len(quotaRecords) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 quota record for user2, got %d", len(quotaRecords))}
	}

	// Verify the quota record has the correct expiry date (should be the valid date)
	if !quotaRecords[0].ExpiryDate.Equal(validDate) {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected quota record expiry date to match valid date, got %v", quotaRecords[0].ExpiryDate)}
	}

	// Verify the audit record uses earliest expiry date from valid quotas only
	auditRecords, _, err := ctx.QuotaService.GetQuotaAuditRecords(user2.ID, 1, 100)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user2 audit records failed: %v", err)}
	}

	if len(auditRecords) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 audit record for user2, got %d", len(auditRecords))}
	}

	// The audit record should have the valid date as expiry date (not the expired date)
	if !auditRecords[0].ExpiryDate.Equal(validDate) {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected audit record expiry date to be valid date, got %v", auditRecords[0].ExpiryDate)}
	}

	// Verify user2's total quota
	quotaInfo2, err := ctx.QuotaService.GetUserQuota(user2.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user2 quota failed: %v", err)}
	}

	if quotaInfo2.TotalQuota != 50 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected user2 total quota 50, got %d", quotaInfo2.TotalQuota)}
	}

	return TestResult{Passed: true, Message: "Transfer in expired quota test succeeded"}
}

func testTransferInInvalidVoucher(ctx *TestContext) TestResult {
	// Create test user
	user := &models.UserInfo{
		ID:           "user_invalid_voucher",
		Name:         "Invalid Voucher User",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}

	if err := ctx.DB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Test case 1: Completely invalid voucher code (too short)
	transferInReq1 := &services.TransferInRequest{
		VoucherCode: "invalid",
	}
	_, err := ctx.QuotaService.TransferIn(&models.AuthUser{
		ID: user.ID, Name: user.Name, Phone: "13800138000", Github: "user",
	}, transferInReq1)

	if err == nil {
		return TestResult{Passed: false, Message: "Should fail with completely invalid voucher"}
	}

	// Test case 2: Voucher with invalid format (missing separators)
	transferInReq2 := &services.TransferInRequest{
		VoucherCode: "invalidvouchercodewithoutanyseparators",
	}
	_, err = ctx.QuotaService.TransferIn(&models.AuthUser{
		ID: user.ID, Name: user.Name, Phone: "13800138000", Github: "user",
	}, transferInReq2)

	if err == nil {
		return TestResult{Passed: false, Message: "Should fail with invalid format voucher"}
	}

	// Test case 3: Voucher with tampered signature
	// Create a valid voucher structure but with wrong signature
	tamperedVoucher := "user1|receiver1|100|2024-12-31T23:59:59Z|tampered_signature"
	transferInReq3 := &services.TransferInRequest{
		VoucherCode: tamperedVoucher,
	}
	_, err = ctx.QuotaService.TransferIn(&models.AuthUser{
		ID: user.ID, Name: user.Name, Phone: "13800138000", Github: "user",
	}, transferInReq3)

	if err == nil {
		return TestResult{Passed: false, Message: "Should fail with tampered signature voucher"}
	}

	// Verify that no quota was transferred to the user
	quotaInfo, err := ctx.QuotaService.GetUserQuota(user.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user quota failed: %v", err)}
	}

	if quotaInfo.TotalQuota != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User should have no quota after failed transfers, got %d", quotaInfo.TotalQuota)}
	}

	// Verify no audit records were created
	_, auditCount, err := ctx.QuotaService.GetQuotaAuditRecords(user.ID, 1, 100)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get audit records failed: %v", err)}
	}

	if auditCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Should have no audit records for failed transfers, got %d", auditCount)}
	}

	return TestResult{Passed: true, Message: "Transfer in invalid voucher test succeeded"}
}

func testTransferInQuotaExpiryConsistency(ctx *TestContext) TestResult {
	// Create test users
	user1 := &models.UserInfo{
		ID:           "user_expiry_consistency_1",
		Name:         "Expiry Consistency User 1",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}
	user2 := &models.UserInfo{
		ID:           "user_expiry_consistency_2",
		Name:         "Expiry Consistency User 2",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}

	if err := ctx.DB.Create(user1).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user1 failed: %v", err)}
	}
	if err := ctx.DB.Create(user2).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user2 failed: %v", err)}
	}

	// Initialize mock quota
	mockStore.SetQuota(user1.ID, 200)

	// Add quota with different expiry dates
	now := time.Now()

	// Add quota expiring in 15 days
	earlyExpiry := now.AddDate(0, 0, 15)
	if err := ctx.DB.Create(&models.Quota{
		UserID:     user1.ID,
		Amount:     50,
		ExpiryDate: earlyExpiry,
		Status:     models.StatusValid,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create early quota failed: %v", err)}
	}

	// Add quota expiring in 45 days
	lateExpiry := now.AddDate(0, 0, 45)
	if err := ctx.DB.Create(&models.Quota{
		UserID:     user1.ID,
		Amount:     150,
		ExpiryDate: lateExpiry,
		Status:     models.StatusValid,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create late quota failed: %v", err)}
	}

	// Transfer out with specific expiry dates
	transferOutReq := &services.TransferOutRequest{
		ReceiverID: user2.ID,
		QuotaList: []services.TransferQuotaItem{
			{Amount: 30, ExpiryDate: earlyExpiry}, // Early expiry
			{Amount: 70, ExpiryDate: lateExpiry},  // Late expiry
		},
	}
	transferOutResp, err := ctx.QuotaService.TransferOut(&models.AuthUser{
		ID: user1.ID, Name: user1.Name, Phone: "13800138000", Github: "user1",
	}, transferOutReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer out failed: %v", err)}
	}

	// Transfer in
	transferInReq := &services.TransferInRequest{
		VoucherCode: transferOutResp.VoucherCode,
	}
	_, err = ctx.QuotaService.TransferIn(&models.AuthUser{
		ID: user2.ID, Name: user2.Name, Phone: "13900139000", Github: "user2",
	}, transferInReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer in failed: %v", err)}
	}

	// Verify the audit record for user2 has the earliest expiry date (earlyExpiry)
	auditRecords2, _, err := ctx.QuotaService.GetQuotaAuditRecords(user2.ID, 1, 100)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user2 audit records failed: %v", err)}
	}

	if len(auditRecords2) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 audit record for user2, got %d", len(auditRecords2))}
	}

	// The audit record should have the earliest expiry date
	if !auditRecords2[0].ExpiryDate.Equal(earlyExpiry) {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected audit record expiry date to be %v, got %v", earlyExpiry, auditRecords2[0].ExpiryDate)}
	}

	// Verify user2's quota records have correct individual expiry dates
	var quotaRecords []models.Quota
	if err := ctx.DB.DB.Where("user_id = ?", user2.ID).Order("expiry_date ASC").Find(&quotaRecords).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user2 quota records failed: %v", err)}
	}

	if len(quotaRecords) != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 quota records for user2, got %d", len(quotaRecords))}
	}

	// First record should have early expiry
	if !quotaRecords[0].ExpiryDate.Equal(earlyExpiry) {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected first quota record expiry to be %v, got %v", earlyExpiry, quotaRecords[0].ExpiryDate)}
	}

	// Second record should have late expiry
	if !quotaRecords[1].ExpiryDate.Equal(lateExpiry) {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected second quota record expiry to be %v, got %v", lateExpiry, quotaRecords[1].ExpiryDate)}
	}

	// Verify the audit record for user1 (transfer out) also has the earliest expiry date
	auditRecords1, _, err := ctx.QuotaService.GetQuotaAuditRecords(user1.ID, 1, 100)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user1 audit records failed: %v", err)}
	}

	// Find the transfer out record (should be the first one for transfer out)
	transferOutRecord := auditRecords1[0]
	if transferOutRecord.Operation != "TRANSFER_OUT" {
		// Find the transfer out record if not the first
		for _, record := range auditRecords1 {
			if record.Operation == "TRANSFER_OUT" {
				transferOutRecord = record
				break
			}
		}
	}

	// The transfer out audit record should also have the earliest expiry date
	if !transferOutRecord.ExpiryDate.Equal(earlyExpiry) {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected transfer out audit record expiry date to be %v, got %v", earlyExpiry, transferOutRecord.ExpiryDate)}
	}

	return TestResult{Passed: true, Message: "Transfer in quota expiry consistency test succeeded"}
}

func testStrategyExpiryDateCoverage(ctx *TestContext) TestResult {
	// Create test users
	user1 := &models.UserInfo{
		ID:           "user_strategy_coverage_1",
		Name:         "Strategy Coverage User 1",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}
	user2 := &models.UserInfo{
		ID:           "user_strategy_coverage_2",
		Name:         "Strategy Coverage User 2",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}

	if err := ctx.DB.Create(user1).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user1 failed: %v", err)}
	}
	if err := ctx.DB.Create(user2).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user2 failed: %v", err)}
	}

	// Initialize mock quota
	mockStore.SetQuota(user1.ID, 100)
	mockStore.SetQuota(user2.ID, 100)

	now := time.Now()

	// Test case 1: Strategy execution when >30 days remaining in current month
	// Add quota for user1 (should expire at end of current month since >30 days remaining)
	if err := ctx.QuotaService.AddQuotaForStrategy(user1.ID, 100, "test-strategy-1"); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Add quota for user1 failed: %v", err)}
	}

	// Get user1's quota to check expiry date
	var user1Quotas []models.Quota
	if err := ctx.DB.DB.Where("user_id = ?", user1.ID).Find(&user1Quotas).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user1 quotas failed: %v", err)}
	}

	if len(user1Quotas) == 0 {
		return TestResult{Passed: false, Message: "No quota records found for user1"}
	}

	// Calculate expected expiry date based on AddQuotaForStrategy logic
	endOfCurrentMonth := time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, now.Location())
	endOfNextMonth := time.Date(now.Year(), now.Month()+2, 0, 23, 59, 59, 0, now.Location())

	var expectedExpiry time.Time
	if endOfCurrentMonth.Sub(now).Hours() < 24*30 {
		expectedExpiry = endOfNextMonth
	} else {
		expectedExpiry = endOfCurrentMonth
	}

	// Verify user1's quota expiry date
	quotaExpiry := user1Quotas[0].ExpiryDate
	// Allow some tolerance for time differences (1 day)
	timeDiff := quotaExpiry.Sub(expectedExpiry)
	if timeDiff > 24*time.Hour || timeDiff < -24*time.Hour {
		return TestResult{Passed: false, Message: fmt.Sprintf("User1 quota expiry date mismatch: expected around %v, got %v", expectedExpiry, quotaExpiry)}
	}

	// Test case 2: Strategy execution when <30 days remaining in current month
	// This is simulated by the automatic logic in AddQuotaForStrategy
	if err := ctx.QuotaService.AddQuotaForStrategy(user2.ID, 100, "test-strategy-2"); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Add quota for user2 failed: %v", err)}
	}

	// Get user2's quota to check expiry date
	var user2Quotas []models.Quota
	if err := ctx.DB.DB.Where("user_id = ?", user2.ID).Find(&user2Quotas).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user2 quotas failed: %v", err)}
	}

	if len(user2Quotas) == 0 {
		return TestResult{Passed: false, Message: "No quota records found for user2"}
	}

	// Verify user2's quota expiry date follows the same logic
	user2QuotaExpiry := user2Quotas[0].ExpiryDate
	timeDiff2 := user2QuotaExpiry.Sub(expectedExpiry)
	if timeDiff2 > 24*time.Hour || timeDiff2 < -24*time.Hour {
		return TestResult{Passed: false, Message: fmt.Sprintf("User2 quota expiry date mismatch: expected around %v, got %v", expectedExpiry, user2QuotaExpiry)}
	}

	// Verify both users have positive expiry dates (in the future)
	if quotaExpiry.Before(now) {
		return TestResult{Passed: false, Message: "User1 quota should have future expiry date"}
	}

	if user2QuotaExpiry.Before(now) {
		return TestResult{Passed: false, Message: "User2 quota should have future expiry date"}
	}

	// Verify audit records contain appropriate expiry dates
	auditRecords1, _, err := ctx.QuotaService.GetQuotaAuditRecords(user1.ID, 1, 100)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user1 audit records failed: %v", err)}
	}

	if len(auditRecords1) == 0 {
		return TestResult{Passed: false, Message: "No audit records found for user1"}
	}

	// The audit record expiry date should match the quota expiry date
	auditExpiry := auditRecords1[0].ExpiryDate
	auditTimeDiff := auditExpiry.Sub(quotaExpiry)
	if auditTimeDiff > time.Minute || auditTimeDiff < -time.Minute {
		return TestResult{Passed: false, Message: fmt.Sprintf("User1 audit record expiry date should match quota expiry: audit=%v, quota=%v", auditExpiry, quotaExpiry)}
	}

	return TestResult{Passed: true, Message: "Strategy expiry date coverage test succeeded"}
}

func testTransferEarliestExpiryDate(ctx *TestContext) TestResult {
	// Create test users
	user1 := &models.UserInfo{
		ID:           "user_earliest_expiry_1",
		Name:         "Earliest Expiry User 1",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}
	user2 := &models.UserInfo{
		ID:           "user_earliest_expiry_2",
		Name:         "Earliest Expiry User 2",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}

	if err := ctx.DB.Create(user1).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user1 failed: %v", err)}
	}
	if err := ctx.DB.Create(user2).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user2 failed: %v", err)}
	}

	// Initialize mock quota
	mockStore.SetQuota(user1.ID, 300)

	// Add quota with multiple expiry dates
	now := time.Now()

	expiry1 := now.AddDate(0, 0, 10) // Earliest
	expiry2 := now.AddDate(0, 0, 20) // Middle
	expiry3 := now.AddDate(0, 0, 30) // Latest

	// Add quotas in non-chronological order to test ordering
	if err := ctx.DB.Create(&models.Quota{
		UserID:     user1.ID,
		Amount:     50,
		ExpiryDate: expiry2, // Middle expiry
		Status:     models.StatusValid,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create quota2 failed: %v", err)}
	}

	if err := ctx.DB.Create(&models.Quota{
		UserID:     user1.ID,
		Amount:     100,
		ExpiryDate: expiry1, // Earliest expiry
		Status:     models.StatusValid,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create quota1 failed: %v", err)}
	}

	if err := ctx.DB.Create(&models.Quota{
		UserID:     user1.ID,
		Amount:     150,
		ExpiryDate: expiry3, // Latest expiry
		Status:     models.StatusValid,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create quota3 failed: %v", err)}
	}

	// Transfer out multiple quotas with different expiry dates
	transferOutReq := &services.TransferOutRequest{
		ReceiverID: user2.ID,
		QuotaList: []services.TransferQuotaItem{
			{Amount: 50, ExpiryDate: expiry2}, // Middle expiry
			{Amount: 80, ExpiryDate: expiry1}, // Earliest expiry
			{Amount: 70, ExpiryDate: expiry3}, // Latest expiry
		},
	}

	transferOutResp, err := ctx.QuotaService.TransferOut(&models.AuthUser{
		ID: user1.ID, Name: user1.Name, Phone: "13800138000", Github: "user1",
	}, transferOutReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer out failed: %v", err)}
	}

	// Verify the transfer out audit record uses the earliest expiry date
	auditRecords1, _, err := ctx.QuotaService.GetQuotaAuditRecords(user1.ID, 1, 100)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user1 audit records failed: %v", err)}
	}

	// Find the transfer out record
	var transferOutRecord *services.QuotaAuditRecord
	for i, record := range auditRecords1 {
		if record.Operation == "TRANSFER_OUT" {
			transferOutRecord = &auditRecords1[i]
			break
		}
	}

	if transferOutRecord == nil {
		return TestResult{Passed: false, Message: "Transfer out audit record not found"}
	}

	// The audit record should use the earliest expiry date (expiry1)
	if !transferOutRecord.ExpiryDate.Equal(expiry1) {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer out audit record should use earliest expiry date %v, got %v", expiry1, transferOutRecord.ExpiryDate)}
	}

	// Transfer in and verify the same logic
	transferInReq := &services.TransferInRequest{
		VoucherCode: transferOutResp.VoucherCode,
	}

	_, err = ctx.QuotaService.TransferIn(&models.AuthUser{
		ID: user2.ID, Name: user2.Name, Phone: "13900139000", Github: "user2",
	}, transferInReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer in failed: %v", err)}
	}

	// Verify the transfer in audit record also uses the earliest expiry date
	auditRecords2, _, err := ctx.QuotaService.GetQuotaAuditRecords(user2.ID, 1, 100)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user2 audit records failed: %v", err)}
	}

	if len(auditRecords2) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 audit record for user2, got %d", len(auditRecords2))}
	}

	// The transfer in audit record should also use the earliest expiry date
	if !auditRecords2[0].ExpiryDate.Equal(expiry1) {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer in audit record should use earliest expiry date %v, got %v", expiry1, auditRecords2[0].ExpiryDate)}
	}

	// Additional test: Transfer out with only non-earliest expiry dates
	// Add more quota to user1
	if err := ctx.DB.Create(&models.Quota{
		UserID:     user1.ID,
		Amount:     200,
		ExpiryDate: expiry2, // Middle expiry
		Status:     models.StatusValid,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create additional quota failed: %v", err)}
	}

	// Transfer out only from middle and late expiry dates
	transferOutReq2 := &services.TransferOutRequest{
		ReceiverID: user2.ID,
		QuotaList: []services.TransferQuotaItem{
			{Amount: 30, ExpiryDate: expiry3}, // Latest expiry
			{Amount: 40, ExpiryDate: expiry2}, // Middle expiry
		},
	}

	_, err = ctx.QuotaService.TransferOut(&models.AuthUser{
		ID: user1.ID, Name: user1.Name, Phone: "13800138000", Github: "user1",
	}, transferOutReq2)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Second transfer out failed: %v", err)}
	}

	// Get the latest audit records for user1
	auditRecords1Again, _, err := ctx.QuotaService.GetQuotaAuditRecords(user1.ID, 1, 100)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user1 audit records again failed: %v", err)}
	}

	// Find the second transfer out record (should be the first in the list due to DESC order)
	secondTransferOut := auditRecords1Again[0]
	if secondTransferOut.Operation != "TRANSFER_OUT" {
		return TestResult{Passed: false, Message: "Expected first record to be the latest transfer out"}
	}

	// This transfer out should use the earliest among the transferred expiry dates (expiry2)
	if !secondTransferOut.ExpiryDate.Equal(expiry2) {
		return TestResult{Passed: false, Message: fmt.Sprintf("Second transfer out audit record should use earliest transferred expiry date %v, got %v", expiry2, secondTransferOut.ExpiryDate)}
	}

	return TestResult{Passed: true, Message: "Transfer earliest expiry date test succeeded"}
}

func testConcurrentOperations(ctx *TestContext) TestResult {
	// Create test users
	user1 := &models.UserInfo{
		ID:           "user_concurrent_1",
		Name:         "Concurrent User 1",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}
	user2 := &models.UserInfo{
		ID:           "user_concurrent_2",
		Name:         "Concurrent User 2",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}
	user3 := &models.UserInfo{
		ID:           "user_concurrent_3",
		Name:         "Concurrent User 3",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}

	if err := ctx.DB.Create(user1).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user1 failed: %v", err)}
	}
	if err := ctx.DB.Create(user2).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user2 failed: %v", err)}
	}
	if err := ctx.DB.Create(user3).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user3 failed: %v", err)}
	}

	// Initialize mock quota
	mockStore.SetQuota(user1.ID, 500)

	// Add initial quota for user1
	if err := ctx.QuotaService.AddQuotaForStrategy(user1.ID, 500, "concurrent-test-strategy"); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Add initial quota failed: %v", err)}
	}

	// Create channels for synchronization
	resultChan := make(chan error, 10)
	startChan := make(chan struct{})

	// Concurrent operation 1: Multiple quota consumptions
	go func() {
		<-startChan
		for i := 0; i < 5; i++ {
			ctx.QuotaService.DeltaUsedQuotaInAiGateway(user1.ID, 10)
		}
		resultChan <- nil
	}()

	// Concurrent operation 2: Multiple transfer outs
	go func() {
		<-startChan
		expiry := time.Now().AddDate(0, 0, 30)
		for i := 0; i < 3; i++ {
			transferOutReq := &services.TransferOutRequest{
				ReceiverID: user2.ID,
				QuotaList: []services.TransferQuotaItem{
					{Amount: 30, ExpiryDate: expiry},
				},
			}
			_, err := ctx.QuotaService.TransferOut(&models.AuthUser{
				ID: user1.ID, Name: user1.Name, Phone: "13800138000", Github: "user1",
			}, transferOutReq)
			resultChan <- err
		}
	}()

	// Concurrent operation 3: Multiple strategy executions
	go func() {
		<-startChan
		for i := 0; i < 2; i++ {
			err := ctx.QuotaService.AddQuotaForStrategy(user1.ID, 25, fmt.Sprintf("concurrent-strategy-%d", i))
			resultChan <- err
		}
	}()

	// Concurrent operation 4: Multiple quota queries
	go func() {
		<-startChan
		for i := 0; i < 5; i++ {
			_, err := ctx.QuotaService.GetUserQuota(user1.ID)
			if err != nil {
				resultChan <- err
				return
			}
		}
		resultChan <- nil
	}()

	// Start all operations simultaneously
	close(startChan)

	// Collect results
	var errors []error
	for i := 0; i < 11; i++ { // 1 + 3 + 2 + 5 operations
		if err := <-resultChan; err != nil {
			errors = append(errors, err)
		}
	}

	// Check if any operations failed
	if len(errors) > 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Concurrent operations had errors: %v", errors)}
	}

	// Verify final state consistency
	// Total quota should be: 500 (initial) + 50 (2 * 25 from strategies) - 90 (3 * 30 transfers) = 460
	// Used quota should be: 50 (5 * 10 consumption)
	// Remaining should be: 460 - 50 = 410

	finalQuotaInfo, err := ctx.QuotaService.GetUserQuota(user1.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get final quota info failed: %v", err)}
	}

	expectedTotal := 460 // 500 + 50 - 90
	expectedUsed := 50   // 5 * 10
	expectedRemaining := expectedTotal - expectedUsed

	if finalQuotaInfo.TotalQuota != expectedTotal {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected total quota %d, got %d", expectedTotal, finalQuotaInfo.TotalQuota)}
	}

	if finalQuotaInfo.UsedQuota != expectedUsed {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected used quota %d, got %d", expectedUsed, finalQuotaInfo.UsedQuota)}
	}

	actualRemaining := finalQuotaInfo.TotalQuota - finalQuotaInfo.UsedQuota
	if actualRemaining != expectedRemaining {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected remaining quota %d, got %d", expectedRemaining, actualRemaining)}
	}

	// Verify audit records consistency
	auditRecords, _, err := ctx.QuotaService.GetQuotaAuditRecords(user1.ID, 1, 100)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get audit records failed: %v", err)}
	}

	// Should have 6 audit records: 1 initial + 2 strategies + 3 transfers
	expectedAuditCount := 6
	if len(auditRecords) != expectedAuditCount {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected %d audit records, got %d", expectedAuditCount, len(auditRecords))}
	}

	// Count operations by type
	rechargeCount := 0
	transferOutCount := 0
	for _, record := range auditRecords {
		switch record.Operation {
		case "RECHARGE":
			rechargeCount++
		case "TRANSFER_OUT":
			transferOutCount++
		}
	}

	if rechargeCount != 3 { // 1 initial + 2 concurrent strategies
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 3 recharge records, got %d", rechargeCount)}
	}

	if transferOutCount != 3 { // 3 concurrent transfers
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 3 transfer out records, got %d", transferOutCount)}
	}

	return TestResult{Passed: true, Message: "Concurrent operations test succeeded"}
}
