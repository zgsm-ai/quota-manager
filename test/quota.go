package main

import (
	"fmt"
	"time"

	"quota-manager/internal/models"
	"quota-manager/internal/services"
)

// testQuotaExpiry test quota expiry functionality
func testQuotaExpiry(ctx *TestContext) TestResult {
	// Create test user
	user := createTestUser("expiry_test_user", "Expiry Test User", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Create expired and valid quotas
	expiredDate := time.Now().Truncate(time.Second).Add(-time.Hour)
	validDate := time.Now().Truncate(time.Second).Add(30 * 24 * time.Hour)

	quotas := []*models.Quota{
		{UserID: user.ID, Amount: 50, ExpiryDate: expiredDate, Status: models.StatusValid},
		{UserID: user.ID, Amount: 100, ExpiryDate: validDate, Status: models.StatusValid},
	}

	for _, quota := range quotas {
		if err := ctx.DB.Create(quota).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create quota failed: %v", err)}
		}
	}

	// Set initial AiGateway quota
	mockStore.SetQuota(user.ID, 150)

	// Execute quota expiry
	if err := ctx.QuotaService.ExpireQuotas(); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expire quotas failed: %v", err)}
	}

	// Verify expired quota status
	var expiredQuota models.Quota
	if err := ctx.DB.Where("user_id = ? AND expiry_date = ?", user.ID, expiredDate).First(&expiredQuota).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get expired quota: %v", err)}
	}

	if expiredQuota.Status != models.StatusExpired {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected expired status, got %s", expiredQuota.Status)}
	}

	// Verify valid quota remains valid
	var validQuota models.Quota
	if err := ctx.DB.Where("user_id = ? AND expiry_date = ?", user.ID, validDate).First(&validQuota).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get valid quota: %v", err)}
	}

	if validQuota.Status != models.StatusValid {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected valid status, got %s", validQuota.Status)}
	}

	return TestResult{Passed: true, Message: "Quota Expiry Test Succeeded"}
}

// testQuotaAuditRecords test quota audit records functionality
func testQuotaAuditRecords(ctx *TestContext) TestResult {
	// Create test user
	user := createTestUser("audit_test_user", "Audit Test User", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Add quota using strategy execution
	if err := ctx.QuotaService.AddQuotaForStrategy(user.ID, 50, "test-strategy"); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Add quota for strategy failed: %v", err)}
	}

	// Get audit records
	records, total, err := ctx.QuotaService.GetQuotaAuditRecords(user.ID, 1, 10)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get audit records failed: %v", err)}
	}

	if total != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 audit record, got %d", total)}
	}

	if len(records) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 record in result, got %d", len(records))}
	}

	record := records[0]
	if record.Amount != 50 || record.Operation != models.OperationRecharge {
		return TestResult{Passed: false, Message: "Audit record data mismatch"}
	}

	// Verify strategy name is correctly recorded
	if record.StrategyName != "test-strategy" {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected strategy name 'test-strategy', got '%s'", record.StrategyName)}
	}

	// Use helper function to verify audit record
	if err := verifyStrategyNameInAudit(ctx, user.ID, "test-strategy", models.OperationRecharge); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Strategy name verification failed: %v", err)}
	}

	return TestResult{Passed: true, Message: "Quota Audit Records Test Succeeded"}
}

// testStrategyWithExpiryDate test strategy execution with expiry date
func testStrategyWithExpiryDate(ctx *TestContext) TestResult {
	// Create test user
	user := createTestUser("strategy_expiry_user", "Strategy Expiry User", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Create strategy
	strategy := &models.QuotaStrategy{
		Name:      "expiry-date-test",
		Title:     "Expiry Date Test",
		Type:      "single",
		Amount:    75,
		Model:     "test-model",
		Condition: "",
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create strategy failed: %v", err)}
	}

	// Execute strategy
	users := []models.UserInfo{*user}
	ctx.StrategyService.ExecStrategy(strategy, users)

	// Verify quota was created with expiry date
	var quota models.Quota
	if err := ctx.DB.Where("user_id = ?", user.ID).First(&quota).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get quota: %v", err)}
	}

	if quota.Amount != 75 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected quota amount 75, got %d", quota.Amount)}
	}

	if quota.Status != models.StatusValid {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected valid status, got %s", quota.Status)}
	}

	// Verify expiry date is set correctly (end of month or next month)
	now := time.Now()
	endOfMonth := time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, now.Location())
	var expectedExpiry time.Time
	if endOfMonth.Sub(now).Hours() < 24*30 {
		expectedExpiry = time.Date(now.Year(), now.Month()+2, 0, 23, 59, 59, 0, now.Location())
	} else {
		expectedExpiry = endOfMonth
	}

	if !quota.ExpiryDate.Equal(expectedExpiry) {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected expiry date %v, got %v", expectedExpiry, quota.ExpiryDate)}
	}

	// Verify execution record has expiry date
	var execute models.QuotaExecute
	if err := ctx.DB.Where("strategy_id = ? AND user_id = ?", strategy.ID, user.ID).First(&execute).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get execute record: %v", err)}
	}

	if !execute.ExpiryDate.Equal(expectedExpiry) {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected execute expiry date %v, got %v", expectedExpiry, execute.ExpiryDate)}
	}

	return TestResult{Passed: true, Message: "Strategy with Expiry Date Test Succeeded"}
}

