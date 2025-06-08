package main

import (
	"fmt"
	"time"

	"quota-manager/internal/models"
)

// testEmptyCondition test empty condition expression
func testEmptyCondition(ctx *TestContext) TestResult {
	// Create test user
	user := &models.UserInfo{
		ID:           "test_user_empty",
		Name:         "Test User Empty",
		RegisterTime: time.Now().Truncate(time.Second).Add(-time.Hour * 24),
		AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
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
			RegisterTime: time.Now().Truncate(time.Second).Add(-time.Hour * 24),
			AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
		},
		{
			ID:           "user_match_2",
			Name:         "Match User 2",
			RegisterTime: time.Now().Truncate(time.Second).Add(-time.Hour * 24),
			AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
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
			RegisterTime: time.Now().Truncate(time.Second).Add(-time.Hour * 24),
			AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
		},
		{
			ID:           "user_star_no",
			Name:         "Non-starred User",
			GithubStar:   "microsoft/vscode,google/tensorflow",
			RegisterTime: time.Now().Truncate(time.Second).Add(-time.Hour * 24),
			AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
		},
		{
			ID:           "user_star_empty",
			Name:         "Empty Star User",
			GithubStar:   "",
			RegisterTime: time.Now().Truncate(time.Second).Add(-time.Hour * 24),
			AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
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
			RegisterTime: time.Now().Truncate(time.Second).Add(-time.Hour * 24),
			AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
		},
		{
			ID:           "user_quota_high",
			Name:         "High Quota User",
			RegisterTime: time.Now().Truncate(time.Second).Add(-time.Hour * 24),
			AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
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
			RegisterTime: time.Now().Truncate(time.Second).Add(-time.Hour * 24),
			AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
		},
		{
			ID:           "user_vip_low",
			Name:         "Low VIP User",
			VIP:          0,
			RegisterTime: time.Now().Truncate(time.Second).Add(-time.Hour * 24),
			AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
		},
		{
			ID:           "user_vip_equal",
			Name:         "Equal VIP User",
			VIP:          2,
			RegisterTime: time.Now().Truncate(time.Second).Add(-time.Hour * 24),
			AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
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
