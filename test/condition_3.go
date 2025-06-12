package main

import (
	"fmt"
	"quota-manager/internal/models"
	"time"
)

// testThreeLevelNestingCondition tests three-level nested conditions (AND + OR + NOT)
func testThreeLevelNestingCondition(ctx *TestContext) TestResult {
	// Create test users using createTestUser function
	userVipOrgNoStar := createTestUser("user_three_vip_org_no_star", "VIP Org No Star User", 3)
	userVipOrgNoStar.Company = "org001"
	userVipOrgNoStar.GithubStar = "microsoft/vscode,google/tensorflow"

	userVipOrgWithStar := createTestUser("user_three_vip_org_star", "VIP Org Star User", 3)
	userVipOrgWithStar.Company = "org001"
	userVipOrgWithStar.GithubStar = "zgsm,openai/gpt-4"

	userVipNoOrgNoStar := createTestUser("user_three_vip_no_org_no_star", "VIP No Org No Star User", 3)
	userVipNoOrgNoStar.Company = "org002"
	userVipNoOrgNoStar.GithubStar = "microsoft/vscode"

	userLowVipOrgNoStar := createTestUser("user_three_low_vip_org_no_star", "Low VIP Org No Star User", 1)
	userLowVipOrgNoStar.Company = "org001"
	userLowVipOrgNoStar.GithubStar = "microsoft/vscode"

	userNoVipOrgNoStar := createTestUser("user_three_no_vip_org_no_star", "No VIP Org No Star User", 0)
	userNoVipOrgNoStar.Company = "org001"
	userNoVipOrgNoStar.GithubStar = "microsoft/vscode"

	users := []*models.UserInfo{
		userVipOrgNoStar,
		userVipOrgWithStar,
		userVipNoOrgNoStar,
		userLowVipOrgNoStar,
		userNoVipOrgNoStar,
	}

	for _, user := range users {
		if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
		}
	}

	// Create strategy with three-level nested condition
	// Condition: and(is-vip(3), or(belong-to("org001"), not(github-star("zgsm"))))
	uniqueStrategyName := fmt.Sprintf("three-level-nesting-test-%d", time.Now().UnixNano())
	strategy := &models.QuotaStrategy{
		Name:      uniqueStrategyName,
		Title:     "Three-Level Nesting Test",
		Type:      "single",
		Amount:    100,
		Model:     "test-model",
		Condition: `and(is-vip(3), or(belong-to("org001"), not(github-star("zgsm"))))`,
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
	// 1. VIP user in org001 without star should be executed (satisfies VIP AND (org OR NOT star))
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userVipOrgNoStar.ID).Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("VIP user in org001 without star expected execution 1 time, actually executed %d times", executeCount)}
	}

	// 2. VIP user in org001 with star should be executed (satisfies VIP AND org)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userVipOrgWithStar.ID).Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("VIP user in org001 with star expected execution 1 time, actually executed %d times", executeCount)}
	}

	// 3. VIP user not in org001 without star should be executed (satisfies VIP AND NOT star)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userVipNoOrgNoStar.ID).Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("VIP user not in org001 without star expected execution 1 time, actually executed %d times", executeCount)}
	}

	// 4. Low VIP user in org001 without star should not be executed (doesn't satisfy VIP)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userLowVipOrgNoStar.ID).Count(&executeCount)
	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Low VIP user in org001 without star expected execution 0 times, actually executed %d times", executeCount)}
	}

	// 5. No VIP user in org001 without star should not be executed (doesn't satisfy VIP)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userNoVipOrgNoStar.ID).Count(&executeCount)
	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("No VIP user in org001 without star expected execution 0 times, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "Three-level nesting condition test succeeded"}
}

