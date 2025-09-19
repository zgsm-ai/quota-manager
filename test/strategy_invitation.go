package main

import (
	"fmt"
	"time"

	"quota-manager/internal/models"
)

// testExpiryDaysStrategy test strategy custom expiry days functionality
func testExpiryDaysStrategy(ctx *TestContext) TestResult {
	fmt.Println("Starting custom expiry days test")

	// Create test user
	user := createTestUser("expiry_days_test_user", "ExpiryDays Test User", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create test user: %v", err)}
	}
	defer func() {
		ctx.DB.AuthDB.Delete(user, user.ID)
	}()

	// Test 1: Create strategy with custom expiry period (7 days)
	strategy1 := &models.QuotaStrategy{
		Name:       "test-expiry-days-7",
		Title:      "Test ExpiryDays 7 Days",
		Type:       "single",
		Amount:     50,
		Model:      "test-model",
		Condition:  "true()",
		Status:     true,
		ExpiryDays: &[]int{7}[0],
	}
	if err := ctx.StrategyService.CreateStrategy(strategy1); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create 7-day expiry strategy: %v", err)}
	}
	defer func() {
		ctx.DB.DB.Delete(strategy1, strategy1.ID)
	}()

	// Test 2: Create strategy without custom expiry period (default end-of-month)
	strategy2 := &models.QuotaStrategy{
		Name:      "test-default-expiry",
		Title:     "Test Default Expiry",
		Type:      "single",
		Amount:    30,
		Model:     "test-model",
		Condition: "true()",
		Status:    true,
		// ExpiryDays is nil, use default end-of-month expiry
	}
	if err := ctx.StrategyService.CreateStrategy(strategy2); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create default expiry strategy: %v", err)}
	}
	defer func() {
		ctx.DB.DB.Delete(strategy2, strategy2.ID)
	}()

	// Execute strategy 1 (7-day expiry)
	users := []models.UserInfo{*user}
	fmt.Printf("Executing strategy 1 (ID: %d, ExpiryDays: %v)\n", strategy1.ID, strategy1.ExpiryDays)
	ctx.StrategyService.ExecStrategy(strategy1, users)

	// Execute strategy 2 (default end-of-month expiry)
	fmt.Printf("Executing strategy 2 (ID: %d, ExpiryDays: %v)\n", strategy2.ID, strategy2.ExpiryDays)
	ctx.StrategyService.ExecStrategy(strategy2, users)

	// Check strategy execution records
	var executes []models.QuotaExecute
	if err := ctx.DB.DB.Where("user_id = ?", user.ID).Find(&executes).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to query strategy execution records: %v", err)}
	}
	fmt.Printf("Strategy execution record count: %d\n", len(executes))
	for i, exec := range executes {
		fmt.Printf("Execution record %d: StrategyID=%d, Status=%s\n", i+1, exec.StrategyID, exec.Status)
		if exec.Status == "failed" {
			fmt.Printf("Strategy execution failed! Possibly due to AiGateway call failure causing transaction rollback\n")
		}
	}

	// Verify quota record expiry times
	var quotas []models.Quota
	if err := ctx.DB.DB.Where("user_id = ?", user.ID).Find(&quotas).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to query quota records: %v", err)}
	}

	fmt.Printf("Quota record count: %d\n", len(quotas))
	if len(quotas) != 2 {
		// If quota records are incorrect, check execution record status
		var failedExecs []models.QuotaExecute
		ctx.DB.DB.Where("user_id = ? AND status = ?", user.ID, "failed").Find(&failedExecs)
		if len(failedExecs) > 0 {
			return TestResult{Passed: false, Message: fmt.Sprintf("Strategy execution failed, expected 2 quota records, got %d. %d execution records failed", len(quotas), len(failedExecs))}
		}
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 quota records, got %d", len(quotas))}
	}

	// Get current time to calculate expected expiry time (using configured timezone)
	now := time.Now() // Use system timezone in test environment

	// Verify 7-day expiry quota
	var sevenDayQuota *models.Quota
	var defaultQuota *models.Quota

	for i := range quotas {
		if quotas[i].Amount == 50 {
			sevenDayQuota = &quotas[i]
		} else if quotas[i].Amount == 30 {
			defaultQuota = &quotas[i]
		}
	}

	if sevenDayQuota == nil {
		return TestResult{Passed: false, Message: "7-day expiry quota record not found"}
	}
	if defaultQuota == nil {
		return TestResult{Passed: false, Message: "Default expiry quota record not found"}
	}

	// Verify 7-day expiry quota expiration time (should be 23:59:59 after 7 days)
	expectedSevenDayExpiry := time.Date(now.Year(), now.Month(), now.Day()+7, 23, 59, 59, 0, now.Location())
	if !sevenDayQuota.ExpiryDate.Equal(expectedSevenDayExpiry) {
		return TestResult{
			Passed: false,
			Message: fmt.Sprintf("7-day expiry quota expiration time incorrect: expected %v, actual %v",
				expectedSevenDayExpiry, sevenDayQuota.ExpiryDate),
		}
	}

	// Verify default expiry quota expiration time (should be end-of-month 23:59:59)
	expectedDefaultExpiry := time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, now.Location())
	if !defaultQuota.ExpiryDate.Equal(expectedDefaultExpiry) {
		return TestResult{
			Passed: false,
			Message: fmt.Sprintf("Default expiry quota expiration time incorrect: expected %v, actual %v",
				expectedDefaultExpiry, defaultQuota.ExpiryDate),
		}
	}

	// Clean up test data
	// ctx.DB.DB.Delete(&quotas) // Temporarily commented for data inspection

	fmt.Println("Custom expiry days test completed")
	return TestResult{Passed: true, Message: "ExpiryDays Strategy Test Succeeded"}
}

