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

// TestContext 测试上下文
type TestContext struct {
	DB              *database.DB
	StrategyService *services.StrategyService
	Gateway         *aigateway.Client
	MockServer      *httptest.Server
	FailServer      *httptest.Server
}

// TestResult 测试结果
type TestResult struct {
	TestName string
	Passed   bool
	Message  string
	Duration time.Duration
}

// MockQuotaStore 模拟配额存储
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
	fmt.Println("=== 配额管理器集成测试 ===")

	// 初始化测试环境
	ctx, err := setupTestEnvironment()
	if err != nil {
		log.Fatalf("Failed to setup test environment: %v", err)
	}
	defer cleanupTestEnvironment(ctx)

	// 运行所有测试
	results := runAllTests(ctx)

	// 输出测试结果
	printTestResults(results)
}

// setupTestEnvironment 设置测试环境
func setupTestEnvironment() (*TestContext, error) {
	// 初始化日志
	logger.Init()

	// 加载配置
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// 连接数据库
	db, err := database.NewDB(&cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	// 自动迁移
	if err := models.AutoMigrate(db.DB); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	// 创建成功的mock服务器
	mockServer := createMockServer(false)

	// 创建失败的mock服务器
	failServer := createMockServer(true)

	// 创建AiGateway客户端
	gateway := aigateway.NewClient(mockServer.URL, "/v1/chat/completions", "credential3")

	// 创建策略服务
	strategyService := services.NewStrategyService(db, gateway)

	return &TestContext{
		DB:              db,
		StrategyService: strategyService,
		Gateway:         gateway,
		MockServer:      mockServer,
		FailServer:      failServer,
	}, nil
}

// createMockServer 创建模拟服务器
func createMockServer(shouldFail bool) *httptest.Server {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// 中间件：验证Authorization
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

			// 模拟增加配额
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

// cleanupTestEnvironment 清理测试环境
func cleanupTestEnvironment(ctx *TestContext) {
	if ctx.MockServer != nil {
		ctx.MockServer.Close()
	}
	if ctx.FailServer != nil {
		ctx.FailServer.Close()
	}
}

// runAllTests 运行所有测试
func runAllTests(ctx *TestContext) []TestResult {
	var results []TestResult

	// 测试用例列表
	testCases := []struct {
		name string
		fn   func(*TestContext) TestResult
	}{
		{"清空数据测试", testClearData},
		{"条件表达式-空条件测试", testEmptyCondition},
		{"条件表达式-match-user测试", testMatchUserCondition},
		{"条件表达式-register-before测试", testRegisterBeforeCondition},
		{"条件表达式-access-after测试", testAccessAfterCondition},
		{"条件表达式-github-star测试", testGithubStarCondition},
		{"条件表达式-quota-le测试", testQuotaLECondition},
		{"条件表达式-is-vip测试", testIsVipCondition},
		{"条件表达式-belong-to测试", testBelongToCondition},
		{"条件表达式-and嵌套测试", testAndCondition},
		// {"条件表达式-or嵌套测试", testOrCondition},
		// {"条件表达式-not嵌套测试", testNotCondition},
		// {"条件表达式-复杂嵌套测试", testComplexCondition},
		// {"单次充值策略测试", testSingleTypeStrategy},
		// {"定时充值策略测试", testPeriodicTypeStrategy},
		// {"策略状态控制测试", testStrategyStatusControl},
		// {"AiGateway请求失败测试", testAiGatewayFailure},
		// {"批量用户处理测试", testBatchUserProcessing},
	}

	for _, tc := range testCases {
		fmt.Printf("运行测试: %s\n", tc.name)
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

// testClearData 测试清空数据
func testClearData(ctx *TestContext) TestResult {
	// 清空所有表
	tables := []string{"quota_execute", "quota_strategy", "user_info"}
	for _, table := range tables {
		if err := ctx.DB.Exec("DELETE FROM " + table).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("清空表 %s 失败: %v", table, err)}
		}
	}

	// 重置mock存储
	mockStore.data = make(map[string]int)

	return TestResult{Passed: true, Message: "数据清空成功"}
}

// testEmptyCondition 测试空条件表达式
func testEmptyCondition(ctx *TestContext) TestResult {
	// 创建测试用户
	user := &models.UserInfo{
		ID:           "test_user_empty",
		Name:         "Test User Empty",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}
	if err := ctx.DB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("创建用户失败: %v", err)}
	}

	// 创建空条件策略
	strategy := &models.QuotaStrategy{
		Name:      "empty-condition-test",
		Title:     "空条件测试",
		Type:      "single",
		Amount:    10,
		Model:     "test-model",
		Condition: "", // 空条件
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("创建策略失败: %v", err)}
	}

	// 执行策略
	users := []models.UserInfo{*user}
	ctx.StrategyService.ExecStrategy(strategy, users)

	// 检查执行结果
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("期望执行1次，实际执行%d次", executeCount)}
	}

	return TestResult{Passed: true, Message: "空条件策略执行成功"}
}