// testMultipleConditionsNestingCondition tests multiple conditions nesting (multiple AND/OR in parallel)
func testMultipleConditionsNestingCondition(ctx *TestContext) TestResult {
	// Create test users using createTestUser function
	userAllConditions := createTestUser("user_multi_all", "All Conditions User", 3)
	userAllConditions.Company = "org001"
	userAllConditions.GithubStar = "zgsm,openai/gpt-4"
	userAllConditions.EmployeeNumber = "EMP001"

	userVipAndOrg := createTestUser("user_multi_vip_org", "VIP Org User", 3)
	userVipAndOrg.Company = "org001"
	userVipAndOrg.GithubStar = "microsoft/vscode"
	userVipAndOrg.EmployeeNumber = "EMP002"

	userVipAndStar := createTestUser("user_multi_vip_star", "VIP Star User", 3)
	userVipAndStar.Company = "org002"
	userVipAndStar.GithubStar = "zgsm,google/tensorflow"
	userVipAndStar.EmployeeNumber = "EMP003"

	userOrgAndStar := createTestUser("user_multi_org_star", "Org Star User", 1)
	userOrgAndStar.Company = "org001"
	userOrgAndStar.GithubStar = "zgsm,microsoft/vscode"
	userOrgAndStar.EmployeeNumber = "EMP004"

	userOnlyVip := createTestUser("user_multi_only_vip", "Only VIP User", 3)
	userOnlyVip.Company = "org002"
	userOnlyVip.GithubStar = "microsoft/vscode"
	userOnlyVip.EmployeeNumber = "EMP005"

	userOnlyOrg := createTestUser("user_multi_only_org", "Only Org User", 1)
	userOnlyOrg.Company = "org001"
	userOnlyOrg.GithubStar = "microsoft/vscode"
	userOnlyOrg.EmployeeNumber = "EMP006"

	userOnlyStar := createTestUser("user_multi_only_star", "Only Star User", 1)
	userOnlyStar.Company = "org002"
	userOnlyStar.GithubStar = "zgsm,facebook/react"
	userOnlyStar.EmployeeNumber = "EMP007"

	userNoConditions := createTestUser("user_multi_none", "No Conditions User", 0)
	userNoConditions.Company = "org002"
	userNoConditions.GithubStar = "microsoft/vscode"
	userNoConditions.EmployeeNumber = "EMP008"

	users := []*models.UserInfo{
		userAllConditions,
		userVipAndOrg,
		userVipAndStar,
		userOrgAndStar,
		userOnlyVip,
		userOnlyOrg,
		userOnlyStar,
		userNoConditions,
	}

	for _, user := range users {
		if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
		}
	}

	// Create strategy with multiple conditions nesting
	// Condition: or(and(is-vip(3), or(belong-to("org001"), github-star("zgsm"))), and(belong-to("org001"), github-star("zgsm")))
	uniqueStrategyName := fmt.Sprintf("multiple-conditions-nesting-test-%d", time.Now().UnixNano())
	strategy := &models.QuotaStrategy{
		Name:      uniqueStrategyName,
		Title:     "Multiple Conditions Nesting Test",
		Type:      "single",
		Amount:    100,
		Model:     "test-model",
		Condition: `or(and(is-vip(3), or(belong-to("org001"), github-star("zgsm"))), and(belong-to("org001"), github-star("zgsm")))`,
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
	// 1. User with all conditions should be executed (satisfies all combinations)
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userAllConditions.ID).Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User with all conditions expected execution 1 time, actually executed %d times", executeCount)}
	}

	// 2. User with VIP and org should be executed (satisfies first AND)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userVipAndOrg.ID).Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User with VIP and org expected execution 1 time, actually executed %d times", executeCount)}
	}

	// 3. User with VIP and star should be executed (satisfies second AND)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userVipAndStar.ID).Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User with VIP and star expected execution 1 time, actually executed %d times", executeCount)}
	}

	// 4. User with org and star should be executed (satisfies third AND)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userOrgAndStar.ID).Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User with org and star expected execution 1 time, actually executed %d times", executeCount)}
	}

	// 5. User with only VIP should not be executed (doesn't satisfy any AND)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userOnlyVip.ID).Count(&executeCount)
	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User with only VIP expected execution 0 times, actually executed %d times", executeCount)}
	}

	// 6. User with only org should not be executed (doesn't satisfy any AND)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userOnlyOrg.ID).Count(&executeCount)
	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User with only org expected execution 0 times, actually executed %d times", executeCount)}
	}

	// 7. User with only star should not be executed (doesn't satisfy any AND)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userOnlyStar.ID).Count(&executeCount)
	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User with only star expected execution 0 times, actually executed %d times", executeCount)}
	}

	// 8. User with no conditions should not be executed
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userNoConditions.ID).Count(&executeCount)
	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User with no conditions expected execution 0 times, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "Multiple conditions nesting test succeeded"}
}
