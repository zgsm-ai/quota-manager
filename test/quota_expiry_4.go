package main

import (
	"fmt"
	"time"
)

import (
	"quota-manager/internal/models"
)

// 3.1 Idempotency Test
func testExpireQuotasTaskIdempotency(ctx *TestContext) TestResult {
	startTime := time.Now()

	// Clean up previous test data
	cleanupMockQuotaStore(ctx)

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
		err := executeExpireQuotasTask(ctx)
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