// testMatchUserCondition 测试match-user条件
func testMatchUserCondition(ctx *TestContext) TestResult {
	// 创建测试用户
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
			return TestResult{Passed: false, Message: fmt.Sprintf("创建用户失败: %v", err)}
		}
	}

	// 创建match-user策略，只匹配第一个用户
	strategy := &models.QuotaStrategy{
		Name:      "match-user-test",
		Title:     "匹配用户测试",
		Type:      "single",
		Amount:    15,
		Model:     "test-model",
		Condition: `match-user("user_match_1")`,
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("创建策略失败: %v", err)}
	}

	// 执行策略
	userList := []models.UserInfo{*users[0], *users[1]}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// 检查执行结果 - 只有user_match_1应该被执行
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_match_1").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("user_match_1期望执行1次，实际执行%d次", executeCount)}
	}

	// 检查user_match_2不应该被执行
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_match_2").Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("user_match_2期望执行0次，实际执行%d次", executeCount)}
	}

	return TestResult{Passed: true, Message: "match-user条件测试成功"}
}

// testRegisterBeforeCondition 测试register-before条件
func testRegisterBeforeCondition(ctx *TestContext) TestResult {
	// 使用固定的时间点
	baseTime, _ := time.Parse("2006-01-02 15:04:05", "2025-01-01 12:00:00")
	cutoffTime := baseTime

	// 创建测试用户
	users := []*models.UserInfo{
		{
			ID:           "user_reg_before",
			Name:         "Early User",
			RegisterTime: baseTime.Add(-time.Hour * 2), // 在截止时间之前注册
			AccessTime:   baseTime,
		},
		{
			ID:           "user_reg_after",
			Name:         "Late User",
			RegisterTime: baseTime.Add(time.Hour * 2), // 在截止时间之后注册
			AccessTime:   baseTime,
		},
	}

	for _, user := range users {
		if err := ctx.DB.Create(user).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("创建用户失败: %v", err)}
		}
	}

	// 创建register-before策略
	strategy := &models.QuotaStrategy{
		Name:      "register-before-test",
		Title:     "注册时间测试",
		Type:      "single",
		Amount:    20,
		Model:     "test-model",
		Condition: fmt.Sprintf(`register-before("%s")`, cutoffTime.Format("2006-01-02 15:04:05")),
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("创建策略失败: %v", err)}
	}

	// 执行策略
	userList := []models.UserInfo{*users[0], *users[1]}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// 检查早期用户应该被执行
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_reg_before").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("早期用户期望执行1次，实际执行%d次", executeCount)}
	}

	// 检查晚期用户不应该被执行
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_reg_after").Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("晚期用户期望执行0次，实际执行%d次", executeCount)}
	}

	return TestResult{Passed: true, Message: "register-before条件测试成功"}
}

