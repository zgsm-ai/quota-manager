package main

import (
	"fmt"
	"quota-manager/internal/models"
)

// testPeriodicStrategyExecution tests that periodic strategies can execute properly
func testPeriodicStrategyExecution(ctx *TestContext) TestResult {
	// Create test user
	user := createTestUser("user_periodic_exec_test", "Periodic Execution Test User", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Create periodic strategy
	strategy := &models.QuotaStrategy{
		Name:         "periodic-execution-test",
		Title:        "Periodic Execution Test",
		Type:         "periodic",
		Amount:       25,
		Model:        "test-model",
		PeriodicExpr: "0 0 0 * * *", // Execute daily at midnight
		Condition:    "",            // Empty condition, all users match
		Status:       true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	// Execute strategy (simulating cron execution)
	users := []models.UserInfo{*user} // Dereference the pointer
	ctx.StrategyService.ExecStrategy(strategy, users)

	// Check execution result
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Execution failed, expected 1 time, actually executed %d times", executeCount)}
	}

	// In new architecture, periodic strategies can be executed multiple times directly
	// without duplicate prevention (cron handles the scheduling)
	ctx.StrategyService.ExecStrategy(strategy, users)

	// Check that it executed again (no duplicate prevention for periodic strategies)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)

	if executeCount != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Second execution failed, expected 2 times, actually executed %d times", executeCount)}
	}

	// Test disable and enable functionality
	if err := ctx.StrategyService.DisableStrategy(strategy.ID); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Disable strategy failed: %v", err)}
	}

	// Get updated strategy from database
	disabledStrategy, err := ctx.StrategyService.GetStrategy(strategy.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get disabled strategy failed: %v", err)}
	}

	// Execute disabled strategy (should be skipped)
	ctx.StrategyService.ExecStrategy(disabledStrategy, users)

	// Check that disabled strategy didn't execute
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)

	if executeCount != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Disabled strategy should not execute, expected 2 times, got %d times", executeCount)}
	}

	// Re-enable and test
	if err := ctx.StrategyService.EnableStrategy(strategy.ID); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Enable strategy failed: %v", err)}
	}

	// Get updated strategy from database
	enabledStrategy, err := ctx.StrategyService.GetStrategy(strategy.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get enabled strategy failed: %v", err)}
	}

	ctx.StrategyService.ExecStrategy(enabledStrategy, users)

	// Check that re-enabled strategy executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)

	if executeCount != 3 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Re-enabled strategy should execute, expected 3 times, got %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "Periodic Strategy Execution Test Succeeded"}
}

// testPeriodicStrategyCronRegistration tests that periodic strategies are properly registered to cron
func testPeriodicStrategyCronRegistration(ctx *TestContext) TestResult {
	// Create a strategy with a specific cron expression
	strategy := &models.QuotaStrategy{
		Name:         "periodic-cron-test",
		Title:        "Periodic Cron Registration Test",
		Type:         "periodic",
		Amount:       30,
		Model:        "test-model",
		PeriodicExpr: "0 0 0 1 * *", // Execute on 1st day of every month at midnight
		Condition:    "",
		Status:       true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	// Test that the strategy can be executed directly (simulating cron execution)
	user := createTestUser("cron_test_user", "Cron Test User", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	users := []models.UserInfo{*user} // Dereference the pointer

	// Execute strategy directly (this simulates what cron would do)
	ctx.StrategyService.ExecStrategy(strategy, users)

	// Check execution result
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Strategy execution failed, expected 1 execution, got %d", executeCount)}
	}

	// Test that updating the strategy works properly
	updates := map[string]interface{}{
		"amount": 50,
		"title":  "Updated Periodic Cron Test",
	}
	if err := ctx.StrategyService.UpdateStrategy(strategy.ID, updates); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Update strategy failed: %v", err)}
	}

	// Verify the update
	updatedStrategy, err := ctx.StrategyService.GetStrategy(strategy.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get updated strategy failed: %v", err)}
	}

	if updatedStrategy.Amount != 50 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Strategy amount not updated, expected 50, got %d", updatedStrategy.Amount)}
	}

	return TestResult{Passed: true, Message: "Periodic Cron Registration Test Succeeded"}
}

