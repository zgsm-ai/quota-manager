package main

import (
	"fmt"
	"quota-manager/internal/models"
	"time"
)

// testComplexNestedConditions1 tests a complex combination of nested conditions
// Testing: (match-user OR (register-before AND access-after)) AND (github-star OR quota-le)
func testComplexNestedConditions1(ctx *TestContext) TestResult {
	baseTime, _ := time.Parse("2006-01-02 15:04:05", "2025-01-01 12:00:00")

	// Create test users
	user1 := createTestUser("complex_user1", "Complex User 1", 0)
	user1.CreatedAt = baseTime.Add(-time.Hour * 48) // Registered before
	user1.AccessTime = baseTime.Add(time.Hour * 24) // Accessed after
	user1.GithubStar = "test-project"               // Has starred

	user2 := createTestUser("complex_user2", "Complex User 2", 0)
	user2.CreatedAt = baseTime.Add(-time.Hour * 48) // Registered before
	user2.AccessTime = baseTime.Add(time.Hour * 24) // Accessed after
	// Will set quota to 5 after user creation

	user3 := createTestUser("complex_user3", "Complex User 3", 0) // Should match by ID
	user3.CreatedAt = baseTime.Add(time.Hour * 48)                // Registered after
	user3.AccessTime = baseTime.Add(-time.Hour * 24)              // Accessed before

	user4 := createTestUser("complex_user4", "Complex User 4", 0)
	user4.CreatedAt = baseTime.Add(time.Hour * 48)   // Registered after
	user4.AccessTime = baseTime.Add(-time.Hour * 24) // Accessed before
	// Will set quota to 20 after user creation

	users := []*models.UserInfo{user1, user2, user3, user4}
	for _, user := range users {
		if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
		}
	}

	// Set quotas for users
	quotas := []*models.Quota{
		{
			UserID:     user2.ID,
			Amount:     5,
			ExpiryDate: time.Now().Add(24 * time.Hour),
			Status:     "VALID",
		},
		{
			UserID:     user4.ID,
			Amount:     20,
			ExpiryDate: time.Now().Add(24 * time.Hour),
			Status:     "VALID",
		},
	}

	for _, quota := range quotas {
		if err := ctx.DB.DB.Create(quota).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create quota failed: %v", err)}
		}
	}

	// Create complex nested condition strategy
	strategy := &models.QuotaStrategy{
		Name:   "complex-nested-test-1",
		Title:  "Complex Nested Test 1",
		Type:   "single",
		Amount: 100,
		Model:  "test-model",
		Condition: fmt.Sprintf(`and(or(match-user("%s"),and(register-before("%s"),access-after("%s"))),or(github-star("test-project"),quota-le("test-model",10)))`,
			user3.ID,
			baseTime.Format("2006-01-02 15:04:05"),
			baseTime.Format("2006-01-02 15:04:05")),
		Status: true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	// Execute strategy
	userList := []models.UserInfo{*user1, *user2, *user3, *user4}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// Check results
	var executeCount int64

	// user1 should be executed (matches register-before AND access-after AND github-star)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user1.ID).Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User1 expected execution 1 time, actually executed %d times", executeCount)}
	}

	// user2 should be executed (matches register-before AND access-after AND quota-le)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user2.ID).Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User2 expected execution 1 time, actually executed %d times", executeCount)}
	}

	// user3 should be executed (matches by ID)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user3.ID).Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User3 expected execution 1 time, actually executed %d times", executeCount)}
	}

	// user4 should not be executed (matches nothing)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user4.ID).Count(&executeCount)
	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User4 expected execution 0 times, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "Complex nested conditions test 1 succeeded"}
}

