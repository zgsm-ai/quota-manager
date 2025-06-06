package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
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

	// Create AiGateway client
	gateway := aigateway.NewClient(mockServer.URL, "/v1/chat/completions", "credential3")

	// Create strategy service
	strategyService := services.NewStrategyService(db, gateway)

	return &TestContext{
		DB:              db,
		StrategyService: strategyService,
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
	tables := []string{"quota_execute", "quota_strategy", "user_info"}
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

	// Create strategy service using failed gateway
	failGateway := aigateway.NewClient(ctx.FailServer.URL, "/v1/chat/completions", "credential3")
	failStrategyService := services.NewStrategyService(ctx.DB, failGateway)

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
	err := ctx.DB.Where("strategy_id = ? AND user_id = ? AND status = 'failed'", strategy.ID, user.ID).First(&execute).Error

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

// printTestResults print test results
func printTestResults(results []TestResult) {
	fmt.Println("\n=== Test Results Summary ===")

	totalTests := len(results)
	passedTests := 0
	totalDuration := time.Duration(0)

	for _, result := range results {
		if result.Passed {
			passedTests++
		}
		totalDuration += result.Duration
	}

	fmt.Printf("Total tests: %d\n", totalTests)
	fmt.Printf("Passed tests: %d\n", passedTests)
	fmt.Printf("Failed tests: %d\n", totalTests-passedTests)
	fmt.Printf("Total duration: %.2fs\n", totalDuration.Seconds())
	fmt.Printf("Success rate: %.1f%%\n", float64(passedTests)/float64(totalTests)*100)

	if passedTests != totalTests {
		fmt.Println("\nFailed tests:")
		for _, result := range results {
			if !result.Passed {
				fmt.Printf("❌ %s: %s\n", result.TestName, result.Message)
			}
		}
	} else {
		fmt.Println("\nAll tests passed!")
	}
}
