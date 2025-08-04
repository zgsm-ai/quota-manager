package main

import (
	"fmt"
	"quota-manager/internal/models"
	"time"
)

// Idempotency Test
func testExpireQuotasTaskIdempotency(ctx *TestContext) TestResult {
	startTime := time.Now()

	// Clean up previous test data to ensure test isolation
	result := testClearData(ctx)
	if !result.Passed {
		return TestResult{Passed: false, Message: fmt.Sprintf("Clear test data failed: %s", result.Message)}
	}

	// Create test user
	user := createTestUser("test_user_idempotency", "Test User Idempotency", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Create expired quota record (amount 200.0, expiry time is last day of previous month at 23:59:59, status VALID)
	quota, err := createExpiredTestQuota(ctx, user.ID, 200.0)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create expired quota failed: %v", err)}
	}

	// Set MockQuotaStore initial state: total quota 200.0, used quota 80.0
	ctx.MockQuotaStore.SetQuota(user.ID, 200.0)
	ctx.MockQuotaStore.SetUsed(user.ID, 80.0)

	// Execute expireQuotasTask function multiple times (e.g., execute 3 times consecutively)
	executionCount := 3

	for i := 0; i < executionCount; i++ {
		// Log used delta calls before execution
		beforeCalls := ctx.MockQuotaStore.GetUsedDeltaCalls()
		fmt.Printf("[DEBUG] Idempotency Test: Execution %d - Before executeExpireQuotasTask, used delta calls count: %d\n", i+1, len(beforeCalls))
		for j, call := range beforeCalls {
			fmt.Printf("[DEBUG] Idempotency Test: Execution %d - Before call %d: EmployeeNumber=%s, Delta=%f\n", i+1, j, call.EmployeeNumber, call.Delta)
		}

		err := executeExpireQuotasTask(ctx)

		// Log used delta calls after execution
		afterCalls := ctx.MockQuotaStore.GetUsedDeltaCalls()
		fmt.Printf("[DEBUG] Idempotency Test: Execution %d - After executeExpireQuotasTask, used delta calls count: %d\n", i+1, len(afterCalls))
		for j, call := range afterCalls {
			fmt.Printf("[DEBUG] Idempotency Test: Execution %d - After call %d: EmployeeNumber=%s, Delta=%f\n", i+1, j, call.EmployeeNumber, call.Delta)
		}

		if i == 0 {
			// First execution should succeed
			if err != nil {
				return TestResult{Passed: false, Message: fmt.Sprintf("First execution of expireQuotasTask failed: %v", err)}
			}
		} else {
			// Subsequent executions should also succeed (idempotency)
			if err != nil {
				return TestResult{Passed: false, Message: fmt.Sprintf("Execution %d of expireQuotasTask failed: %v", i+1, err)}
			}
		}
	}

	// Comprehensive verification: verify idempotency of repeated executions, ensuring multiple executions yield consistent results with single execution and no side effects

	// Verify quota status is updated only once (from VALID to EXPIRED)
	if err := verifyQuotaStatus(ctx, fmt.Sprintf("%d", quota.ID), models.StatusExpired); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Quota status verification failed: %v", err)}
	}

	// Verify user's valid quota count is 0
	if err := verifyUserValidQuotaCount(ctx, user.ID, 0); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Valid quota count verification failed: %v", err)}
	}

	// Verify user's expired quota count is 1
	if err := verifyUserExpiredQuotaCount(ctx, user.ID, 1); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expired quota count verification failed: %v", err)}
	}

	// Verify user's valid quota amount is 0.0
	if err := verifyUserQuotaAmountByStatus(ctx, user.ID, models.StatusValid, 0.0); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Valid quota amount verification failed: %v", err)}
	}

	// Verify user's expired quota amount is 200.0
	if err := verifyUserQuotaAmountByStatus(ctx, user.ID, models.StatusExpired, 200.0); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expired quota amount verification failed: %v", err)}
	}

	// Verify MockQuotaStore total quota is synchronized to 0.0 (all quotas expired)
	if err := verifyMockQuotaStoreTotalQuota(ctx, user.ID, 0.0); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("MockQuotaStore total quota verification failed: %v", err)}
	}

	// Verify MockQuotaStore used quota is synchronized to 0.0 (used quota is reset)
	if err := verifyMockQuotaStoreUsedQuota(ctx, user.ID, 0.0); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("MockQuotaStore used quota verification failed: %v", err)}
	}

	// Verify MockQuotaStore delta call records only once: total quota delta is -200.0
	expectedDeltaCalls := []MockQuotaStoreDeltaCall{
		{EmployeeNumber: user.ID, Delta: -200.0},
	}

	if err := verifyMockQuotaStoreDeltaCalls(ctx, expectedDeltaCalls); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("MockQuotaStore delta calls verification failed: %v", err)}
	}

	// Verify MockQuotaStore used delta call records only once: used quota delta is -80.0
	expectedUsedDeltaCalls := []MockQuotaStoreUsedDeltaCall{
		{EmployeeNumber: user.ID, Delta: -80.0},
	}

	if err := verifyMockQuotaStoreUsedDeltaCalls(ctx, expectedUsedDeltaCalls); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("MockQuotaStore used delta calls verification failed: %v", err)}
	}

	// Verify quota audit record is generated only once, amount is -200.0, operation type is EXPIRE
	if err := verifyQuotaExpiryAuditExists(ctx, user.ID, -200.0); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Audit record verification failed: %v", err)}
	}

	// Verify second and subsequent executions do not produce side effects:

	// Verify no additional MockQuotaStore calls
	actualDeltaCalls := ctx.MockQuotaStore.GetDeltaCalls()
	if len(actualDeltaCalls) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 MockQuotaStore delta call, got %d", len(actualDeltaCalls))}
	}

	actualUsedDeltaCalls := ctx.MockQuotaStore.GetUsedDeltaCalls()
	if len(actualUsedDeltaCalls) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 MockQuotaStore used delta call, got %d", len(actualUsedDeltaCalls))}
	}

	// Verify no additional audit records are generated
	var auditCount int64
	err = ctx.DB.Model(&models.QuotaAudit{}).
		Where("user_id = ? AND amount < ? AND operation = ?", user.ID, 0, "EXPIRE").
		Count(&auditCount).Error
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Count audit records failed: %v", err)}
	}

	if auditCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 audit record, got %d", auditCount)}
	}

	// Verify quota status is not repeatedly updated (still EXPIRED)
	if err := verifyQuotaStatus(ctx, fmt.Sprintf("%d", quota.ID), models.StatusExpired); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Quota status verification after multiple executions failed: %v", err)}
	}

	// Verify database data remains unchanged (no duplicate expired records)
	var totalQuotas int64
	err = ctx.DB.Model(&models.Quota{}).Where("user_id = ?", user.ID).Count(&totalQuotas).Error
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Count total quotas failed: %v", err)}
	}

	if totalQuotas != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 quota record, got %d", totalQuotas)}
	}

	// Verify quota record integrity: 1 expired record, amount 200.0, status EXPIRED
	expectedRecords := []QuotaRecordExpectation{
		{Amount: 200.0, ExpiryDate: quota.ExpiryDate, Status: models.StatusExpired},
	}

	if err := verifyUserQuotaRecordsIntegrity(ctx, user.ID, expectedRecords); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Quota records integrity verification failed: %v", err)}
	}

	// Verify time difference of multiple executions (subsequent executions should be faster as there are no actual data changes)
	// This can be verified by checking execution time, but main logic has already been verified above

	duration := time.Since(startTime)
	return TestResult{
		Passed:    true,
		Message:   fmt.Sprintf("Idempotency Test Succeeded (executed %d times)", executionCount),
		Duration:  duration,
		TestName:  "testExpireQuotasTaskIdempotency",
		StartTime: startTime,
		EndTime:   time.Now(),
	}
}