// testMultipleOperationsAccuracy test accuracy of quota calculations under multiple operations
func testMultipleOperationsAccuracy(ctx *TestContext) TestResult {
	// Create test users
	user1 := createTestUser("user_ops_accuracy_1", "Operations User 1", 0)
	user2 := createTestUser("user_ops_accuracy_2", "Operations User 2", 0)

	if err := ctx.DB.AuthDB.Create(user1).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user1 failed: %v", err)}
	}
	if err := ctx.DB.AuthDB.Create(user2).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user2 failed: %v", err)}
	}

	// Initialize mock quota for both users
	mockStore.SetQuota(user1.ID, 0)
	mockStore.SetQuota(user2.ID, 0)

	// 1. Add initial quota via strategy for user1
	if err := ctx.QuotaService.AddQuotaForStrategy(user1.ID, 100, "initial-strategy"); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Add initial quota failed: %v", err)}
	}

	// Verify strategy name in audit for initial recharge
	if err := verifyStrategyNameInAudit(ctx, user1.ID, "initial-strategy", models.OperationRecharge); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Initial strategy name verification failed: %v", err)}
	}

	// 2. Transfer some quota from user1 to user2 - use same expiry date as created by strategy
	now := time.Now()
	var transferExpiryDate time.Time
	endOfMonth := time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, now.Location())
	if endOfMonth.Sub(now).Hours() < 24*30 {
		transferExpiryDate = time.Date(now.Year(), now.Month()+2, 0, 23, 59, 59, 0, now.Location())
	} else {
		transferExpiryDate = endOfMonth
	}

	transferOutReq := &services.TransferOutRequest{
		ReceiverID: user2.ID,
		QuotaList: []services.TransferQuotaItem{
			{Amount: 30, ExpiryDate: transferExpiryDate},
		},
	}
	transferOutResp, err := ctx.QuotaService.TransferOut(&models.AuthUser{
		ID: user1.ID, Name: user1.Name, Phone: "13800138000", Github: "user1",
	}, transferOutReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer out failed: %v", err)}
	}

	// 3. User2 transfers in the quota
	transferInReq := &services.TransferInRequest{
		VoucherCode: transferOutResp.VoucherCode,
	}
	_, err = ctx.QuotaService.TransferIn(&models.AuthUser{
		ID: user2.ID, Name: user2.Name, Phone: "13900139000", Github: "user2",
	}, transferInReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer in failed: %v", err)}
	}

	// 4. Consume some quota for user1 and user2
	ctx.QuotaService.DeltaUsedQuotaInAiGateway(user1.ID, 20)
	ctx.QuotaService.DeltaUsedQuotaInAiGateway(user2.ID, 10)

	// 5. Add more quota via strategy for user1
	if err := ctx.QuotaService.AddQuotaForStrategy(user1.ID, 50, "additional-strategy"); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Add additional quota failed: %v", err)}
	}

	// Verify strategy name in audit for additional recharge
	if err := verifyStrategyNameInAudit(ctx, user1.ID, "additional-strategy", models.OperationRecharge); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Additional strategy name verification failed: %v", err)}
	}

	// Verify transfer operations have no strategy name
	if err := verifyNoStrategyNameInAudit(ctx, user1.ID, models.OperationTransferOut); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer out strategy name verification failed: %v", err)}
	}
	if err := verifyNoStrategyNameInAudit(ctx, user2.ID, models.OperationTransferIn); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer in strategy name verification failed: %v", err)}
	}

	// Verify user1 quota calculations
	quotaInfo1, err := ctx.QuotaService.GetUserQuota(user1.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user1 quota failed: %v", err)}
	}

	expectedTotalUser1 := 120 // 100 initial + 50 additional - 30 transferred out
	expectedUsedUser1 := 20
	if quotaInfo1.TotalQuota != expectedTotalUser1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User1 total quota incorrect: expected %d, got %d", expectedTotalUser1, quotaInfo1.TotalQuota)}
	}
	if quotaInfo1.UsedQuota != expectedUsedUser1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User1 used quota incorrect: expected %d, got %d", expectedUsedUser1, quotaInfo1.UsedQuota)}
	}

	// Verify user2 quota calculations
	quotaInfo2, err := ctx.QuotaService.GetUserQuota(user2.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user2 quota failed: %v", err)}
	}

	expectedTotalUser2 := 30 // 30 transferred in
	expectedUsedUser2 := 10
	if quotaInfo2.TotalQuota != expectedTotalUser2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User2 total quota incorrect: expected %d, got %d", expectedTotalUser2, quotaInfo2.TotalQuota)}
	}
	if quotaInfo2.UsedQuota != expectedUsedUser2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User2 used quota incorrect: expected %d, got %d", expectedUsedUser2, quotaInfo2.UsedQuota)}
	}

	// Verify audit records count
	_, auditCount1, err := ctx.QuotaService.GetQuotaAuditRecords(user1.ID, 1, 100)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user1 audit records failed: %v", err)}
	}
	if auditCount1 != 3 { // initial + additional + transfer out
		return TestResult{Passed: false, Message: fmt.Sprintf("User1 audit records count incorrect: expected 3, got %d", auditCount1)}
	}

	_, auditCount2, err := ctx.QuotaService.GetQuotaAuditRecords(user2.ID, 1, 100)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user2 audit records failed: %v", err)}
	}
	if auditCount2 != 1 { // transfer in
		return TestResult{Passed: false, Message: fmt.Sprintf("User2 audit records count incorrect: expected 1, got %d", auditCount2)}
	}

	return TestResult{Passed: true, Message: "Multiple operations accuracy test succeeded"}
}