// testComplexNestedConditions2 tests another complex combination of nested conditions
// Testing: (is-vip AND belong-to) OR (not(quota-le) AND (register-before OR access-after))
func testComplexNestedConditions2(ctx *TestContext) TestResult {
	baseTime, _ := time.Parse("2006-01-02 15:04:05", "2025-01-01 12:00:00")

	// Create test users
	user1 := createTestUser("complex2_user1", "Complex2 User 1", 3) // VIP level 3
	user1.Company = "test-org"                                      // Belongs to org

	user2 := createTestUser("complex2_user2", "Complex2 User 2", 0)
	user2.CreatedAt = baseTime.Add(-time.Hour * 48) // Registered before
	// Will set high quota after user creation

	user3 := createTestUser("complex2_user3", "Complex2 User 3", 0)
	user3.AccessTime = baseTime.Add(time.Hour * 24) // Accessed after
	// Will set high quota after user creation

	user4 := createTestUser("complex2_user4", "Complex2 User 4", 0)
	user4.CreatedAt = baseTime.Add(time.Hour * 48)   // Registered after
	user4.AccessTime = baseTime.Add(-time.Hour * 24) // Accessed before
	// Will set low quota after user creation

	users := []*models.UserInfo{user1, user2, user3, user4}
	for _, user := range users {
		if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
		}
	}

	// Set quotas for users
	quotas := []*models.Quota{
		{
			UserID:     user2.ID,
			Amount:     15,
			ExpiryDate: time.Now().Add(24 * time.Hour),
			Status:     "VALID",
		},
		{
			UserID:     user3.ID,
			Amount:     15,
			ExpiryDate: time.Now().Add(24 * time.Hour),
			Status:     "VALID",
		},
		{
			UserID:     user4.ID,
			Amount:     5,
			ExpiryDate: time.Now().Add(24 * time.Hour),
			Status:     "VALID",
		},
	}

	for _, quota := range quotas {
		if err := ctx.DB.DB.Create(quota).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create quota failed: %v", err)}
		}
	}

	// Set mock quota values in AiGateway
	mockStore.SetQuota(user2.ID, 15)
	mockStore.SetQuota(user3.ID, 15)
	mockStore.SetQuota(user4.ID, 5)

	// Create complex nested condition strategy
	strategy := &models.QuotaStrategy{
		Name:   "complex-nested-test-2",
		Title:  "Complex Nested Test 2",
		Type:   "single",
		Amount: 100,
		Model:  "test-model",
		Condition: fmt.Sprintf(`or(and(is-vip(3),belong-to("test-org")),and(not(quota-le("test-model",10)),or(register-before("%s"),access-after("%s"))))`,
			baseTime.Format("2006-01-02 15:04:05"),
			baseTime.Format("2006-01-02 15:04:05")),
		Status: true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	// Execute strategy
	userList := []models.UserInfo{*user1, *user2, *user3, *user4}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// Check results
	var executeCount int64

	// user1 should be executed (matches VIP AND belong-to)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user1.ID).Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User1 expected execution 1 time, actually executed %d times", executeCount)}
	}

	// user2 should be executed (matches not(quota-le) AND register-before)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user2.ID).Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User2 expected execution 1 time, actually executed %d times", executeCount)}
	}

	// user3 should be executed (matches not(quota-le) AND access-after)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user3.ID).Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User3 expected execution 1 time, actually executed %d times", executeCount)}
	}

	// user4 should not be executed (matches nothing)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user4.ID).Count(&executeCount)
	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User4 expected execution 0 times, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "Complex nested conditions test 2 succeeded"}
}

// testComplexNestedConditions3 tests a third complex combination of nested conditions
// Testing: not(and(quota-le OR not(is-vip), not(or(github-star, belong-to))))
func testComplexNestedConditions3(ctx *TestContext) TestResult {
	// Create test users
	user1 := createTestUser("complex3_user1", "Complex3 User 1", 2) // VIP level 2
	user1.GithubStar = "test-project"                               // Has starred
	user1.Company = "test-org"                                      // Belongs to org
	// Will set high quota after user creation

	user2 := createTestUser("complex3_user2", "Complex3 User 2", 0)
	user2.GithubStar = "test-project" // Has starred
	// Will set low quota after user creation

	user3 := createTestUser("complex3_user3", "Complex3 User 3", 2) // VIP level 2
	// Will set low quota after user creation

	user4 := createTestUser("complex3_user4", "Complex3 User 4", 0)
	// Will set high quota after user creation

	users := []*models.UserInfo{user1, user2, user3, user4}
	for _, user := range users {
		if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
		}
	}

	// Set quotas for users
	quotas := []*models.Quota{
		{
			UserID:     user1.ID,
			Amount:     15,
			ExpiryDate: time.Now().Add(24 * time.Hour),
			Status:     "VALID",
		},
		{
			UserID:     user2.ID,
			Amount:     5,
			ExpiryDate: time.Now().Add(24 * time.Hour),
			Status:     "VALID",
		},
		{
			UserID:     user3.ID,
			Amount:     5,
			ExpiryDate: time.Now().Add(24 * time.Hour),
			Status:     "VALID",
		},
		{
			UserID:     user4.ID,
			Amount:     15,
			ExpiryDate: time.Now().Add(24 * time.Hour),
			Status:     "VALID",
		},
	}

	for _, quota := range quotas {
		if err := ctx.DB.DB.Create(quota).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create quota failed: %v", err)}
		}
	}

	// Create complex nested condition strategy
	strategy := &models.QuotaStrategy{
		Name:      "complex-nested-test-3",
		Title:     "Complex Nested Test 3",
		Type:      "single",
		Amount:    100,
		Model:     "test-model",
		Condition: `not(and(or(quota-le("test-model",10),not(is-vip(2))),not(or(github-star("test-project"),belong-to("test-org")))))`,
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	// Execute strategy
	userList := []models.UserInfo{*user1, *user2, *user3, *user4}
	ctx.StrategyService.ExecStrategy(strategy, userList)

	// Check results
	var executeCount int64

	// user1 should be executed (matches everything)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user1.ID).Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User1 expected execution 1 time, actually executed %d times", executeCount)}
	}

	// user2 should be executed (matches github-star)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user2.ID).Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User2 expected execution 1 time, actually executed %d times", executeCount)}
	}

	// user3 should not be executed (low quota and no github-star/org)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user3.ID).Count(&executeCount)
	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User3 expected execution 0 times, actually executed %d times", executeCount)}
	}

	// user4 should not be executed (not VIP and no github-star/org)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, user4.ID).Count(&executeCount)
	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User4 expected execution 0 times, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "Complex nested conditions test 3 succeeded"}
}