// testAccessAfterCondition 测试access-after条件
func testAccessAfterCondition(ctx *TestContext) TestResult {
	// 使用固定的时间点
	baseTime, _ := time.Parse("2006-01-02 15:04:05", "2025-01-01 12:00:00")
	cutoffTime := baseTime

	// 创建测试用户
	users := []*models.UserInfo{
		{
			ID:           "user_access_recent",
			Name:         "Recent User",
			RegisterTime: baseTime.Add(-time.Hour * 24),
			AccessTime:   baseTime.Add(time.Hour * 2), // 在截止时间之后访问
		},
		{
			ID:           "user_access_old",
			Name:         "Old User",
			RegisterTime: baseTime.Add(-time.Hour * 24),
			AccessTime:   baseTime.Add(-time.Hour * 2), // 在截止时间之前访问
		},
	}

	for _, user := range users {
		if err := ctx.DB.Create(user).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("创建用户失败: %v", err)}
		}
	}

	// 创建access-after策略
	strategy := &models.QuotaStrategy{
		Name:      "access-after-test",
		Title:     "最近访问测试",
		Type:      "single",
		Amount:    25,
		Model:     "test-model",
		Condition: fmt.Sprintf(`access-after("%s")`, cutoffTime.Format("2006-01-02 15:04:05")),
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("创建策略失败: %v", err)}
	}

	// 执行策略
	userList := []models.UserInfo{*users[0], *users[1]}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// 检查最近访问用户应该被执行
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_access_recent").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("最近访问用户期望执行1次，实际执行%d次", executeCount)}
	}

	// 检查旧访问用户不应该被执行
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_access_old").Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("旧访问用户期望执行0次，实际执行%d次", executeCount)}
	}

	return TestResult{Passed: true, Message: "access-after条件测试成功"}
}

// testGithubStarCondition 测试github-star条件
func testGithubStarCondition(ctx *TestContext) TestResult {
	// 创建测试用户
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
			return TestResult{Passed: false, Message: fmt.Sprintf("创建用户失败: %v", err)}
		}
	}

	// 创建github-star策略
	strategy := &models.QuotaStrategy{
		Name:      "github-star-test",
		Title:     "GitHub星标测试",
		Type:      "single",
		Amount:    30,
		Model:     "test-model",
		Condition: `github-star("zgsm")`,
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("创建策略失败: %v", err)}
	}

	// 执行策略
	userList := []models.UserInfo{*users[0], *users[1], *users[2]}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// 检查有星标用户应该被执行
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_star_yes").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("有星标用户期望执行1次，实际执行%d次", executeCount)}
	}

	// 检查无星标用户不应该被执行
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_star_no").Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("无星标用户期望执行0次，实际执行%d次", executeCount)}
	}

	// 检查空星标用户不应该被执行
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_star_empty").Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("空星标用户期望执行0次，实际执行%d次", executeCount)}
	}

	return TestResult{Passed: true, Message: "github-star条件测试成功"}
}

// testQuotaLECondition 测试quota-le条件
func testQuotaLECondition(ctx *TestContext) TestResult {
	// 创建测试用户
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
			return TestResult{Passed: false, Message: fmt.Sprintf("创建用户失败: %v", err)}
		}
	}

	// 设置mock配额
	mockStore.SetQuota("user_quota_low", 5)   // 低配额
	mockStore.SetQuota("user_quota_high", 50) // 高配额

	// 创建quota-le策略
	strategy := &models.QuotaStrategy{
		Name:      "quota-le-test",
		Title:     "配额低于测试",
		Type:      "single",
		Amount:    35,
		Model:     "test-model",
		Condition: `quota-le("test-model", 10)`,
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("创建策略失败: %v", err)}
	}

	// 执行策略
	userList := []models.UserInfo{*users[0], *users[1]}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// 检查低配额用户应该被执行
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_quota_low").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("低配额用户期望执行1次，实际执行%d次", executeCount)}
	}

	// 检查高配额用户不应该被执行
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_quota_high").Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("高配额用户期望执行0次，实际执行%d次", executeCount)}
	}

	return TestResult{Passed: true, Message: "quota-le条件测试成功"}
}