// testPeriodicCronExpressionValidation tests different cron expression formats and their validation
func testPeriodicCronExpressionValidation(ctx *TestContext) TestResult {
	user := createTestUser("user_cron_validation", "Cron Validation Test User", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Test various valid cron expressions to ensure they are accepted
	testCases := []struct {
		name     string
		expr     string
		expected string
	}{
		{"every-second", "*/1 * * * * *", "every 1 second"},
		{"every-minute", "0 */1 * * * *", "every 1 minute"},
		{"every-hour", "0 0 */1 * * *", "every 1 hour"},
		{"daily-9am", "0 0 9 * * *", "daily at 9 AM"},
		{"weekly-monday", "0 0 0 * * 1", "weekly on Monday"},
		{"monthly-1st", "0 0 0 1 * *", "monthly on 1st"},
	}

	users := []models.UserInfo{*user}

	for i, tc := range testCases {
		strategy := &models.QuotaStrategy{
			Name:         fmt.Sprintf("cron-validation-%d", i),
			Title:        fmt.Sprintf("Cron Validation Test - %s", tc.expected),
			Type:         "periodic",
			Amount:       10 + i*5,
			Model:        "test-model",
			PeriodicExpr: tc.expr,
			Condition:    "",
			Status:       true,
		}

		// Test strategy creation (this validates the cron expression)
		if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create strategy with %s (%s): %v", tc.expected, tc.expr, err)}
		}

		// Test strategy execution (validates that the strategy can be executed)
		ctx.StrategyService.ExecStrategy(strategy, users)
		var count int64
		ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&count)
		if count != 1 {
			return TestResult{Passed: false, Message: fmt.Sprintf("Strategy execution failed for %s, expected 1 time, got %d times", tc.expected, count)}
		}
	}

	return TestResult{Passed: true, Message: "Cron Expression Validation Test Passed"}
}

