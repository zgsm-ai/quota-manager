package main

import (
	"fmt"
	"time"

	"quota-manager/internal/models"
)

// Single User Quota Expiry Test
func testExpireQuotasTaskBasic(ctx *TestContext) TestResult {
	startTime := time.Now()

	// Clean up previous test data
	cleanupMockQuotaStore(ctx)

	// Create test user
	user := createTestUser("test_user_basic", "Test User Basic", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Create expired quota record (status: VALID, expiry time: last month end 23:59:59, amount: 100.0)
	quota, err := createExpiredTestQuota(ctx, user.ID, 100.0)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create expired quota failed: %v", err)}
	}

	// Set AiGateway Mock initial state: total quota 100.0, used quota 30.0
	ctx.MockQuotaStore.SetQuota(user.ID, 100.0)
	ctx.MockQuotaStore.SetUsed(user.ID, 30.0)

	// Execute expireQuotasTask function
	if err := executeExpireQuotasTask(ctx); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Execute expireQuotasTask failed: %v", err)}
	}

	// Comprehensive verification conditions
	expectedData := QuotaExpiryExpectation{
		ValidQuotaCount:          0,
		ExpiredQuotaCount:        1,
		ValidQuotaAmount:         0.0,
		ExpiredQuotaAmount:       100.0,
		MockQuotaStoreTotalQuota: 0.0,
		MockQuotaStoreUsedQuota:  0.0,
		ExpectedDeltaCalls:       []MockQuotaStoreDeltaCall{{EmployeeNumber: user.ID, Delta: -100.0}},
		ExpectedUsedDeltaCalls:   []MockQuotaStoreUsedDeltaCall{{EmployeeNumber: user.ID, Delta: -30.0}},
		ExpectedAuditAmount:      -100.0,
		AllowedAuditOperations:   []string{"EXPIRE"},
		ExpectedQuotaRecords: []QuotaRecordExpectation{
			{Amount: 100.0, ExpiryDate: quota.ExpiryDate, Status: models.StatusExpired},
		},
	}

	if err := verifyQuotaExpiryDataConsistency(ctx, user.ID, expectedData); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Data consistency verification failed: %v", err)}
	}

	// Verify quota status changed from VALID to EXPIRED
	if err := verifyQuotaStatus(ctx, fmt.Sprintf("%d", quota.ID), models.StatusExpired); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Quota status verification failed: %v", err)}
	}

	duration := time.Since(startTime)
	return TestResult{
		Passed:    true,
		Message:   "Single User Quota Expiry Test Succeeded",
		Duration:  duration,
		TestName:  "testExpireQuotasTaskBasic",
		StartTime: startTime,
		EndTime:   time.Now(),
	}
}

