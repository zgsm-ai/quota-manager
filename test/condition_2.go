package main

import (
	"fmt"
	"quota-manager/internal/models"
	"time"
)

// testBelongToCondition test belong-to condition
func testBelongToCondition(ctx *TestContext) TestResult {
	// Create test users using createTestUser function
	userOrgTarget := createTestUser("user_org_target", "Target Org User", 0)
	userOrgTarget.Company = "org001" // Set target organization

	userOrgOther := createTestUser("user_org_other", "Other Org User", 0)
	userOrgOther.Company = "org002" // Set different organization

	userOrgEmpty := createTestUser("user_org_empty", "No Org User", 0)
	// No organization set for this user

	users := []*models.UserInfo{userOrgTarget, userOrgOther, userOrgEmpty}

	for _, user := range users {
		if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
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
	userList := []models.UserInfo{*userOrgTarget, *userOrgOther, *userOrgEmpty}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// Check target organization user should be executed
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userOrgTarget.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Target organization user expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check other organization user should not be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userOrgOther.ID).Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Other organization user expected execution 0 times, actually executed %d times", executeCount)}
	}

	// Check no organization user should not be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userOrgEmpty.ID).Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("No organization user expected execution 0 times, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "belong-to condition test succeeded"}
}

// testAndCondition test and nesting condition
func testAndCondition(ctx *TestContext) TestResult {
	// Create test users using createTestUser function
	userAndBoth := createTestUser("user_and_both", "Both Conditions User", 2)
	userAndBoth.GithubStar = "zgsm,openai/gpt-4"

	userAndVipOnly := createTestUser("user_and_vip_only", "VIP Only User", 3)
	userAndVipOnly.GithubStar = "microsoft/vscode"

	userAndStarOnly := createTestUser("user_and_star_only", "Star Only User", 0)
	userAndStarOnly.GithubStar = "zgsm,facebook/react"

	users := []*models.UserInfo{userAndBoth, userAndVipOnly, userAndStarOnly}

	for _, user := range users {
		if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
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
	userList := []models.UserInfo{*userAndBoth, *userAndVipOnly, *userAndStarOnly}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// Check users simultaneously satisfying both conditions should be executed
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userAndBoth.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Users simultaneously satisfying conditions expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check users satisfying only VIP condition should not be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userAndVipOnly.ID).Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Users satisfying only VIP condition expected execution 0 times, actually executed %d times", executeCount)}
	}

	// Check users satisfying only star condition should not be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userAndStarOnly.ID).Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Users satisfying only star condition expected execution 0 times, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "and condition test succeeded"}
}

// testOrCondition test or nesting condition
func testOrCondition(ctx *TestContext) TestResult {
	// Create test users using createTestUser function
	userOrBoth := createTestUser("user_or_both", "Both Conditions User", 3)
	userOrBoth.Company = "org001" // Both VIP and belong to org001

	userOrVipOnly := createTestUser("user_or_vip_only", "VIP Only User", 2)
	// No organization set

	userOrOrgOnly := createTestUser("user_or_org_only", "Org Only User", 0)
	userOrOrgOnly.Company = "org001" // Only belong to org001

	userOrNeither := createTestUser("user_or_neither", "Neither User", 0)
	// Neither VIP nor belong to org001

	users := []*models.UserInfo{userOrBoth, userOrVipOnly, userOrOrgOnly, userOrNeither}

	for _, user := range users {
		if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
		}
	}

	// Create or condition strategy with unique name
	uniqueStrategyName := fmt.Sprintf("or-condition-test-%d", time.Now().UnixNano())
	strategy := &models.QuotaStrategy{
		Name:      uniqueStrategyName,
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
	userList := []models.UserInfo{*userOrBoth, *userOrVipOnly, *userOrOrgOnly, *userOrNeither}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// Check users simultaneously satisfying both conditions should be executed
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userOrBoth.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Users simultaneously satisfying conditions expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check users satisfying only VIP condition should be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userOrVipOnly.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Users satisfying only VIP condition expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check users satisfying only organization condition should be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userOrOrgOnly.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Users satisfying only organization condition expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check users not satisfying any condition should not be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userOrNeither.ID).Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Users not satisfying any condition expected execution 0 times, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "or condition test succeeded"}
}