// testIsVipCondition 测试is-vip条件
func testIsVipCondition(ctx *TestContext) TestResult {
	// 创建测试用户
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
			return TestResult{Passed: false, Message: fmt.Sprintf("创建用户失败: %v", err)}
		}
	}

	// 创建is-vip策略
	strategy := &models.QuotaStrategy{
		Name:      "is-vip-test",
		Title:     "VIP等级测试",
		Type:      "single",
		Amount:    40,
		Model:     "test-model",
		Condition: `is-vip(2)`,
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("创建策略失败: %v", err)}
	}

	// 执行策略
	userList := []models.UserInfo{*users[0], *users[1], *users[2]}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// 检查高VIP用户应该被执行
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_vip_high").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("高VIP用户期望执行1次，实际执行%d次", executeCount)}
	}

	// 检查等于VIP用户应该被执行
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_vip_equal").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("等于VIP用户期望执行1次，实际执行%d次", executeCount)}
	}

	// 检查低VIP用户不应该被执行
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_vip_low").Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("低VIP用户期望执行0次，实际执行%d次", executeCount)}
	}

	return TestResult{Passed: true, Message: "is-vip条件测试成功"}
}

// testBelongToCondition 测试belong-to条件
func testBelongToCondition(ctx *TestContext) TestResult {
	// 创建测试用户
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
			return TestResult{Passed: false, Message: fmt.Sprintf("创建用户失败: %v", err)}
		}
	}

	// 创建belong-to策略
	strategy := &models.QuotaStrategy{
		Name:      "belong-to-test",
		Title:     "组织归属测试",
		Type:      "single",
		Amount:    45,
		Model:     "test-model",
		Condition: `belong-to("org001")`,
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("创建策略失败: %v", err)}
	}

	// 执行策略
	userList := []models.UserInfo{*users[0], *users[1], *users[2]}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// 检查目标组织用户应该被执行
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_org_target").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("目标组织用户期望执行1次，实际执行%d次", executeCount)}
	}

	// 检查其他组织用户不应该被执行
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_org_other").Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("其他组织用户期望执行0次，实际执行%d次", executeCount)}
	}

	// 检查无组织用户不应该被执行
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_org_empty").Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("无组织用户期望执行0次，实际执行%d次", executeCount)}
	}

	return TestResult{Passed: true, Message: "belong-to条件测试成功"}
}

// testAndCondition 测试and嵌套条件
func testAndCondition(ctx *TestContext) TestResult {
	// 创建测试用户
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
			return TestResult{Passed: false, Message: fmt.Sprintf("创建用户失败: %v", err)}
		}
	}

	// 创建and条件策略
	strategy := &models.QuotaStrategy{
		Name:      "and-condition-test",
		Title:     "AND条件测试",
		Type:      "single",
		Amount:    50,
		Model:     "test-model",
		Condition: `and(is-vip(2), github-star("zgsm"))`,
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("创建策略失败: %v", err)}
	}

	// 执行策略
	userList := []models.UserInfo{*users[0], *users[1], *users[2]}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// 检查同时满足两个条件的用户应该被执行
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_and_both").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("同时满足条件用户期望执行1次，实际执行%d次", executeCount)}
	}

	// 检查只满足VIP条件的用户不应该被执行
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_and_vip_only").Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("只满足VIP条件用户期望执行0次，实际执行%d次", executeCount)}
	}

	// 检查只满足星标条件的用户不应该被执行
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_and_star_only").Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("只满足星标条件用户期望执行0次，实际执行%d次", executeCount)}
	}

	return TestResult{Passed: true, Message: "and条件测试成功"}
}

// testOrCondition 测试or嵌套条件
func testOrCondition(ctx *TestContext) TestResult {
	// 创建测试用户
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
			return TestResult{Passed: false, Message: fmt.Sprintf("创建用户失败: %v", err)}
		}
	}

	// 创建or条件策略
	strategy := &models.QuotaStrategy{
		Name:      "or-condition-test",
		Title:     "OR条件测试",
		Type:      "single",
		Amount:    55,
		Model:     "test-model",
		Condition: `or(is-vip(2), belong-to("org001"))`,
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("创建策略失败: %v", err)}
	}

	// 执行策略
	userList := []models.UserInfo{*users[0], *users[1], *users[2], *users[3]}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// 检查同时满足两个条件的用户应该被执行
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_or_both").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("同时满足条件用户期望执行1次，实际执行%d次", executeCount)}
	}

	// 检查只满足VIP条件的用户应该被执行
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_or_vip_only").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("只满足VIP条件用户期望执行1次，实际执行%d次", executeCount)}
	}

	// 检查只满足组织条件的用户应该被执行
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_or_org_only").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("只满足组织条件用户期望执行1次，实际执行%d次", executeCount)}
	}

	// 检查都不满足条件的用户不应该被执行
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_or_neither").Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("都不满足条件用户期望执行0次，实际执行%d次", executeCount)}
	}

	return TestResult{Passed: true, Message: "or条件测试成功"}
}