// Multiple Users Quota Expiry Test
func testExpireQuotasTaskMultiple(ctx *TestContext) TestResult {
	startTime := time.Now()

	// Clean up previous test data
	cleanupMockQuotaStore(ctx)

	// Create multiple test users
	user1 := createTestUser("test_user_multiple_1", "Test User Multiple 1", 0)
	user2 := createTestUser("test_user_multiple_2", "Test User Multiple 2", 0)
	user3 := createTestUser("test_user_multiple_3", "Test User Multiple 3", 0)

	// Batch create users
	if err := ctx.DB.AuthDB.Create(&[]*models.UserInfo{user1, user2, user3}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create users failed: %v", err)}
	}

	// Create expired quota records for multiple users
	quota1, err := createExpiredTestQuota(ctx, user1.ID, 100.0)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create expired quota for user1 failed: %v", err)}
	}

	quota2, err := createExpiredTestQuota(ctx, user2.ID, 150.0)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create expired quota for user2 failed: %v", err)}
	}

	quota3, err := createExpiredTestQuota(ctx, user3.ID, 200.0)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create expired quota for user3 failed: %v", err)}
	}

	// Set AiGateway Mock initial state
	ctx.MockQuotaStore.SetQuota(user1.ID, 100.0)
	ctx.MockQuotaStore.SetUsed(user1.ID, 30.0)
	ctx.MockQuotaStore.SetQuota(user2.ID, 150.0)
	ctx.MockQuotaStore.SetUsed(user2.ID, 50.0)
	ctx.MockQuotaStore.SetQuota(user3.ID, 200.0)
	ctx.MockQuotaStore.SetUsed(user3.ID, 80.0)

	// Execute expireQuotasTask function
	if err := executeExpireQuotasTask(ctx); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Execute expireQuotasTask failed: %v", err)}
	}

	// Verify all quota statuses are updated to EXPIRED
	for _, userID := range []string{user1.ID, user2.ID, user3.ID} {
		if err := verifyUserValidQuotaCount(ctx, userID, 0); err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("User %s valid quota count verification failed: %v", userID, err)}
		}
		if err := verifyUserExpiredQuotaCount(ctx, userID, 1); err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("User %s expired quota count verification failed: %v", userID, err)}
		}
	}

	// Verify each user's valid quota is 0
	for _, userID := range []string{user1.ID, user2.ID, user3.ID} {
		if err := verifyUserQuotaAmountByStatus(ctx, userID, models.StatusValid, 0.0); err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("User %s valid quota amount verification failed: %v", userID, err)}
		}
	}

	// Verify AiGateway sync is correct
	expectedDeltaCalls := []MockQuotaStoreDeltaCall{
		{EmployeeNumber: user1.ID, Delta: -100.0},
		{EmployeeNumber: user2.ID, Delta: -150.0},
		{EmployeeNumber: user3.ID, Delta: -200.0},
	}
	expectedUsedDeltaCalls := []MockQuotaStoreUsedDeltaCall{
		{EmployeeNumber: user1.ID, Delta: -30.0},
		{EmployeeNumber: user2.ID, Delta: -50.0},
		{EmployeeNumber: user3.ID, Delta: -80.0},
	}

	if err := verifyMockQuotaStoreDeltaCalls(ctx, expectedDeltaCalls); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("MockQuotaStore delta calls verification failed: %v", err)}
	}

	if err := verifyMockQuotaStoreUsedDeltaCalls(ctx, expectedUsedDeltaCalls); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("MockQuotaStore used delta calls verification failed: %v", err)}
	}

	// Verify quota status
	if err := verifyQuotaStatus(ctx, fmt.Sprintf("%d", quota1.ID), models.StatusExpired); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Quota1 status verification failed: %v", err)}
	}
	if err := verifyQuotaStatus(ctx, fmt.Sprintf("%d", quota2.ID), models.StatusExpired); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Quota2 status verification failed: %v", err)}
	}
	if err := verifyQuotaStatus(ctx, fmt.Sprintf("%d", quota3.ID), models.StatusExpired); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Quota3 status verification failed: %v", err)}
	}

	// Verify audit records for each quota are correctly generated
	for _, userID := range []string{user1.ID, user2.ID, user3.ID} {
		var expectedAmount float64
		switch userID {
		case user1.ID:
			expectedAmount = -100.0
		case user2.ID:
			expectedAmount = -150.0
		case user3.ID:
			expectedAmount = -200.0
		}

		if err := verifyQuotaExpiryAuditExists(ctx, userID, expectedAmount); err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("User %s audit record verification failed: %v", userID, err)}
		}
	}

	duration := time.Since(startTime)
	return TestResult{
		Passed:    true,
		Message:   "Multiple Users Quota Expiry Test Succeeded",
		Duration:  duration,
		TestName:  "testExpireQuotasTaskMultiple",
		StartTime: startTime,
		EndTime:   time.Now(),
	}
}