// testStrategyInviteRegister test strategy invite register
func testStrategyInviteRegister(ctx *TestContext) TestResult {
	// Create test inviter user
	inviterUser := createTestInviterUser("invite_user_register_inviter_test", "Inviter User Test", 0, "")
	if err2 := ctx.DB.AuthDB.Create(inviterUser).Error; err2 != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err2)}
	}
	// Create test invited user
	user := createTestInviterUser("invite_user_register_invited_test", "Invited User Test", 0, inviterUser.ID)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Debug: verify if inviter ID is correctly set
	if user.InviterID == "" {
		return TestResult{Passed: false, Message: "InviterID is empty after user creation"}
	}
	if user.InviterID != inviterUser.ID {
		return TestResult{Passed: false, Message: fmt.Sprintf("InviterID mismatch: expected %s, got %s", inviterUser.ID, user.InviterID)}
	}

	// Debug: verify user data in database
	var dbUser models.UserInfo
	if err := ctx.DB.AuthDB.Where("id = ?", user.ID).First(&dbUser).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to query user from database: %v", err)}
	}
	if dbUser.InviterID == "" {
		return TestResult{Passed: false, Message: "InviterID is empty in database"}
	}
	if dbUser.InviterID != inviterUser.ID {
		return TestResult{Passed: false, Message: fmt.Sprintf("Database InviterID mismatch: expected %s, got %s", inviterUser.ID, dbUser.InviterID)}
	}

	// Create single recharge strategy
	strategy := &models.QuotaStrategy{
		Name:   "inviter-register-reward",
		Title:  "Inviter Register Reward",
		Type:   "single",
		Amount: 25,
		Model:  "test-model",
		// Condition: "true()", // Always true condition, all users match
		Condition: `has-inviter()`,
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}
	// return TestResult{Passed: true, Message: "Single Recharge Strategy Test Succeeded"}

	// user.ID = "635be6cb-3aee-4a56-9c82-abfeaf3283e3"
	// strategy.ID = 1
	// First execute strategy
	users := []models.UserInfo{*user}
	ctx.StrategyService.ExecStrategy(strategy, users)

	// Check first execution result
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("First execution expected 1 time, actually executed %d times", executeCount)}
	}

	// Verify strategy name in audit record (for invitation strategy, check inviter's audit record)
	if err := verifyStrategyNameInAudit(ctx, inviterUser.ID, "inviter-register-reward", models.OperationRecharge); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Strategy name verification failed: %v", err)}
	}

	// Second execute strategy (should be skipped because it has already been executed)
	ctx.StrategyService.ExecStrategy(strategy, users)

	// Check second execution result (should still be 1 time)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Single strategy repeated execution, expected still 1 time, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "Invitation Register Reward Strategy Test Succeeded"}
}