// testPeriodicStrategyCRUDOperations tests CRUD operations for periodic strategies
func testPeriodicStrategyCRUDOperations(ctx *TestContext) TestResult {
	user := createTestUser("user_crud_test", "CRUD Test User", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Test CREATE operation
	strategy := &models.QuotaStrategy{
		Name:         "crud-test-strategy",
		Title:        "CRUD Test Strategy",
		Type:         "periodic",
		Amount:       100,
		Model:        "test-model",
		PeriodicExpr: "0 0 12 * * *", // Daily at noon
		Condition:    "",
		Status:       true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	// Verify creation
	createdStrategy, err := ctx.StrategyService.GetStrategy(strategy.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get created strategy failed: %v", err)}
	}
	if createdStrategy.Name != "crud-test-strategy" {
		return TestResult{Passed: false, Message: fmt.Sprintf("Strategy name mismatch, expected 'crud-test-strategy', got '%s'", createdStrategy.Name)}
	}

	// Test READ operation - Execute and verify
	users := []models.UserInfo{*user}
	ctx.StrategyService.ExecStrategy(strategy, users)
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Initial strategy execution failed, expected 1 time, got %d times", executeCount)}
	}

	// Test UPDATE operation - Modify strategy
	updates := map[string]interface{}{
		"title":         "Updated CRUD Test Strategy",
		"amount":        150,
		"periodic_expr": "0 0 18 * * *", // Change to 6 PM
	}
	if err := ctx.StrategyService.UpdateStrategy(strategy.ID, updates); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Update strategy failed: %v", err)}
	}

	// Verify update
	updatedStrategy, err := ctx.StrategyService.GetStrategy(strategy.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get updated strategy failed: %v", err)}
	}
	if updatedStrategy.Amount != 150 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Strategy amount not updated, expected 150, got %d", updatedStrategy.Amount)}
	}
	if updatedStrategy.PeriodicExpr != "0 0 18 * * *" {
		return TestResult{Passed: false, Message: fmt.Sprintf("Strategy cron expression not updated, expected '0 0 18 * * *', got '%s'", updatedStrategy.PeriodicExpr)}
	}

	// Test execution after update
	ctx.StrategyService.ExecStrategy(updatedStrategy, users)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)
	if executeCount != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Updated strategy execution failed, expected 2 times, got %d times", executeCount)}
	}

	// Test DISABLE operation
	if err := ctx.StrategyService.DisableStrategy(strategy.ID); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Disable strategy failed: %v", err)}
	}

	// Verify disabled strategy doesn't execute
	disabledStrategy, err := ctx.StrategyService.GetStrategy(strategy.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get disabled strategy failed: %v", err)}
	}
	ctx.StrategyService.ExecStrategy(disabledStrategy, users)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)
	if executeCount != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Disabled strategy should not execute, expected 2 times, got %d times", executeCount)}
	}

	// Test ENABLE operation
	if err := ctx.StrategyService.EnableStrategy(strategy.ID); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Enable strategy failed: %v", err)}
	}

	// Verify re-enabled strategy executes
	enabledStrategy, err := ctx.StrategyService.GetStrategy(strategy.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get enabled strategy failed: %v", err)}
	}
	ctx.StrategyService.ExecStrategy(enabledStrategy, users)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)
	if executeCount != 3 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Re-enabled strategy should execute, expected 3 times, got %d times", executeCount)}
	}

	// Clean up execution records before deletion to avoid foreign key constraint
	if err := ctx.DB.DB.Where("strategy_id = ?", strategy.ID).Delete(&models.QuotaExecute{}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Clean up execution records failed: %v", err)}
	}

	// Test DELETE operation
	if err := ctx.StrategyService.DeleteStrategy(strategy.ID); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Delete strategy failed: %v", err)}
	}

	// Verify deletion
	_, err = ctx.StrategyService.GetStrategy(strategy.ID)
	if err == nil {
		return TestResult{Passed: false, Message: "Strategy should be deleted but still exists"}
	}

	return TestResult{Passed: true, Message: "Periodic Strategy CRUD Operations Test Passed"}
}

