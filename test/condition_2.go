package main

import (
	"fmt"
	"quota-manager/internal/models"
	"time"
)

// testBelongToCondition test belong-to condition
func testBelongToCondition(ctx *TestContext) TestResult {
	// Create test users
	users := []*models.UserInfo{
		{
			ID:           "user_org_target",
			Name:         "Target Org User",
			Org:          "org001",
			RegisterTime: time.Now().Truncate(time.Second).Add(-time.Hour * 24),
			AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
		},
		{
			ID:           "user_org_other",
			Name:         "Other Org User",
			Org:          "org002",
			RegisterTime: time.Now().Truncate(time.Second).Add(-time.Hour * 24),
			AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
		},
		{
			ID:           "user_org_empty",
			Name:         "No Org User",
			Org:          "",
			RegisterTime: time.Now().Truncate(time.Second).Add(-time.Hour * 24),
			AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
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
			RegisterTime: time.Now().Truncate(time.Second).Add(-time.Hour * 24),
			AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
		},
		{
			ID:           "user_and_vip_only",
			Name:         "VIP Only User",
			VIP:          3,
			GithubStar:   "microsoft/vscode",
			RegisterTime: time.Now().Truncate(time.Second).Add(-time.Hour * 24),
			AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
		},
		{
			ID:           "user_and_star_only",
			Name:         "Star Only User",
			VIP:          0,
			GithubStar:   "zgsm,facebook/react",
			RegisterTime: time.Now().Truncate(time.Second).Add(-time.Hour * 24),
			AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
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
			RegisterTime: time.Now().Truncate(time.Second).Add(-time.Hour * 24),
			AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
		},
		{
			ID:           "user_or_vip_only",
			Name:         "VIP Only User",
			VIP:          2,
			Org:          "org002",
			RegisterTime: time.Now().Truncate(time.Second).Add(-time.Hour * 24),
			AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
		},
		{
			ID:           "user_or_org_only",
			Name:         "Org Only User",
			VIP:          0,
			Org:          "org001",
			RegisterTime: time.Now().Truncate(time.Second).Add(-time.Hour * 24),
			AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
		},
		{
			ID:           "user_or_neither",
			Name:         "Neither User",
			VIP:          0,
			Org:          "org002",
			RegisterTime: time.Now().Truncate(time.Second).Add(-time.Hour * 24),
			AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
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
			RegisterTime: time.Now().Truncate(time.Second).Add(-time.Hour * 24),
			AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
		},
		{
			ID:           "user_not_normal",
			Name:         "Normal User",
			VIP:          0,
			RegisterTime: time.Now().Truncate(time.Second).Add(-time.Hour * 24),
			AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
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
			RegisterTime: time.Now().Truncate(time.Second).Add(-time.Hour * 24),
			AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
		},
		{
			ID:           "user_complex_match2",
			Name:         "Complex Match 2",
			VIP:          0,
			GithubStar:   "",
			Org:          "org002",
			RegisterTime: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
		},
		{
			ID:           "user_complex_no_match",
			Name:         "Complex No Match",
			VIP:          1,
			GithubStar:   "microsoft/vscode",
			Org:          "org003",
			RegisterTime: time.Now().Truncate(time.Second).Add(-time.Hour * 24),
			AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
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
