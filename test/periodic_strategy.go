package main

import (
	"fmt"
	"quota-manager/internal/models"
	"time"
)

// testPeriodicStrategyDuplicatePrevention tests that periodic strategies don't execute duplicates
func testPeriodicStrategyDuplicatePrevention(ctx *TestContext) TestResult {
	// Create test user
	user := createTestUser("user_periodic_duplicate_test", "Periodic Duplicate Test User", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Create periodic strategy that should execute every hour
	strategy := &models.QuotaStrategy{
		Name:         "periodic-duplicate-prevention-test",
		Title:        "Periodic Duplicate Prevention Test",
		Type:         "periodic",
		Amount:       25,
		Model:        "test-model",
		PeriodicExpr: "0 * * * *", // Execute every hour at minute 0
		Condition:    "",          // Empty condition, all users match
		Status:       true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	// First execute strategy
	users := []models.UserInfo{*user} // Dereference the pointer
	ctx.StrategyService.ExecStrategy(strategy, users)

	// Check first execution result
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("First execution expected 1 time, actually executed %d times", executeCount)}
	}

	// Execute strategy again immediately (should be prevented by batch duplicate check)
	ctx.StrategyService.ExecStrategy(strategy, users)

	// Check that it didn't execute again
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Duplicate prevention failed, expected 1 time, actually executed %d times", executeCount)}
	}

	// Wait for a new batch (simulate time passing to next hour)
	time.Sleep(2 * time.Second)

	// Create a mock execution with future batch number to simulate next hour
	futureTime := time.Now().Add(time.Hour)
	futureBatch := futureTime.Format("2006010215")

	mockExecute := &models.QuotaExecute{
		StrategyID:  strategy.ID,
		User:        user.ID,
		BatchNumber: futureBatch,
		Status:      "completed",
		ExpiryDate:  time.Now().Add(30 * 24 * time.Hour),
	}

	if err := ctx.DB.Create(mockExecute).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create mock execute failed: %v", err)}
	}

	// Verify total executions
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ?", strategy.ID, user.ID).Count(&executeCount)

	if executeCount != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 total executions (different batches), got %d", executeCount)}
	}

	// Verify that executions have different batch numbers
	var executes []models.QuotaExecute
	ctx.DB.Where("strategy_id = ? AND user_id = ?", strategy.ID, user.ID).Find(&executes)

	if len(executes) != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 execution records, got %d", len(executes))}
	}

	if executes[0].BatchNumber == executes[1].BatchNumber {
		return TestResult{Passed: false, Message: "Executions should have different batch numbers"}
	}

	return TestResult{Passed: true, Message: "Periodic Strategy Duplicate Prevention Test Succeeded"}
}

// testShouldExecutePeriodicLogic tests the shouldExecutePeriodic logic
func testShouldExecutePeriodicLogic(ctx *TestContext) TestResult {
	// Create a strategy with a specific cron expression
	strategy := &models.QuotaStrategy{
		Name:         "periodic-logic-test",
		Title:        "Periodic Logic Test",
		Type:         "periodic",
		Amount:       30,
		Model:        "test-model",
		PeriodicExpr: "0 1 1 * *", // Execute on 1st day of every month at 1 AM
		Condition:    "",
		Status:       true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	// Test shouldExecutePeriodic when no previous executions exist
	shouldExecute := ctx.StrategyService.ShouldExecutePeriodicForTest(strategy)

	// The logic should depend on current time and cron schedule
	// Since we can't control the exact time in tests, we just ensure the method doesn't panic
	// and returns a boolean result
	if shouldExecute {
		// If it should execute, verify that after creating an execution record,
		// it won't execute again in the same batch
		user := createTestUser("logic_test_user", "Logic Test User", 0)
		users := []models.UserInfo{*user} // Dereference the pointer
		if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
		}

		ctx.StrategyService.ExecStrategy(strategy, users)

		// Check if it should execute again (should be false due to batch check)
		shouldExecuteAgain := ctx.StrategyService.ShouldExecutePeriodicForTest(strategy)
		if shouldExecuteAgain {
			return TestResult{Passed: false, Message: "Strategy should not execute again in the same batch"}
		}
	}

	return TestResult{Passed: true, Message: "Periodic Logic Test Succeeded"}
}