// 4.1 Month End Batch Expiry Test
func testExpireQuotasTask_MonthEndBatchExpiry(ctx *TestContext) TestResult {
	startTime := time.Now()

	// Clean up previous test data
	cleanupMockQuotaStore(ctx)

	// Create multiple users, all quotas set to expire on the same month end day
	user1 := createTestUser("test_user_monthend_batch_1", "Test User MonthEnd Batch 1", 0)
	user2 := createTestUser("test_user_monthend_batch_2", "Test User MonthEnd Batch 2", 0)
	user3 := createTestUser("test_user_monthend_batch_3", "Test User MonthEnd Batch 3", 0)

	// Batch create users
	if err := ctx.DB.AuthDB.Create(&[]*models.UserInfo{user1, user2, user3}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create users failed: %v", err)}
	}

	// Create expired quota records for multiple users (user1: amount 50.0, user2: amount 75.0, user3: amount 100.0, expiry time is last day of previous month at 23:59:59, status VALID)

	expiryTime := getLastMonthEndTime()

	quota1, err := createTestQuotaWithExpiry(ctx, user1.ID, 50.0, expiryTime)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create expired quota for user1 failed: %v", err)}
	}

	quota2, err := createTestQuotaWithExpiry(ctx, user2.ID, 75.0, expiryTime)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create expired quota for user2 failed: %v", err)}
	}

	quota3, err := createTestQuotaWithExpiry(ctx, user3.ID, 100.0, expiryTime)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create expired quota for user3 failed: %v", err)}
	}

	// Set MockQuotaStore initial state: user1 total quota 50.0 used 10.0, user2 total quota 75.0 used 25.0, user3 total quota 100.0 used 40.0
	ctx.MockQuotaStore.SetQuota(user1.ID, 50.0)
	ctx.MockQuotaStore.SetUsed(user1.ID, 10.0)
	ctx.MockQuotaStore.SetQuota(user2.ID, 75.0)
	ctx.MockQuotaStore.SetUsed(user2.ID, 25.0)
	ctx.MockQuotaStore.SetQuota(user3.ID, 100.0)
	ctx.MockQuotaStore.SetUsed(user3.ID, 40.0)

	// Set current time to first day of next month
	// Note: This may need to be adjusted based on actual time control mechanism

	// Execute expireQuotasTask function
	if err := executeExpireQuotasTask(ctx); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Execute expireQuotasTask failed: %v", err)}
	}

	// Verify batch processing performance (execution time within reasonable range)
	duration := time.Since(startTime)
	if duration > 5*time.Second {
		return TestResult{Passed: false, Message: fmt.Sprintf("Batch processing took too long: %v", duration)}
	}

	// Verify all quota statuses are correctly updated to EXPIRED
	if err := verifyQuotaStatus(ctx, fmt.Sprintf("%d", quota1.ID), models.StatusExpired); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Quota1 status verification failed: %v", err)}
	}
	if err := verifyQuotaStatus(ctx, fmt.Sprintf("%d", quota2.ID), models.StatusExpired); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Quota2 status verification failed: %v", err)}
	}
	if err := verifyQuotaStatus(ctx, fmt.Sprintf("%d", quota3.ID), models.StatusExpired); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Quota3 status verification failed: %v", err)}
	}

	// Verify each user's valid quota count is 0
	for i, user := range []*models.UserInfo{user1, user2, user3} {
		if err := verifyUserValidQuotaCount(ctx, user.ID, 0); err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("User %d valid quota count verification failed: %v", i+1, err)}
		}
	}

	// Verify each user's expired quota count is 1
	for i, user := range []*models.UserInfo{user1, user2, user3} {
		if err := verifyUserExpiredQuotaCount(ctx, user.ID, 1); err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("User %d expired quota count verification failed: %v", i+1, err)}
		}
	}

	// Verify each user's valid quota amount is 0.0
	for _, user := range []*models.UserInfo{user1, user2, user3} {
		if err := verifyUserQuotaAmountByStatus(ctx, user.ID, models.StatusValid, 0.0); err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("User %s valid quota amount verification failed: %v", user.ID, err)}
		}
	}

	// Verify each user's expired quota amount is the original quota amount (user1: 50.0, user2: 75.0, user3: 100.0)
	expectedAmounts := map[string]float64{
		user1.ID: 50.0,
		user2.ID: 75.0,
		user3.ID: 100.0,
	}

	for userID, expectedAmount := range expectedAmounts {
		if err := verifyUserQuotaAmountByStatus(ctx, userID, models.StatusExpired, expectedAmount); err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("User %s expired quota amount verification failed: %v", userID, err)}
		}
	}

	// Verify MockQuotaStore total quota is synchronized to 0.0 (all quotas expired)
	for _, user := range []*models.UserInfo{user1, user2, user3} {
		if err := verifyMockQuotaStoreTotalQuota(ctx, user.ID, 0.0); err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("User %s MockQuotaStore total quota verification failed: %v", user.ID, err)}
		}
	}

	// Verify MockQuotaStore used quota is synchronized to 0.0 (used quota is reset)
	for _, user := range []*models.UserInfo{user1, user2, user3} {
		if err := verifyMockQuotaStoreUsedQuota(ctx, user.ID, 0.0); err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("User %s MockQuotaStore used quota verification failed: %v", user.ID, err)}
		}
	}

	// Verify MockQuotaStore delta call records
	expectedDeltaCalls := []MockQuotaStoreDeltaCall{
		{EmployeeNumber: user1.ID, Delta: -50.0},
		{EmployeeNumber: user2.ID, Delta: -75.0},
		{EmployeeNumber: user3.ID, Delta: -100.0},
	}

	if err := verifyMockQuotaStoreDeltaCalls(ctx, expectedDeltaCalls); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("MockQuotaStore delta calls verification failed: %v", err)}
	}

	// Verify MockQuotaStore used delta call records
	expectedUsedDeltaCalls := []MockQuotaStoreUsedDeltaCall{
		{EmployeeNumber: user1.ID, Delta: -10.0},
		{EmployeeNumber: user2.ID, Delta: -25.0},
		{EmployeeNumber: user3.ID, Delta: -40.0},
	}

	if err := verifyMockQuotaStoreUsedDeltaCalls(ctx, expectedUsedDeltaCalls); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("MockQuotaStore used delta calls verification failed: %v", err)}
	}

	// Verify status change audit records for all expired quotas are correctly generated (status changed from VALID to EXPIRED)
	for _, user := range []*models.UserInfo{user1, user2, user3} {
		var expectedAmount float64
		switch user.ID {
		case user1.ID:
			expectedAmount = -50.0
		case user2.ID:
			expectedAmount = -75.0
		case user3.ID:
			expectedAmount = -100.0
		}

		if err := verifyQuotaExpiryAuditExists(ctx, user.ID, expectedAmount); err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("User %s audit record verification failed: %v", user.ID, err)}
		}
	}

	// Verify accuracy of audit record timestamps (record precise expiry time)
	// This can be implemented by querying audit records and verifying their timestamps

	// Verify quota record integrity: each user has 1 expired record with correct amount and status
	expectedRecordsList := map[string][]QuotaRecordExpectation{
		user1.ID: {
			{Amount: 50.0, ExpiryDate: quota1.ExpiryDate, Status: models.StatusExpired},
		},
		user2.ID: {
			{Amount: 75.0, ExpiryDate: quota2.ExpiryDate, Status: models.StatusExpired},
		},
		user3.ID: {
			{Amount: 100.0, ExpiryDate: quota3.ExpiryDate, Status: models.StatusExpired},
		},
	}

	for userID, expectedRecords := range expectedRecordsList {
		if err := verifyUserQuotaRecordsIntegrity(ctx, userID, expectedRecords); err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("User %s quota records integrity verification failed: %v", userID, err)}
		}
	}

	return TestResult{
		Passed:   true,
		Message:  "Month End Batch Expiry Test Succeeded",
		Duration: duration,
		TestName: "testExpireQuotasTask_MonthEndBatchExpiry",
	}
}