// No Expired Quota Test
func testExpireQuotasTaskEmpty(ctx *TestContext) TestResult {
	startTime := time.Now()

	// Clean up previous test data
	cleanupMockQuotaStore(ctx)

	// Create test user
	user := createTestUser("test_user_empty", "Test User Empty", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Create non-expired quota (expires in one month)
	quota, err := createValidTestQuota(ctx, user.ID, 100.0)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create valid quota failed: %v", err)}
	}

	// Set AiGateway Mock initial state: total quota 100.0, used quota 30.0
	ctx.MockQuotaStore.SetQuota(user.ID, 100.0)
	ctx.MockQuotaStore.SetUsed(user.ID, 30.0)

	// Execute expireQuotasTask function
	if err := executeExpireQuotasTask(ctx); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Execute expireQuotasTask failed: %v", err)}
	}

	// Verify no quota status was modified
	if err := verifyQuotaStatus(ctx, fmt.Sprintf("%d", quota.ID), models.StatusValid); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Quota status verification failed: %v", err)}
	}

	// Verify user's valid quota remains unchanged
	if err := verifyUserValidQuotaCount(ctx, user.ID, 1); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Valid quota count verification failed: %v", err)}
	}

	if err := verifyUserExpiredQuotaCount(ctx, user.ID, 0); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expired quota count verification failed: %v", err)}
	}

	if err := verifyUserQuotaAmountByStatus(ctx, user.ID, models.StatusValid, 100.0); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Valid quota amount verification failed: %v", err)}
	}

	// Verify AiGateway data remains unchanged
	if err := verifyMockQuotaStoreTotalQuota(ctx, user.ID, 100.0); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("MockQuotaStore total quota verification failed: %v", err)}
	}

	if err := verifyMockQuotaStoreUsedQuota(ctx, user.ID, 30.0); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("MockQuotaStore used quota verification failed: %v", err)}
	}

	// Verify no MockQuotaStore calls
	if err := verifyMockQuotaStoreDeltaCalls(ctx, []MockQuotaStoreDeltaCall{}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("MockQuotaStore delta calls verification failed: %v", err)}
	}

	if err := verifyMockQuotaStoreUsedDeltaCalls(ctx, []MockQuotaStoreUsedDeltaCall{}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("MockQuotaStore used delta calls verification failed: %v", err)}
	}

	// Verify no quota status change audit records were generated
	if err := verifyNoUnexpectedAuditRecords(ctx, user.ID, []string{}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Unexpected audit records verification failed: %v", err)}
	}

	// Verify quota record integrity
	expectedRecords := []QuotaRecordExpectation{
		{Amount: 100.0, ExpiryDate: quota.ExpiryDate, Status: models.StatusValid},
	}

	if err := verifyUserQuotaRecordsIntegrity(ctx, user.ID, expectedRecords); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Quota records integrity verification failed: %v", err)}
	}

	duration := time.Since(startTime)
	return TestResult{
		Passed:    true,
		Message:   "No Expired Quota Test Succeeded",
		Duration:  duration,
		TestName:  "testExpireQuotasTaskEmpty",
		StartTime: startTime,
		EndTime:   time.Now(),
	}
}

// Quota Expiry Test - Just Expired 1 Minute Ago
func testExpireQuotasTaskJustExpired(ctx *TestContext) TestResult {
	startTime := time.Now()

	// Clean up previous test data
	cleanupMockQuotaStore(ctx)

	// Create test user
	user := createTestUser("test_user_just_expired", "Test User Just Expired", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Create quota record that expired just 1 minute ago (status: VALID, expiry time: current time - 1 minute, amount: 100.0)
	expiryTime := time.Now().Add(-1 * time.Minute)
	quota, err := createTestQuotaWithExpiry(ctx, user.ID, 100.0, expiryTime)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create just expired quota failed: %v", err)}
	}

	// Set AiGateway Mock initial state: total quota 100.0, used quota 30.0
	ctx.MockQuotaStore.SetQuota(user.ID, 100.0)
	ctx.MockQuotaStore.SetUsed(user.ID, 30.0)

	// Execute expireQuotasTask function
	if err := executeExpireQuotasTask(ctx); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Execute expireQuotasTask failed: %v", err)}
	}

	// Comprehensive verification conditions
	expectedData := QuotaExpiryExpectation{
		ValidQuotaCount:          0,
		ExpiredQuotaCount:        1,
		ValidQuotaAmount:         0.0,
		ExpiredQuotaAmount:       100.0,
		MockQuotaStoreTotalQuota: 0.0,
		MockQuotaStoreUsedQuota:  0.0,
		ExpectedDeltaCalls:       []MockQuotaStoreDeltaCall{{EmployeeNumber: user.ID, Delta: -100.0}},
		ExpectedUsedDeltaCalls:   []MockQuotaStoreUsedDeltaCall{{EmployeeNumber: user.ID, Delta: -30.0}},
		ExpectedAuditAmount:      -100.0,
		AllowedAuditOperations:   []string{"EXPIRE"},
		ExpectedQuotaRecords: []QuotaRecordExpectation{
			{Amount: 100.0, ExpiryDate: quota.ExpiryDate, Status: models.StatusExpired},
		},
	}

	if err := verifyQuotaExpiryDataConsistency(ctx, user.ID, expectedData); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Data consistency verification failed: %v", err)}
	}

	// Verify quota status changed from VALID to EXPIRED
	if err := verifyQuotaStatus(ctx, fmt.Sprintf("%d", quota.ID), models.StatusExpired); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Quota status verification failed: %v", err)}
	}

	duration := time.Since(startTime)
	return TestResult{
		Passed:    true,
		Message:   "Just Expired Quota (1 Minute Ago) Test Succeeded",
		Duration:  duration,
		TestName:  "testExpireQuotasTaskJustExpired",
		StartTime: startTime,
		EndTime:   time.Now(),
	}
}