// testStrategyInviteStar test strategy invite star reward (reward to inviter when invited user stars)
func testStrategyInviteStar(ctx *TestContext) TestResult {
	// Create test inviter user
	inviterUser := createTestInviterUser("inviter_star_reward_inviter_test", "Invite Star Reward Test", 0, "")
	if err2 := ctx.DB.AuthDB.Create(inviterUser).Error; err2 != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create inviter user failed: %v", err2)}
	}

	// Create test invited user with GitHub star
	user := createTestInviterUser("inviter_star_reward_invited_test", "Invited User For Invite Star Test", 0, inviterUser.ID)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create invited user failed: %v", err)}
	}

	// Create invite star reward strategy (reward goes to inviter)
	strategy := &models.QuotaStrategy{
		Name:      "inviter-star-reward",
		Title:     "Inviter Star Reward",
		Type:      "single",
		Amount:    75,
		Model:     "test-model",
		Condition: `and(has-inviter(), github-star("zgsm-ai.zgsm"))`,
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
		return TestResult{Passed: false, Message: fmt.Sprintf("First execution expected 1 time, actually executed %d times", executeCount)}
	}

	// Verify strategy name in audit record (for invitation strategy, check inviter's audit record)
	if err := verifyStrategyNameInAudit(ctx, inviterUser.ID, "inviter-star-reward", models.OperationRecharge); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Strategy name verification failed: %v", err)}
	}

	// Second execute strategy (should be skipped because it has already been executed)
	ctx.StrategyService.ExecStrategy(strategy, users)

	// Check second execution result (should still be 1 time)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Single strategy repeated execution, expected still 1 time, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "Inviter Star Reward Strategy Test Succeeded"}
}

// testStrategyInviteUserStar test strategy invited user star reward (reward to invited user)
func testStrategyInviteUserStar(ctx *TestContext) TestResult {
	// Create test inviter user
	inviterUser := createTestInviterUser("invite_user_star_inviter_test", "Inviter User Star Test", 0, "")
	if err2 := ctx.DB.AuthDB.Create(inviterUser).Error; err2 != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create inviter user failed: %v", err2)}
	}

	// Create test invited user with GitHub star
	user := createTestInviterUser("invite_user_star_invited_test", "Invited User Star Test", 0, inviterUser.ID)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create invited user failed: %v", err)}
	}

	// Create invited user star reward strategy (reward goes to invited user, not inviter)
	strategy := &models.QuotaStrategy{
		Name:      "invitee-star-reward",
		Title:     "Invitee Star Reward",
		Type:      "single",
		Amount:    25,
		Model:     "test-model",
		Condition: `and(has-inviter(), github-star("zgsm-ai.zgsm"))`,
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
		return TestResult{Passed: false, Message: fmt.Sprintf("First execution expected 1 time, actually executed %d times", executeCount)}
	}

	// Verify strategy name in audit record (for invitee strategy, check invited user's audit record)
	if err := verifyStrategyNameInAudit(ctx, user.ID, "invitee-star-reward", models.OperationRecharge); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Strategy name verification failed: %v", err)}
	}

	// Second execute strategy (should be skipped because it has already been executed)
	ctx.StrategyService.ExecStrategy(strategy, users)

	// Check second execution result (should still be 1 time)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Single strategy repeated execution, expected still 1 time, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "Invited User Star Reward Strategy Test Succeeded"}
}