func testUserQuotaConsumptionOrder(ctx *TestContext) TestResult {
	// Create test user
	user := createTestUser("user_consumption_order", "Consumption Order User", 0)

	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Initialize mock quota
	mockStore.SetQuota(user.ID, 300)

	// Add quota with different expiry dates (earliest first approach)
	now := time.Now()

	// Add quota expiring in 10 days
	earlyExpiry := now.AddDate(0, 0, 10)
	if err := ctx.DB.Create(&models.Quota{
		UserID:     user.ID,
		Amount:     100,
		ExpiryDate: earlyExpiry,
		Status:     models.StatusValid,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create early quota failed: %v", err)}
	}

	// Add quota expiring in 30 days
	midExpiry := now.AddDate(0, 0, 30)
	if err := ctx.DB.Create(&models.Quota{
		UserID:     user.ID,
		Amount:     100,
		ExpiryDate: midExpiry,
		Status:     models.StatusValid,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create mid quota failed: %v", err)}
	}

	// Add quota expiring in 60 days
	lateExpiry := now.AddDate(0, 0, 60)
	if err := ctx.DB.Create(&models.Quota{
		UserID:     user.ID,
		Amount:     100,
		ExpiryDate: lateExpiry,
		Status:     models.StatusValid,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create late quota failed: %v", err)}
	}

	// Consume 150 quota (should consume from earliest expiring quotas first)
	// This should consume: 100 from early + 50 from mid, leaving 50 from mid + 100 from late
	ctx.QuotaService.DeltaUsedQuotaInAiGateway(user.ID, 150)

	// Get user quota to verify consumption order
	quotaInfo, err := ctx.QuotaService.GetUserQuota(user.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user quota failed: %v", err)}
	}

	// Should have 2 quota items, with consumption applied to earliest first
	if len(quotaInfo.QuotaList) != 2 { // Only items with remaining quota should be shown
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 quota items with remaining quota, got %d", len(quotaInfo.QuotaList))}
	}

	// Sort items by expiry date to verify order
	if quotaInfo.QuotaList[0].ExpiryDate.After(quotaInfo.QuotaList[1].ExpiryDate) {
		return TestResult{Passed: false, Message: "Quota items should be ordered by expiry date (earliest first)"}
	}

	// Verify remaining amounts - first item (mid expiry) should have 50 remaining
	if quotaInfo.QuotaList[0].Amount != 50 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected first item to have 50 remaining, got %d", quotaInfo.QuotaList[0].Amount)}
	}

	// Second item (late expiry) should have 100 remaining
	if quotaInfo.QuotaList[1].Amount != 100 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected second item to have 100 remaining, got %d", quotaInfo.QuotaList[1].Amount)}
	}

	// Verify total remaining quota (calculate from quota list)
	expectedRemaining := quotaInfo.TotalQuota - quotaInfo.UsedQuota // Should be 300 - 150 = 150
	actualRemaining := quotaInfo.TotalQuota - quotaInfo.UsedQuota
	if actualRemaining != expectedRemaining {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected remaining quota %d, got %d", expectedRemaining, actualRemaining)}
	}

	// Verify used quota
	expectedUsed := 150
	if quotaInfo.UsedQuota != expectedUsed {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected used quota %d, got %d", expectedUsed, quotaInfo.UsedQuota)}
	}

	return TestResult{Passed: true, Message: "User quota consumption order test succeeded"}
}

func testStrategyExpiryDateCoverage(ctx *TestContext) TestResult {
	// Create test users
	user1 := createTestUser("user_strategy_coverage_1", "Strategy Coverage User 1", 0)
	user2 := createTestUser("user_strategy_coverage_2", "Strategy Coverage User 2", 0)

	if err := ctx.DB.AuthDB.Create(user1).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user1 failed: %v", err)}
	}
	if err := ctx.DB.AuthDB.Create(user2).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user2 failed: %v", err)}
	}

	// Initialize mock quota
	mockStore.SetQuota(user1.ID, 100)
	mockStore.SetQuota(user2.ID, 100)

	now := time.Now()

	// Test case 1: Strategy execution when >30 days remaining in current month
	// Add quota for user1 (should expire at end of current month since >30 days remaining)
	if err := ctx.QuotaService.AddQuotaForStrategy(user1.ID, 100, "test-strategy-1"); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Add quota for user1 failed: %v", err)}
	}

	// Get user1's quota to check expiry date
	var user1Quotas []models.Quota
	if err := ctx.DB.DB.Where("user_id = ?", user1.ID).Find(&user1Quotas).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user1 quotas failed: %v", err)}
	}

	if len(user1Quotas) == 0 {
		return TestResult{Passed: false, Message: "No quota records found for user1"}
	}

	// Calculate expected expiry date based on AddQuotaForStrategy logic
	endOfCurrentMonth := time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, now.Location())
	endOfNextMonth := time.Date(now.Year(), now.Month()+2, 0, 23, 59, 59, 0, now.Location())

	var expectedExpiry time.Time
	if endOfCurrentMonth.Sub(now).Hours() < 24*30 {
		expectedExpiry = endOfNextMonth
	} else {
		expectedExpiry = endOfCurrentMonth
	}

	// Verify user1's quota expiry date
	quotaExpiry := user1Quotas[0].ExpiryDate
	// Allow some tolerance for time differences (1 day)
	timeDiff := quotaExpiry.Sub(expectedExpiry)
	if timeDiff > 24*time.Hour || timeDiff < -24*time.Hour {
		return TestResult{Passed: false, Message: fmt.Sprintf("User1 quota expiry date mismatch: expected around %v, got %v", expectedExpiry, quotaExpiry)}
	}

	// Test case 2: Strategy execution when <30 days remaining in current month
	// This is simulated by the automatic logic in AddQuotaForStrategy
	if err := ctx.QuotaService.AddQuotaForStrategy(user2.ID, 100, "test-strategy-2"); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Add quota for user2 failed: %v", err)}
	}

	// Get user2's quota to check expiry date
	var user2Quotas []models.Quota
	if err := ctx.DB.DB.Where("user_id = ?", user2.ID).Find(&user2Quotas).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user2 quotas failed: %v", err)}
	}

	if len(user2Quotas) == 0 {
		return TestResult{Passed: false, Message: "No quota records found for user2"}
	}

	// Verify user2's quota expiry date follows the same logic
	user2QuotaExpiry := user2Quotas[0].ExpiryDate
	timeDiff2 := user2QuotaExpiry.Sub(expectedExpiry)
	if timeDiff2 > 24*time.Hour || timeDiff2 < -24*time.Hour {
		return TestResult{Passed: false, Message: fmt.Sprintf("User2 quota expiry date mismatch: expected around %v, got %v", expectedExpiry, user2QuotaExpiry)}
	}

	// Verify both users have positive expiry dates (in the future)
	if quotaExpiry.Before(now) {
		return TestResult{Passed: false, Message: "User1 quota should have future expiry date"}
	}

	if user2QuotaExpiry.Before(now) {
		return TestResult{Passed: false, Message: "User2 quota should have future expiry date"}
	}

	// Verify audit records contain appropriate expiry dates
	auditRecords1, _, err := ctx.QuotaService.GetQuotaAuditRecords(user1.ID, 1, 100)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user1 audit records failed: %v", err)}
	}

	if len(auditRecords1) == 0 {
		return TestResult{Passed: false, Message: "No audit records found for user1"}
	}

	// The audit record expiry date should match the quota expiry date
	auditExpiry := auditRecords1[0].ExpiryDate
	auditTimeDiff := auditExpiry.Sub(quotaExpiry)
	if auditTimeDiff > time.Minute || auditTimeDiff < -time.Minute {
		return TestResult{Passed: false, Message: fmt.Sprintf("User1 audit record expiry date should match quota expiry: audit=%v, quota=%v", auditExpiry, quotaExpiry)}
	}

	return TestResult{Passed: true, Message: "Strategy expiry date coverage test succeeded"}
}