// AiGateway Sync Failure Test
func testExpireQuotasTaskAiGatewayFail(ctx *TestContext) TestResult {
	startTime := time.Now()

	// Clean up previous test data
	cleanupMockQuotaStore(ctx)

	// Create test user
	user := createTestUser("test_user_aigateway_fail", "Test User AiGateway Fail", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Create expired quota record (amount 120.0, expiry time: last month end 23:59:59, status VALID)
	quota, err := createExpiredTestQuota(ctx, user.ID, 120.0)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create expired quota failed: %v", err)}
	}

	// Set MockQuotaStore initial state: total quota 120.0, used quota 40.0
	ctx.MockQuotaStore.SetQuota(user.ID, 120.0)
	ctx.MockQuotaStore.SetUsed(user.ID, 40.0)

	// Configure AiGateway client to use failure server (simulate network failure or network internal error)
	restoreFunc := ctx.UseFailServer()
	defer restoreFunc()

	// Execute expireQuotasTask function
	if err := executeExpireQuotasTask(ctx); err == nil {
		return TestResult{Passed: false, Message: "Expected expireQuotasTask to fail, but it succeeded"}
	}

	// Comprehensive verification: verify transaction rollback correctly, all data remains unchanged

	// Verify quota status was not updated (still VALID)
	if err := verifyQuotaStatus(ctx, fmt.Sprintf("%d", quota.ID), models.StatusValid); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Quota status verification failed: %v", err)}
	}

	// Verify user's valid quota count remains 1
	if err := verifyUserValidQuotaCount(ctx, user.ID, 1); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Valid quota count verification failed: %v", err)}
	}

	// Verify user's expired quota count remains 0
	if err := verifyUserExpiredQuotaCount(ctx, user.ID, 0); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expired quota count verification failed: %v", err)}
	}

	// Verify user's valid quota amount remains 120.0
	if err := verifyUserQuotaAmountByStatus(ctx, user.ID, models.StatusValid, 120.0); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Valid quota amount verification failed: %v", err)}
	}

	// Verify user's expired quota amount remains 0.0
	if err := verifyUserQuotaAmountByStatus(ctx, user.ID, models.StatusExpired, 0.0); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expired quota amount verification failed: %v", err)}
	}

	// Verify MockQuotaStore data remains unchanged
	if err := verifyMockQuotaStoreTotalQuota(ctx, user.ID, 120.0); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("MockQuotaStore total quota verification failed: %v", err)}
	}

	if err := verifyMockQuotaStoreUsedQuota(ctx, user.ID, 40.0); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("MockQuotaStore used quota verification failed: %v", err)}
	}

	// Verify no MockQuotaStore delta calls (due to transaction rollback)
	if err := verifyMockQuotaStoreDeltaCalls(ctx, []MockQuotaStoreDeltaCall{}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("MockQuotaStore delta calls verification failed: %v", err)}
	}

	// Verify no MockQuotaStore used delta calls (due to transaction rollback)
	if err := verifyMockQuotaStoreUsedDeltaCalls(ctx, []MockQuotaStoreUsedDeltaCall{}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("MockQuotaStore used delta calls verification failed: %v", err)}
	}

	// Verify no quota status change audit records were generated due to transaction rollback
	if err := verifyNoUnexpectedAuditRecords(ctx, user.ID, []string{}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Unexpected audit records verification failed: %v", err)}
	}

	// Verify quota record integrity: 1 valid record, amount 120.0, status VALID
	expectedRecords := []QuotaRecordExpectation{
		{Amount: 120.0, ExpiryDate: quota.ExpiryDate, Status: models.StatusValid},
	}

	if err := verifyUserQuotaRecordsIntegrity(ctx, user.ID, expectedRecords); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Quota records integrity verification failed: %v", err)}
	}

	duration := time.Since(startTime)
	return TestResult{
		Passed:    true,
		Message:   "MockQuotaStore Sync Failure Test Succeeded",
		Duration:  duration,
		TestName:  "testExpireQuotasTaskAiGatewayFail",
		StartTime: startTime,
		EndTime:   time.Now(),
	}
}
