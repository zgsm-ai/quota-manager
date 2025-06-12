package main

import (
	"fmt"
	"time"

	"quota-manager/internal/models"
)

// testEmptyCondition test empty condition expression
func testEmptyCondition(ctx *TestContext) TestResult {
	// Create test user
	user := createTestUser("test_user_empty", "Test User Empty", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
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
	// Create test users using createTestUser function
	user1 := createTestUser("user_match_1", "Match User 1", 0)
	user2 := createTestUser("user_match_2", "Match User 2", 0)

	users := []*models.UserInfo{user1, user2}

	for _, user := range users {
		if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
		}
	}

	// Create match-user strategy, only match the first user by UUID
	strategy := &models.QuotaStrategy{
		Name:      "match-user-test",
		Title:     "Match User Test",
		Type:      "single",
		Amount:    15,
		Model:     "test-model",
		Condition: fmt.Sprintf(`match-user("%s")`, user1.ID),
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	// Execute strategy
	userList := []models.UserInfo{*user1, *user2}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// Check execution result - only user1 should be executed
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user1.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("user1 expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check user2 should not be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user2.ID).Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("user2 expected execution 0 times, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "match-user condition test succeeded"}
}

// testRegisterBeforeCondition test register-before condition
func testRegisterBeforeCondition(ctx *TestContext) TestResult {
	// Use fixed time point
	baseTime, _ := time.Parse("2006-01-02 15:04:05", "2025-01-01 12:00:00")
	cutoffTime := baseTime

	// Create test users using createTestUser function
	userEarly := createTestUser("user_reg_before", "Early User", 0)
	userLate := createTestUser("user_reg_after", "Late User", 0)

	// Set different created times to test the register-before condition
	userEarly.CreatedAt = baseTime.Add(-time.Hour * 24) // Before cutoff time
	userLate.CreatedAt = baseTime.Add(time.Hour * 24)   // After cutoff time

	users := []*models.UserInfo{userEarly, userLate}

	for _, user := range users {
		if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
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
	userList := []models.UserInfo{*userEarly, *userLate}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// Check early user should be executed
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userEarly.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Early user expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check late user should not be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userLate.ID).Count(&executeCount)

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

	// Create test users using createTestUser function
	userRecent := createTestUser("user_access_recent", "Recent User", 0)
	userOld := createTestUser("user_access_old", "Old User", 0)

	// Set different access times to test the access-after condition
	userRecent.AccessTime = baseTime.Add(time.Hour * 24) // After cutoff time
	userOld.AccessTime = baseTime.Add(-time.Hour * 24)   // Before cutoff time

	users := []*models.UserInfo{userRecent, userOld}

	for _, user := range users {
		if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
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
	userList := []models.UserInfo{*userRecent, *userOld}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// Check recent access user should be executed
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userRecent.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Recent access user expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check old access user should not be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userOld.ID).Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Old access user expected execution 0 times, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "access-after condition test succeeded"}
}

// testGithubStarCondition test github-star condition
func testGithubStarCondition(ctx *TestContext) TestResult {
	// Create test users using createTestUser function
	userStarYes := createTestUser("user_star_yes", "Starred User", 0)
	userStarYes.GithubStar = "zgsm,openai/gpt-4,facebook/react"

	userStarNo := createTestUser("user_star_no", "Non-starred User", 0)
	userStarNo.GithubStar = "microsoft/vscode,google/tensorflow"

	userStarEmpty := createTestUser("user_star_empty", "Empty Star User", 0)
	userStarEmpty.GithubStar = ""

	users := []*models.UserInfo{userStarYes, userStarNo, userStarEmpty}

	for _, user := range users {
		if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
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
	userList := []models.UserInfo{*userStarYes, *userStarNo, *userStarEmpty}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// Check starred user should be executed
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userStarYes.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Starred user expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check non-starred user should not be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userStarNo.ID).Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Non-starred user expected execution 0 times, actually executed %d times", executeCount)}
	}

	// Check empty starred user should not be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userStarEmpty.ID).Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Empty starred user expected execution 0 times, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "github-star condition test succeeded"}
}

// testQuotaLECondition test quota-le condition
func testQuotaLECondition(ctx *TestContext) TestResult {
	// Create test users using createTestUser function
	userQuotaLow := createTestUser("user_quota_low", "Low Quota User", 0)
	userQuotaHigh := createTestUser("user_quota_high", "High Quota User", 0)

	users := []*models.UserInfo{userQuotaLow, userQuotaHigh}

	for _, user := range users {
		if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
		}
	}

	// Set mock quota using the actual user IDs
	mockStore.SetQuota(userQuotaLow.ID, 5)   // Low quota
	mockStore.SetQuota(userQuotaHigh.ID, 50) // High quota

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
	userList := []models.UserInfo{*userQuotaLow, *userQuotaHigh}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// Check low quota user should be executed
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userQuotaLow.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Low quota user expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check high quota user should not be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userQuotaHigh.ID).Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("High quota user expected execution 0 times, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "quota-le condition test succeeded"}
}

// testIsVipCondition test is-vip condition
func testIsVipCondition(ctx *TestContext) TestResult {
	// Create test users using createTestUser function
	userVipHigh := createTestUser("user_vip_high", "High VIP User", 3)
	userVipLow := createTestUser("user_vip_low", "Low VIP User", 0)
	userVipEqual := createTestUser("user_vip_equal", "Equal VIP User", 2)

	users := []*models.UserInfo{userVipHigh, userVipLow, userVipEqual}

	for _, user := range users {
		if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
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
	userList := []models.UserInfo{*userVipHigh, *userVipLow, *userVipEqual}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// Check high VIP user should be executed
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userVipHigh.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("High VIP user expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check equal VIP user should be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userVipEqual.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Equal VIP user expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check low VIP user should not be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userVipLow.ID).Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Low VIP user expected execution 0 times, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "is-vip condition test succeeded"}
}