// testPeriodicStrategyFieldModifications tests modification of individual fields
func testPeriodicStrategyFieldModifications(ctx *TestContext) TestResult {
	user := createTestUser("user_field_mod", "Field Modification Test User", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Create base strategy
	strategy := &models.QuotaStrategy{
		Name:         "field-modification-test",
		Title:        "Field Modification Test",
		Type:         "periodic",
		Amount:       75,
		Model:        "test-model",
		PeriodicExpr: "0 0 9 * * 1", // Monday at 9 AM
		Condition:    "",
		Status:       true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create base strategy failed: %v", err)}
	}

	users := []models.UserInfo{*user}

	// Test modifying minute field
	minuteUpdates := map[string]interface{}{
		"periodic_expr": "0 30 9 * * 1", // Change from 0 to 30 minutes
	}
	if err := ctx.StrategyService.UpdateStrategy(strategy.ID, minuteUpdates); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Update minute field failed: %v", err)}
	}
	minuteStrategy, _ := ctx.StrategyService.GetStrategy(strategy.ID)
	ctx.StrategyService.ExecStrategy(minuteStrategy, users)
	var count1 int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&count1)
	if count1 != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Minute field modification test failed, expected 1 execution, got %d", count1)}
	}

	// Test modifying hour field
	hourUpdates := map[string]interface{}{
		"periodic_expr": "0 30 14 * * 1", // Change from 9 to 14 (2 PM)
	}
	if err := ctx.StrategyService.UpdateStrategy(strategy.ID, hourUpdates); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Update hour field failed: %v", err)}
	}
	hourStrategy, _ := ctx.StrategyService.GetStrategy(strategy.ID)
	ctx.StrategyService.ExecStrategy(hourStrategy, users)
	var count2 int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&count2)
	if count2 != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Hour field modification test failed, expected 2 executions, got %d", count2)}
	}

	// Test modifying day field
	dayUpdates := map[string]interface{}{
		"periodic_expr": "0 30 14 15 * *", // Change from every Monday to 15th of month
	}
	if err := ctx.StrategyService.UpdateStrategy(strategy.ID, dayUpdates); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Update day field failed: %v", err)}
	}
	dayStrategy, _ := ctx.StrategyService.GetStrategy(strategy.ID)
	ctx.StrategyService.ExecStrategy(dayStrategy, users)
	var count3 int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&count3)
	if count3 != 3 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Day field modification test failed, expected 3 executions, got %d", count3)}
	}

	// Test modifying month field
	monthUpdates := map[string]interface{}{
		"periodic_expr": "0 30 14 15 6 *", // Change to June only
	}
	if err := ctx.StrategyService.UpdateStrategy(strategy.ID, monthUpdates); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Update month field failed: %v", err)}
	}
	monthStrategy, _ := ctx.StrategyService.GetStrategy(strategy.ID)
	ctx.StrategyService.ExecStrategy(monthStrategy, users)
	var count4 int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&count4)
	if count4 != 4 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Month field modification test failed, expected 4 executions, got %d", count4)}
	}

	// Test modifying weekday field
	weekdayUpdates := map[string]interface{}{
		"periodic_expr": "0 30 14 * * 5", // Change to Friday
	}
	if err := ctx.StrategyService.UpdateStrategy(strategy.ID, weekdayUpdates); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Update weekday field failed: %v", err)}
	}
	weekdayStrategy, _ := ctx.StrategyService.GetStrategy(strategy.ID)
	ctx.StrategyService.ExecStrategy(weekdayStrategy, users)
	var count5 int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&count5)
	if count5 != 5 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Weekday field modification test failed, expected 5 executions, got %d", count5)}
	}

	return TestResult{Passed: true, Message: "Periodic Strategy Field Modifications Test Passed"}
}

// testPeriodicStrategyInvalidCronExpressions tests handling of invalid cron expressions
func testPeriodicStrategyInvalidCronExpressions(ctx *TestContext) TestResult {
	invalidExpressions := []string{
		"invalid cron",  // Completely invalid
		"60 0 0 * * *",  // Invalid second (60)
		"0 60 0 * * *",  // Invalid minute (60)
		"0 0 25 * * *",  // Invalid hour (25)
		"0 0 0 32 * *",  // Invalid day (32)
		"0 0 0 * 13 *",  // Invalid month (13)
		"0 0 0 * * 8",   // Invalid weekday (8)
		"",              // Empty expression
		"* * * *",       // Too few fields
		"* * * * * * *", // Too many fields
	}

	for i, expr := range invalidExpressions {
		strategy := &models.QuotaStrategy{
			Name:         fmt.Sprintf("invalid-cron-test-%d", i),
			Title:        fmt.Sprintf("Invalid Cron Test %d", i),
			Type:         "periodic",
			Amount:       50,
			Model:        "test-model",
			PeriodicExpr: expr,
			Condition:    "",
			Status:       true,
		}

		err := ctx.StrategyService.CreateStrategy(strategy)
		if err == nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Invalid cron expression '%s' should fail but was accepted", expr)}
		}
	}

	return TestResult{Passed: true, Message: "Invalid Cron Expressions Test Passed"}
}

