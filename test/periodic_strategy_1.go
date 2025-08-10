package main

import (
	"fmt"
	"quota-manager/internal/models"
)

// testPeriodicStrategyMaxExecLimitOne: max_exec_per_user = 1, execute twice -> only first succeeds
func testPeriodicStrategyMaxExecLimitOne(ctx *TestContext) TestResult {
	user := createTestUser("user_max1", "MaxExec One User", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	strategy := &models.QuotaStrategy{
		Name:           "periodic-max1",
		Title:          "Periodic Max 1",
		Type:           "periodic",
		Amount:         10,
		Model:          "test-model",
		PeriodicExpr:   "0 0 0 * * *",
		Condition:      "true()",
		Status:         true,
		MaxExecPerUser: 1,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	users := []models.UserInfo{*user}
	ctx.StrategyService.ExecStrategy(strategy, users)
	ctx.StrategyService.ExecStrategy(strategy, users)

	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).
		Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).
		Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 execution due to max=1, got %d", executeCount)}
	}
	return TestResult{Passed: true, Message: "MaxExecPerUser=1 works"}
}

// testPeriodicStrategyMaxExecLimitTwo: max_exec_per_user = 2, execute three times -> first two succeed
func testPeriodicStrategyMaxExecLimitTwo(ctx *TestContext) TestResult {
	user := createTestUser("user_max2", "MaxExec Two User", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	strategy := &models.QuotaStrategy{
		Name:           "periodic-max2",
		Title:          "Periodic Max 2",
		Type:           "periodic",
		Amount:         15,
		Model:          "test-model",
		PeriodicExpr:   "0 0 0 * * *",
		Condition:      "true()",
		Status:         true,
		MaxExecPerUser: 2,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	users := []models.UserInfo{*user}
	ctx.StrategyService.ExecStrategy(strategy, users)
	ctx.StrategyService.ExecStrategy(strategy, users)
	ctx.StrategyService.ExecStrategy(strategy, users)

	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).
		Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).
		Count(&executeCount)

	if executeCount != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 executions due to max=2, got %d", executeCount)}
	}
	return TestResult{Passed: true, Message: "MaxExecPerUser=2 works"}
}

// testPeriodicStrategyMaxExecUpdateUpDown: raise then lower max to test behavior
func testPeriodicStrategyMaxExecUpdateUpDown(ctx *TestContext) TestResult {
	user := createTestUser("user_max_update", "MaxExec Update User", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	strategy := &models.QuotaStrategy{
		Name:           "periodic-max-update",
		Title:          "Periodic Max Update",
		Type:           "periodic",
		Amount:         20,
		Model:          "test-model",
		PeriodicExpr:   "0 0 0 * * *",
		Condition:      "true()",
		Status:         true,
		MaxExecPerUser: 1,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	users := []models.UserInfo{*user}
	// First exec (1/1)
	ctx.StrategyService.ExecStrategy(strategy, users)

	// Raise limit to 2
	if err := ctx.StrategyService.UpdateStrategy(strategy.ID, map[string]interface{}{"max_exec_per_user": 2}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Update max to 2 failed: %v", err)}
	}
	updated, _ := ctx.StrategyService.GetStrategy(strategy.ID)
	ctx.StrategyService.ExecStrategy(updated, users) // second exec (2/2)

	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).
		Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).
		Count(&executeCount)
	if executeCount != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("After raise to 2, expected 2 executions, got %d", executeCount)}
	}

	// Lower limit to 1 (less than completed). Further exec should be skipped.
	if err := ctx.StrategyService.UpdateStrategy(strategy.ID, map[string]interface{}{"max_exec_per_user": 1}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Lower max to 1 failed: %v", err)}
	}
	lowered, _ := ctx.StrategyService.GetStrategy(strategy.ID)
	ctx.StrategyService.ExecStrategy(lowered, users)

	executeCount = 0
	ctx.DB.Model(&models.QuotaExecute{}).
		Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).
		Count(&executeCount)
	if executeCount != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("After lower to 1 (< completed), should remain 2, got %d", executeCount)}
	}

	return TestResult{Passed: true, Message: "MaxExecPerUser raise/lower works"}
}

// testPeriodicStrategyMaxExecEndToEnd: create with limit, run one schedule (simulate) and verify quota and records
func testPeriodicStrategyMaxExecEndToEnd(ctx *TestContext) TestResult {
	user := createTestUser("user_max_e2e", "MaxExec E2E User", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	strategy := &models.QuotaStrategy{
		Name:           "periodic-max-e2e",
		Title:          "Periodic Max E2E",
		Type:           "periodic",
		Amount:         33,
		Model:          "test-model",
		PeriodicExpr:   "0 0 0 * * *",
		Condition:      "true()",
		Status:         true,
		MaxExecPerUser: 1,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	// Simulate one scheduler tick by calling ExecStrategy
	users := []models.UserInfo{*user}
	ctx.StrategyService.ExecStrategy(strategy, users)

	// Verify execution record
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).
		Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).
		Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("E2E first run expected 1 record, got %d", executeCount)}
	}

	// Verify AiGateway total quota increased by amount (mock server)
	quota, err := ctx.Gateway.QueryQuotaValue(user.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Query AiGateway quota failed: %v", err)}
	}
	if int(quota) != 33 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected AiGateway quota 33, got %v", quota)}
	}

	// Second run should be skipped due to max=1
	ctx.StrategyService.ExecStrategy(strategy, users)
	executeCount = 0
	ctx.DB.Model(&models.QuotaExecute{}).
		Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).
		Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("E2E second run should be skipped, records still 1, got %d", executeCount)}
	}

	return TestResult{Passed: true, Message: "E2E periodic with limit works"}
}