// testNotCondition 测试not嵌套条件
func testNotCondition(ctx *TestContext) TestResult {
	// 创建测试用户
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
			return TestResult{Passed: false, Message: fmt.Sprintf("创建用户失败: %v", err)}
		}
	}

	// 创建not条件策略
	strategy := &models.QuotaStrategy{
		Name:      "not-condition-test",
		Title:     "NOT条件测试",
		Type:      "single",
		Amount:    60,
		Model:     "test-model",
		Condition: `not(is-vip(2))`,
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("创建策略失败: %v", err)}
	}

	// 执行策略
	userList := []models.UserInfo{*users[0], *users[1]}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// 检查VIP用户不应该被执行（被NOT排除）
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_not_vip").Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("VIP用户期望执行0次，实际执行%d次", executeCount)}
	}

	// 检查普通用户应该被执行
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_not_normal").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("普通用户期望执行1次，实际执行%d次", executeCount)}
	}

	return TestResult{Passed: true, Message: "not条件测试成功"}
}

// testComplexCondition 测试复杂嵌套条件
func testComplexCondition(ctx *TestContext) TestResult {
	// 创建测试用户
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
			return TestResult{Passed: false, Message: fmt.Sprintf("创建用户失败: %v", err)}
		}
	}

	// 创建复杂嵌套条件策略
	// (is-vip(3) AND github-star("zgsm")) OR (register-before("2024-01-01 00:00:00") AND belong-to("org002"))
	strategy := &models.QuotaStrategy{
		Name:      "complex-condition-test",
		Title:     "复杂条件测试",
		Type:      "single",
		Amount:    65,
		Model:     "test-model",
		Condition: `or(and(is-vip(3), github-star("zgsm")), and(register-before("2024-01-01 00:00:00"), belong-to("org002")))`,
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("创建策略失败: %v", err)}
	}

	// 执行策略
	userList := []models.UserInfo{*users[0], *users[1], *users[2]}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// 检查匹配第一个条件的用户应该被执行
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_complex_match1").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("匹配第一个条件用户期望执行1次，实际执行%d次", executeCount)}
	}

	// 检查匹配第二个条件的用户应该被执行
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_complex_match2").Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("匹配第二个条件用户期望执行1次，实际执行%d次", executeCount)}
	}

	// 检查不匹配任何条件的用户不应该被执行
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, "user_complex_no_match").Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("不匹配条件用户期望执行0次，实际执行%d次", executeCount)}
	}

	return TestResult{Passed: true, Message: "复杂条件测试成功"}
}

// testSingleTypeStrategy 测试单次充值策略
func testSingleTypeStrategy(ctx *TestContext) TestResult {
	// 创建测试用户
	user := &models.UserInfo{
		ID:           "user_single_test",
		Name:         "Single Test User",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}
	if err := ctx.DB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("创建用户失败: %v", err)}
	}

	// 创建单次充值策略
	strategy := &models.QuotaStrategy{
		Name:      "single-type-test",
		Title:     "单次充值测试",
		Type:      "single",
		Amount:    70,
		Model:     "test-model",
		Condition: "", // 空条件，所有用户都匹配
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("创建策略失败: %v", err)}
	}

	// 第一次执行策略
	users := []models.UserInfo{*user}
	ctx.StrategyService.ExecStrategy(strategy, users)

	// 检查第一次执行结果
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("第一次执行期望1次，实际执行%d次", executeCount)}
	}

	// 第二次执行策略（应该被跳过，因为已经执行过）
	ctx.StrategyService.ExecStrategy(strategy, users)

	// 检查第二次执行后的结果（应该仍然是1次）
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("单次策略重复执行，期望仍然是1次，实际执行%d次", executeCount)}
	}

	return TestResult{Passed: true, Message: "单次充值策略测试成功"}
}