// Month Day Differences Test
func testExpireQuotasTask_MonthDayDifferences(ctx *TestContext) TestResult {
	startTime := time.Now()

	// Clean up previous test data
	cleanupMockQuotaStore(ctx)
	testResult1 := testMonthEndScenario(ctx, "test_user_feb_mar", "Test User Feb Mar",
		time.February, time.March, 60.0, 20.0)
	if !testResult1.Passed {
		return testResult1
	}

	// Test scenario 2: April to May (30 days)
	cleanupMockQuotaStore(ctx)
	testResult2 := testMonthEndScenario(ctx, "test_user_apr_may", "Test User Apr May",
		time.April, time.May, 60.0, 20.0)
	if !testResult2.Passed {
		return testResult2
	}

	// Test scenario 3: July to August (31 days)
	cleanupMockQuotaStore(ctx)
	testResult3 := testMonthEndScenario(ctx, "test_user_jul_aug", "Test User Jul Aug",
		time.July, time.August, 60.0, 20.0)
	if !testResult3.Passed {
		return testResult3
	}

	// Cross-scenario consistency verification
	// Verify expiry processing logic is consistent across all month scenarios
	// Verify differences in month days do not affect expiry judgment accuracy
	// Verify boundary time processing is correct under various month conditions

	duration := time.Since(startTime)
	return TestResult{
		Passed:   true,
		Message:  "Month Day Differences Test Succeeded",
		Duration: duration,
		TestName: "testExpireQuotasTask_MonthDayDifferences",
	}
}

