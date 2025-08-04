package main

import (
	"fmt"
	"time"

	"quota-manager/internal/models"
)

// 2.1 AiGateway Sync Failure Test
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

// 2.2 Partial Failure Handling Test
func testExpireQuotasTaskPartialFail(ctx *TestContext) TestResult {
	startTime := time.Now()

	// Clean up previous test data
	cleanupMockQuotaStore(ctx)

	// Create multiple test users
	user1 := createTestUser("test_user_partial_fail_1", "Test User Partial Fail 1", 0)
	user2 := createTestUser("test_user_partial_fail_2", "Test User Partial Fail 2", 0)
	user3 := createTestUser("test_user_partial_fail_3", "Test User Partial Fail 3", 0)

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

	// Set MockQuotaStore initial state
	ctx.MockQuotaStore.SetQuota(user1.ID, 100.0)
	ctx.MockQuotaStore.SetUsed(user1.ID, 30.0)
	ctx.MockQuotaStore.SetQuota(user2.ID, 150.0)
	ctx.MockQuotaStore.SetUsed(user2.ID, 50.0)
	ctx.MockQuotaStore.SetQuota(user3.ID, 200.0)
	ctx.MockQuotaStore.SetUsed(user3.ID, 80.0)

	// Configure MockQuotaStore responses to fail for certain users (e.g., user2 returns error)
	setupMockQuotaStorePartialFail(ctx, []string{user2.ID})

	// Execute expireQuotasTask function
	if err := executeExpireQuotasTask(ctx); err == nil {
		return TestResult{Passed: false, Message: "Expected expireQuotasTask to fail due to partial failure, but it succeeded"}
	}

	// Comprehensive verification: verify transaction rollback mechanism, all quota statuses remain unchanged (still VALID)

	// Verify all users' valid quota counts remain unchanged
	for i, user := range []*models.UserInfo{user1, user2, user3} {
		if err := verifyUserValidQuotaCount(ctx, user.ID, 1); err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("User %d valid quota count verification failed: %v", i+1, err)}
		}
	}

	// Verify all users' expired quota counts remain unchanged
	for i, user := range []*models.UserInfo{user1, user2, user3} {
		if err := verifyUserExpiredQuotaCount(ctx, user.ID, 0); err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("User %d expired quota count verification failed: %v", i+1, err)}
		}
	}

	// Verify all users' valid quota amounts remain unchanged
	expectedAmounts := map[string]float64{
		user1.ID: 100.0,
		user2.ID: 150.0,
		user3.ID: 200.0,
	}

	for userID, expectedAmount := range expectedAmounts {
		if err := verifyUserQuotaAmountByStatus(ctx, userID, models.StatusValid, expectedAmount); err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("User %s valid quota amount verification failed: %v", userID, err)}
		}
	}

	// Verify all users' expired quota amounts remain unchanged
	for _, user := range []*models.UserInfo{user1, user2, user3} {
		if err := verifyUserQuotaAmountByStatus(ctx, user.ID, models.StatusExpired, 0.0); err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("User %s expired quota amount verification failed: %v", user.ID, err)}
		}
	}

	// Verify MockQuotaStore data remains unchanged
	mockQuotaStoreData := map[string]struct {
		totalQuota float64
		usedQuota  float64
	}{
		user1.ID: {100.0, 30.0},
		user2.ID: {150.0, 50.0},
		user3.ID: {200.0, 80.0},
	}

	for userID, data := range mockQuotaStoreData {
		if err := verifyMockQuotaStoreTotalQuota(ctx, userID, data.totalQuota); err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("User %s MockQuotaStore total quota verification failed: %v", userID, err)}
		}
		if err := verifyMockQuotaStoreUsedQuota(ctx, userID, data.usedQuota); err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("User %s MockQuotaStore used quota verification failed: %v", userID, err)}
		}
	}

	// Verify no MockQuotaStore delta calls (due to transaction rollback caused by partial failure)
	if err := verifyMockQuotaStoreDeltaCalls(ctx, []MockQuotaStoreDeltaCall{}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("MockQuotaStore delta calls verification failed: %v", err)}
	}

	// Verify no MockQuotaStore used delta calls (due to transaction rollback caused by partial failure)
	if err := verifyMockQuotaStoreUsedDeltaCalls(ctx, []MockQuotaStoreUsedDeltaCall{}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("MockQuotaStore used delta calls verification failed: %v", err)}
	}

	// Verify no quota status change audit records were generated due to transaction rollback
	for _, user := range []*models.UserInfo{user1, user2, user3} {
		if err := verifyNoUnexpectedAuditRecords(ctx, user.ID, []string{}); err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("User %s unexpected audit records verification failed: %v", user.ID, err)}
		}
	}

	// Verify quota record integrity: each user has 1 valid record with amounts 100.0, 150.0, and 200.0 respectively, status VALID
	expectedRecordsList := map[string][]QuotaRecordExpectation{
		user1.ID: {
			{Amount: 100.0, ExpiryDate: quota1.ExpiryDate, Status: models.StatusValid},
		},
		user2.ID: {
			{Amount: 150.0, ExpiryDate: quota2.ExpiryDate, Status: models.StatusValid},
		},
		user3.ID: {
			{Amount: 200.0, ExpiryDate: quota3.ExpiryDate, Status: models.StatusValid},
		},
	}

	for userID, expectedRecords := range expectedRecordsList {
		if err := verifyUserQuotaRecordsIntegrity(ctx, userID, expectedRecords); err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("User %s quota records integrity verification failed: %v", userID, err)}
		}
	}

	duration := time.Since(startTime)
	return TestResult{
		Passed:    true,
		Message:   "Partial Failure Handling Test Succeeded",
		Duration:  duration,
		TestName:  "testExpireQuotasTaskPartialFail",
		StartTime: startTime,
		EndTime:   time.Now(),
	}
}