// testPeriodicTypeStrategy 测试定时充值策略
func testPeriodicTypeStrategy(ctx *TestContext) TestResult {
	// 创建测试用户
	user := &models.UserInfo{
		ID:           "user_periodic_test",
		Name:         "Periodic Test User",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}
	if err := ctx.DB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("创建用户失败: %v", err)}
	}

	// 创建定时充值策略（每分钟执行一次，方便测试）
	strategy := &models.QuotaStrategy{
		Name:         "periodic-type-test",
		Title:        "定时充值测试",
		Type:         "periodic",
		Amount:       75,
		Model:        "test-model",
		PeriodicExpr: "* * * * *", // 每分钟执行一次
		Condition:    "",          // 空条件，所有用户都匹配
		Status:       true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("创建策略失败: %v", err)}
	}

	// 第一次执行策略
	users := []models.UserInfo{*user}
	ctx.StrategyService.ExecStrategy(strategy, users)

	// 检查第一次执行结果
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("第一次执行期望1次，实际执行%d次", executeCount)}
	}

	// 等待2秒后再次执行策略（定时策略可以重复执行）
	time.Sleep(2 * time.Second)
	ctx.StrategyService.ExecStrategy(strategy, users)

	// 检查第二次执行后的结果（应该是2次）
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)

	if executeCount != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("定时策略重复执行，期望2次，实际执行%d次", executeCount)}
	}

	return TestResult{Passed: true, Message: "定时充值策略测试成功"}
}

// testStrategyStatusControl 测试策略状态控制
func testStrategyStatusControl(ctx *TestContext) TestResult {
	// 创建测试用户
	user := &models.UserInfo{
		ID:           "user_status_test",
		Name:         "Status Test User",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}
	if err := ctx.DB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("创建用户失败: %v", err)}
	}

	// 创建禁用的策略
	strategy := &models.QuotaStrategy{
		Name:      "status-control-test",
		Title:     "状态控制测试",
		Type:      "single",
		Amount:    80,
		Model:     "test-model",
		Condition: "",    // 空条件
		Status:    false, // 禁用状态
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("创建策略失败: %v", err)}
	}

	// 执行禁用的策略
	users := []models.UserInfo{*user}
	ctx.StrategyService.ExecStrategy(strategy, users)

	// 检查禁用策略不应该被执行
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("禁用策略期望执行0次，实际执行%d次", executeCount)}
	}

	// 启用策略
	strategy.Status = true
	if err := ctx.DB.Save(strategy).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("更新策略状态失败: %v", err)}
	}

	// 再次执行策略
	ctx.StrategyService.ExecStrategy(strategy, users)

	// 检查启用后策略应该被执行
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("启用策略期望执行1次，实际执行%d次", executeCount)}
	}

	return TestResult{Passed: true, Message: "策略状态控制测试成功"}
}

// testAiGatewayFailure 测试AiGateway请求失败
func testAiGatewayFailure(ctx *TestContext) TestResult {
	// 创建测试用户
	user := &models.UserInfo{
		ID:           "user_gateway_fail",
		Name:         "Gateway Fail User",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}
	if err := ctx.DB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("创建用户失败: %v", err)}
	}

	// 创建使用失败网关的策略服务
	failGateway := aigateway.NewClient(ctx.FailServer.URL, "/v1/chat/completions", "credential3")
	failStrategyService := services.NewStrategyService(ctx.DB, failGateway)

	// 创建策略
	strategy := &models.QuotaStrategy{
		Name:      "gateway-failure-test",
		Title:     "网关失败测试",
		Type:      "single",
		Amount:    85,
		Model:     "test-model",
		Condition: "", // 空条件
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("创建策略失败: %v", err)}
	}

	// 使用失败的网关执行策略
	users := []models.UserInfo{*user}
	failStrategyService.ExecStrategy(strategy, users)

	// 检查执行记录存在但状态为失败
	var execute models.QuotaExecute
	err := ctx.DB.Where("strategy_id = ? AND user_id = ? AND status = 'failed'", strategy.ID, user.ID).First(&execute).Error

	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("未找到执行记录: %v", err)}
	}

	if execute.Status != "failed" {
		return TestResult{Passed: false, Message: fmt.Sprintf("期望状态为failed，实际状态为%s", execute.Status)}
	}

	return TestResult{Passed: true, Message: "AiGateway失败测试成功"}
}