// testMonthEndScenario helper function: test month end expiry scenario for specific month
func testMonthEndScenario(ctx *TestContext, userName, displayName string,
	startMonth, endMonth time.Month, quotaAmount, usedAmount float64) TestResult {

	user := createTestUser(userName, displayName, 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user %s failed: %v", userName, err)}
	}

	// Calculate last day of the specified month
	currentYear := time.Now().Year()
	expiryTime := getMonthEndTime(currentYear, startMonth)

	// Create quota with expiry time set to last day of specified month at 23:59:59
	quota, err := createTestQuotaWithExpiry(ctx, user.ID, quotaAmount, expiryTime)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create quota for user %s failed: %v", userName, err)}
	}

	// Set MockQuotaStore initial state: total quota 60.0, used quota 20.0
	ctx.MockQuotaStore.SetQuota(user.ID, quotaAmount)
	ctx.MockQuotaStore.SetUsed(user.ID, usedAmount)

	// Set current time to first day of next month
	// Note: This may need to be adjusted based on actual time control mechanism

	// Execute expireQuotasTask function
	if err := executeExpireQuotasTask(ctx); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Execute expireQuotasTask for user %s failed: %v", userName, err)}
	}

	// Verify correctness of month end expiry time calculation for corresponding month
	switch startMonth {
	case time.February:
		// February: last day is 28 in common years or 29 in leap years
		expectedDay := 28
		if isLeapYear(currentYear) {
			expectedDay = 29
		}
		if expiryTime.Day() != expectedDay {
			return TestResult{Passed: false, Message: fmt.Sprintf("February expiry day should be %d, got %d", expectedDay, expiryTime.Day())}
		}
	case time.April:
		// April: last day of 30 days
		if expiryTime.Day() != 30 {
			return TestResult{Passed: false, Message: fmt.Sprintf("April expiry day should be 30, got %d", expiryTime.Day())}
		}
	case time.July:
		// July: last day of 31 days
		if expiryTime.Day() != 31 {
			return TestResult{Passed: false, Message: fmt.Sprintf("July expiry day should be 31, got %d", expiryTime.Day())}
		}
	}

	// Verify quota status is correctly updated to EXPIRED
	if err := verifyQuotaStatus(ctx, fmt.Sprintf("%d", quota.ID), models.StatusExpired); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Quota status verification for user %s failed: %v", userName, err)}
	}

	// Verify user's valid quota count is 0
	if err := verifyUserValidQuotaCount(ctx, user.ID, 0); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Valid quota count verification for user %s failed: %v", userName, err)}
	}

	// Verify user's expired quota count is 1
	if err := verifyUserExpiredQuotaCount(ctx, user.ID, 1); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expired quota count verification for user %s failed: %v", userName, err)}
	}

	// Verify user's valid quota amount is 0.0
	if err := verifyUserQuotaAmountByStatus(ctx, user.ID, models.StatusValid, 0.0); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Valid quota amount verification for user %s failed: %v", userName, err)}
	}

	// Verify user's expired quota amount is 60.0
	if err := verifyUserQuotaAmountByStatus(ctx, user.ID, models.StatusExpired, quotaAmount); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expired quota amount verification for user %s failed: %v", userName, err)}
	}

	// Verify MockQuotaStore total quota is synchronized to 0.0 (all quotas expired)
	if err := verifyMockQuotaStoreTotalQuota(ctx, user.ID, 0.0); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("MockQuotaStore total quota verification for user %s failed: %v", userName, err)}
	}

	// Verify MockQuotaStore used quota is synchronized to 0.0 (used quota is reset)
	if err := verifyMockQuotaStoreUsedQuota(ctx, user.ID, 0.0); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("MockQuotaStore used quota verification for user %s failed: %v", userName, err)}
	}

	// Verify MockQuotaStore delta call records: total quota delta is -60.0
	expectedDeltaCalls := []MockQuotaStoreDeltaCall{
		{EmployeeNumber: user.ID, Delta: -quotaAmount},
	}

	if err := verifyMockQuotaStoreDeltaCalls(ctx, expectedDeltaCalls); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("MockQuotaStore delta calls verification for user %s failed: %v", userName, err)}
	}

	// Verify MockQuotaStore used delta call records: used quota delta is -20.0
	expectedUsedDeltaCalls := []MockQuotaStoreUsedDeltaCall{
		{EmployeeNumber: user.ID, Delta: -usedAmount},
	}

	if err := verifyMockQuotaStoreUsedDeltaCalls(ctx, expectedUsedDeltaCalls); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("MockQuotaStore used delta calls verification for user %s failed: %v", userName, err)}
	}

	// Verify quota status change audit record is correctly generated
	if err := verifyQuotaExpiryAuditExists(ctx, user.ID, -quotaAmount); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Audit record verification for user %s failed: %v", userName, err)}
	}

	// Verify audit record contains correct month time information
	// This can be implemented by querying audit records and verifying their timestamps

	// Verify quota record integrity: 1 expired record, amount 60.0, status EXPIRED
	expectedRecords := []QuotaRecordExpectation{
		{Amount: quotaAmount, ExpiryDate: quota.ExpiryDate, Status: models.StatusExpired},
	}

	if err := verifyUserQuotaRecordsIntegrity(ctx, user.ID, expectedRecords); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Quota records integrity verification for user %s failed: %v", userName, err)}
	}

	return TestResult{
		Passed:   true,
		Message:  fmt.Sprintf("%s month end scenario test succeeded", startMonth.String()),
		TestName: fmt.Sprintf("testMonthEndScenario_%s", startMonth.String()),
	}
}

// isLeapYear checks if a year is a leap year
func isLeapYear(year int) bool {
	if year%4 != 0 {
		return false
	} else if year%100 != 0 {
		return true
	} else {
		return year%400 == 0
	}
}