// testNotCondition test not nesting condition
func testNotCondition(ctx *TestContext) TestResult {
	// Create test users using createTestUser function
	userNotVip := createTestUser("user_not_vip", "VIP User", 3)
	userNotNormal := createTestUser("user_not_normal", "Normal User", 0)

	users := []*models.UserInfo{userNotVip, userNotNormal}

	for _, user := range users {
		if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
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
	userList := []models.UserInfo{*userNotVip, *userNotNormal}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// Check VIP user should not be executed (excluded by NOT)
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userNotVip.ID).Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("VIP user expected execution 0 times, actually executed %d times", executeCount)}
	}

	// Check normal user should be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userNotNormal.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Normal user expected execution 1 time, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "NOT condition test succeeded"}
}

// testComplexCondition test complex nesting condition
func testComplexCondition(ctx *TestContext) TestResult {
	// Create test users using createTestUser function
	userComplexMatch1 := createTestUser("user_complex_match1", "Complex Match 1", 2)
	userComplexMatch1.GithubStar = "zgsm,openai/gpt-4"

	userComplexMatch2 := createTestUser("user_complex_match2", "Complex Match 2", 0)
	userComplexMatch2.Company = "org001"

	userComplexNoMatch := createTestUser("user_complex_no_match", "No Match User", 1)
	userComplexNoMatch.GithubStar = "microsoft/vscode"

	users := []*models.UserInfo{userComplexMatch1, userComplexMatch2, userComplexNoMatch}

	for _, user := range users {
		if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
		}
	}

	// Create complex condition strategy: (VIP >= 2 AND github-star("zgsm")) OR belong-to("org001")
	strategy := &models.QuotaStrategy{
		Name:      "complex-condition-test",
		Title:     "Complex Condition Test",
		Type:      "single",
		Amount:    65,
		Model:     "test-model",
		Condition: `or(and(is-vip(2), github-star("zgsm")), belong-to("org001"))`,
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	// Execute strategy
	userList := []models.UserInfo{*userComplexMatch1, *userComplexMatch2, *userComplexNoMatch}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// Check first user (VIP + github star) should be executed
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userComplexMatch1.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User with VIP+star expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check second user (belong to org) should be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userComplexMatch2.ID).Count(&executeCount)

	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User with org membership expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check third user (no match) should not be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userComplexNoMatch.ID).Count(&executeCount)

	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User with no match expected execution 0 times, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "complex condition test succeeded"}
}

// testAndOrNestingCondition tests AND and OR nested conditions
func testAndOrNestingCondition(ctx *TestContext) TestResult {
	// Create test users using createTestUser function
	userAllConditions := createTestUser("user_and_or_all", "All Conditions User", 3)
	userAllConditions.Company = "org001"
	userAllConditions.GithubStar = "zgsm,openai/gpt-4"

	userVipAndStar := createTestUser("user_and_or_vip_star", "VIP Star User", 2)
	userVipAndStar.GithubStar = "zgsm,microsoft/vscode"

	userVipAndOrg := createTestUser("user_and_or_vip_org", "VIP Org User", 2)
	userVipAndOrg.Company = "org001"

	userOnlyOrg := createTestUser("user_and_or_org", "Only Org User", 0)
	userOnlyOrg.Company = "org001"

	userOnlyStar := createTestUser("user_and_or_star", "Only Star User", 0)
	userOnlyStar.GithubStar = "zgsm,google/tensorflow"

	userNoConditions := createTestUser("user_and_or_none", "No Conditions User", 0)

	users := []*models.UserInfo{
		userAllConditions,
		userVipAndStar,
		userVipAndOrg,
		userOnlyOrg,
		userOnlyStar,
		userNoConditions,
	}

	for _, user := range users {
		if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
		}
	}

	// Create strategy with AND + OR nested condition
	// Condition: (VIP >= 2 AND github-star("zgsm")) OR belong-to("org001")
	uniqueStrategyName := fmt.Sprintf("and-or-nesting-test-%d", time.Now().UnixNano())
	strategy := &models.QuotaStrategy{
		Name:      uniqueStrategyName,
		Title:     "AND + OR Nesting Test",
		Type:      "single",
		Amount:    100,
		Model:     "test-model",
		Condition: `or(and(is-vip(2), github-star("zgsm")), belong-to("org001"))`,
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

	// Check execution results
	// 1. User with all conditions should be executed (satisfies both AND and OR parts)
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userAllConditions.ID).Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User with all conditions expected execution 1 time, actually executed %d times", executeCount)}
	}

	// 2. User with VIP and Star should be executed (satisfies AND part)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userVipAndStar.ID).Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User with VIP and Star expected execution 1 time, actually executed %d times", executeCount)}
	}

	// 3. User with VIP and Org should be executed (satisfies OR part)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userVipAndOrg.ID).Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User with VIP and Org expected execution 1 time, actually executed %d times", executeCount)}
	}

	// 4. User with only Org should be executed (satisfies OR part)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userOnlyOrg.ID).Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User with only Org expected execution 1 time, actually executed %d times", executeCount)}
	}

	// 5. User with only Star should not be executed (doesn't satisfy either part)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userOnlyStar.ID).Count(&executeCount)
	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User with only Star expected execution 0 times, actually executed %d times", executeCount)}
	}

	// 6. User with no conditions should not be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userNoConditions.ID).Count(&executeCount)
	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User with no conditions expected execution 0 times, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "AND + OR nesting condition test succeeded"}
}