// testBatchUserProcessing 测试批量用户处理
func testBatchUserProcessing(ctx *TestContext) TestResult {
	// 创建多个测试用户
	users := make([]*models.UserInfo, 10)
	for i := 0; i < 10; i++ {
		users[i] = &models.UserInfo{
			ID:           fmt.Sprintf("batch_user_%03d", i),
			Name:         fmt.Sprintf("Batch User %d", i),
			VIP:          i % 4, // VIP等级0-3
			RegisterTime: time.Now().Add(-time.Hour * 24),
			AccessTime:   time.Now().Add(-time.Hour * 1),
		}
		if err := ctx.DB.Create(users[i]).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("创建用户%d失败: %v", i, err)}
		}
	}

	// 创建VIP用户策略
	strategy := &models.QuotaStrategy{
		Name:      "batch-processing-test",
		Title:     "批量处理测试",
		Type:      "single",
		Amount:    90,
		Model:     "test-model",
		Condition: `is-vip(2)`, // VIP等级>=2的用户
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("创建策略失败: %v", err)}
	}

	// 执行策略
	userList := make([]models.UserInfo, len(users))
	for i, user := range users {
		userList[i] = *user
	}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// 检查执行结果 - 应该有4个用户被执行（VIP等级2和3的用户）
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND status = 'completed'", strategy.ID).Count(&executeCount)

	expectedCount := int64(4) // VIP等级2,3的用户各2个，共4个
	if executeCount != expectedCount {
		return TestResult{Passed: false, Message: fmt.Sprintf("批量处理期望执行%d次，实际执行%d次", expectedCount, executeCount)}
	}

	// 验证具体执行的用户
	var executes []models.QuotaExecute
	ctx.DB.Where("strategy_id = ?", strategy.ID).Find(&executes)

	executedUsers := make(map[string]bool)
	for _, exec := range executes {
		executedUsers[exec.User] = true
	}

	// 检查VIP>=2的用户都被执行了
	for _, user := range users {
		shouldExecute := user.VIP >= 2
		wasExecuted := executedUsers[user.ID]

		if shouldExecute != wasExecuted {
			return TestResult{
				Passed:  false,
				Message: fmt.Sprintf("用户%s VIP%d，期望执行:%v，实际执行:%v", user.ID, user.VIP, shouldExecute, wasExecuted),
			}
		}
	}

	return TestResult{Passed: true, Message: "批量用户处理测试成功"}
}

// printTestResults 打印测试结果
func printTestResults(results []TestResult) {
	fmt.Println("\n=== 测试结果摘要 ===")

	totalTests := len(results)
	passedTests := 0
	totalDuration := time.Duration(0)

	for _, result := range results {
		if result.Passed {
			passedTests++
		}
		totalDuration += result.Duration
	}

	fmt.Printf("总测试数: %d\n", totalTests)
	fmt.Printf("通过测试: %d\n", passedTests)
	fmt.Printf("失败测试: %d\n", totalTests-passedTests)
	fmt.Printf("总耗时: %.2fs\n", totalDuration.Seconds())
	fmt.Printf("成功率: %.1f%%\n", float64(passedTests)/float64(totalTests)*100)

	if passedTests != totalTests {
		fmt.Println("\n失败的测试:")
		for _, result := range results {
			if !result.Passed {
				fmt.Printf("❌ %s: %s\n", result.TestName, result.Message)
			}
		}
	} else {
		fmt.Println("\n所有测试都通过了！")
	}
}