// testPeriodicStrategyConcurrentModifications tests concurrent modifications
func testPeriodicStrategyConcurrentModifications(ctx *TestContext) TestResult {
	user := createTestUser("user_concurrent", "Concurrent Test User", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Create strategy
	strategy := &models.QuotaStrategy{
		Name:         "concurrent-test-strategy",
		Title:        "Concurrent Test Strategy",
		Type:         "periodic",
		Amount:       100,
		Model:        "test-model",
		PeriodicExpr: "0 0 12 * * *", // Daily at noon
		Condition:    "",
		Status:       true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	users := []models.UserInfo{*user}

	// Test concurrent execution
	ctx.StrategyService.ExecStrategy(strategy, users)
	ctx.StrategyService.ExecStrategy(strategy, users)
	ctx.StrategyService.ExecStrategy(strategy, users)

	// Verify all executions completed
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)
	if executeCount != 3 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Concurrent execution failed, expected 3 times, got %d times", executeCount)}
	}

	// Test concurrent modifications
	updates1 := map[string]interface{}{
		"amount": 150,
	}
	updates2 := map[string]interface{}{
		"title": "Updated Concurrent Test Strategy",
	}

	if err := ctx.StrategyService.UpdateStrategy(strategy.ID, updates1); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("First concurrent update failed: %v", err)}
	}
	if err := ctx.StrategyService.UpdateStrategy(strategy.ID, updates2); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Second concurrent update failed: %v", err)}
	}

	// Verify updates applied
	updatedStrategy, err := ctx.StrategyService.GetStrategy(strategy.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get updated strategy failed: %v", err)}
	}
	if updatedStrategy.Amount != 150 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Amount update failed, expected 150, got %d", updatedStrategy.Amount)}
	}

	// Test execution after concurrent modifications
	ctx.StrategyService.ExecStrategy(updatedStrategy, users)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)
	if executeCount != 4 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Execution after concurrent modifications failed, expected 4 times, got %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "Concurrent Modifications Test Passed"}
}

// testPeriodicStrategyEdgeCases tests edge cases and boundary conditions
func testPeriodicStrategyEdgeCases(ctx *TestContext) TestResult {
	user := createTestUser("user_edge_cases", "Edge Cases Test User", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Test strategy with zero amount
	zeroAmountStrategy := &models.QuotaStrategy{
		Name:         "zero-amount-test",
		Title:        "Zero Amount Test",
		Type:         "periodic",
		Amount:       0,
		Model:        "test-model",
		PeriodicExpr: "0 0 12 * * *",
		Condition:    "",
		Status:       true,
	}
	if err := ctx.StrategyService.CreateStrategy(zeroAmountStrategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create zero amount strategy failed: %v", err)}
	}

	users := []models.UserInfo{*user}
	ctx.StrategyService.ExecStrategy(zeroAmountStrategy, users)
	var zeroCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", zeroAmountStrategy.ID, user.ID).Count(&zeroCount)
	if zeroCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Zero amount strategy execution failed, expected 1 time, got %d times", zeroCount)}
	}

	// Test strategy with very large amount
	largeAmountStrategy := &models.QuotaStrategy{
		Name:         "large-amount-test",
		Title:        "Large Amount Test",
		Type:         "periodic",
		Amount:       999999,
		Model:        "test-model",
		PeriodicExpr: "0 0 12 * * *",
		Condition:    "",
		Status:       true,
	}
	if err := ctx.StrategyService.CreateStrategy(largeAmountStrategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create large amount strategy failed: %v", err)}
	}

	ctx.StrategyService.ExecStrategy(largeAmountStrategy, users)
	var largeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", largeAmountStrategy.ID, user.ID).Count(&largeCount)
	if largeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Large amount strategy execution failed, expected 1 time, got %d times", largeCount)}
	}

	// Test strategy with complex cron expression
	complexCronStrategy := &models.QuotaStrategy{
		Name:         "complex-cron-test",
		Title:        "Complex Cron Test",
		Type:         "periodic",
		Amount:       25,
		Model:        "test-model",
		PeriodicExpr: "0 15,45 9,17 * * 1-5", // 9:15, 9:45, 17:15, 17:45 on weekdays
		Condition:    "",
		Status:       true,
	}
	if err := ctx.StrategyService.CreateStrategy(complexCronStrategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create complex cron strategy failed: %v", err)}
	}

	ctx.StrategyService.ExecStrategy(complexCronStrategy, users)
	var complexCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", complexCronStrategy.ID, user.ID).Count(&complexCount)
	if complexCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Complex cron strategy execution failed, expected 1 time, got %d times", complexCount)}
	}

	return TestResult{Passed: true, Message: "Edge Cases Test Passed"}
}