// testOrNotNestingCondition tests OR and NOT nested conditions
func testOrNotNestingCondition(ctx *TestContext) TestResult {
	// Create test users using createTestUser function
	userVipNoStar := createTestUser("user_or_not_vip_no_star", "VIP No Star User", 3)
	userVipNoStar.GithubStar = "microsoft/vscode,google/tensorflow"

	userVipWithStar := createTestUser("user_or_not_vip_star", "VIP With Star User", 3)
	userVipWithStar.GithubStar = "zgsm,openai/gpt-4"

	userLowVipNoStar := createTestUser("user_or_not_low_vip_no_star", "Low VIP No Star User", 1)
	userLowVipNoStar.GithubStar = "microsoft/vscode"

	userLowVipWithStar := createTestUser("user_or_not_low_vip_star", "Low VIP With Star User", 1)
	userLowVipWithStar.GithubStar = "zgsm,facebook/react"

	userNoVipNoStar := createTestUser("user_or_not_no_vip_no_star", "No VIP No Star User", 0)
	userNoVipNoStar.GithubStar = "microsoft/vscode"

	users := []*models.UserInfo{
		userVipNoStar,
		userVipWithStar,
		userLowVipNoStar,
		userLowVipWithStar,
		userNoVipNoStar,
	}

	for _, user := range users {
		if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
		}
	}

	// Create strategy with OR + NOT nested condition
	// Condition: is-vip(3) OR not(github-star("zgsm"))
	uniqueStrategyName := fmt.Sprintf("or-not-nesting-test-%d", time.Now().UnixNano())
	strategy := &models.QuotaStrategy{
		Name:      uniqueStrategyName,
		Title:     "OR + NOT Nesting Test",
		Type:      "single",
		Amount:    100,
		Model:     "test-model",
		Condition: `or(is-vip(3), not(github-star("zgsm")))`,
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

	// Check execution results
	// 1. VIP user without star should be executed (satisfies both parts)
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userVipNoStar.ID).Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("VIP user without star expected execution 1 time, actually executed %d times", executeCount)}
	}

	// 2. VIP user with star should be executed (satisfies VIP part)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userVipWithStar.ID).Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("VIP user with star expected execution 1 time, actually executed %d times", executeCount)}
	}

	// 3. Low VIP user without star should be executed (satisfies NOT part)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userLowVipNoStar.ID).Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Low VIP user without star expected execution 1 time, actually executed %d times", executeCount)}
	}

	// 4. Low VIP user with star should not be executed (doesn't satisfy either part)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userLowVipWithStar.ID).Count(&executeCount)
	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Low VIP user with star expected execution 0 times, actually executed %d times", executeCount)}
	}

	// 5. No VIP user without star should be executed (satisfies NOT part)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userNoVipNoStar.ID).Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("No VIP user without star expected execution 1 time, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "OR + NOT nesting condition test succeeded"}
}
