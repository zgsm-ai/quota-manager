package main

import (
	"fmt"
	"time"

	"quota-manager/internal/config"
	"quota-manager/internal/models"
	"quota-manager/internal/services"
	"quota-manager/pkg/aigateway"
)

// testSingleTypeStrategy test single recharge strategy
func testSingleTypeStrategy(ctx *TestContext) TestResult {
	// Create test user
	user := createTestUser("user_single_test", "Single Test User", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
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

	// Verify strategy name in audit record
	if err := verifyStrategyNameInAudit(ctx, user.ID, "single-type-test", models.OperationRecharge); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Strategy name verification failed: %v", err)}
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
	user := createTestUser("user_periodic_test", "Periodic Test User", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
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

	// Verify strategy name in audit records (should have 2 records)
	if err := verifyAuditRecordCount(ctx, user.ID, 2); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Audit record count verification failed: %v", err)}
	}

	// Verify strategy name in latest audit record
	if err := verifyStrategyNameInAudit(ctx, user.ID, "periodic-type-test", models.OperationRecharge); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Strategy name verification failed: %v", err)}
	}

	return TestResult{Passed: true, Message: "Periodic Recharge Strategy Test Succeeded"}
}

// testStrategyStatusControl test strategy status control
func testStrategyStatusControl(ctx *TestContext) TestResult {
	// Create test user
	user := createTestUser("user_status_test", "Status Test User", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
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

	// Verify strategy name in audit record
	if err := verifyStrategyNameInAudit(ctx, user.ID, "status-control-test", models.OperationRecharge); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Strategy name verification failed: %v", err)}
	}

	return TestResult{Passed: true, Message: "Strategy Status Control Test Succeeded"}
}

// testAiGatewayFailure test AiGateway request failure
func testAiGatewayFailure(ctx *TestContext) TestResult {
	// Create test user
	user := createTestUser("user_gateway_fail", "Gateway Fail User", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Create mock AiGateway config pointing to the fail server
	failAiGatewayConfig := &config.AiGatewayConfig{
		BaseURL:    ctx.FailServer.URL,
		AdminPath:  "/v1/chat/completions",
		AuthHeader: "X-Auth-Key",
		AuthValue:  "credential3",
	}

	// Create services using failed gateway configuration
	failQuotaService := services.NewQuotaService(ctx.DB.DB, failAiGatewayConfig, ctx.VoucherService)
	failGateway := aigateway.NewClient(ctx.FailServer.URL, "/v1/chat/completions", "X-Auth-Key", "credential3")
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
		users[i] = createTestUser(fmt.Sprintf("batch_user_%03d", i), fmt.Sprintf("Batch User %d", i), i%4)
		if err := ctx.DB.AuthDB.Create(users[i]).Error; err != nil {
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
