package main

import (
	"fmt"
	"quota-manager/internal/models"
	"time"
)

// Mixed Status Quotas Test
func testExpireQuotasTask_MixedStatusQuotas(ctx *TestContext) TestResult {
	startTime := time.Now()

	// Clean up previous test data to ensure test isolation
	result := testClearData(ctx)
	if !result.Passed {
		return TestResult{Passed: false, Message: fmt.Sprintf("Clear test data failed: %s", result.Message)}
	}

	// Clean up previous test data
	cleanupMockQuotaStore(ctx)

	// Create test user
	user := createTestUser("test_user_mixed_status", "Test User Mixed Status Quotas", 0)

	// Get last month end time
	lastMonthEnd := getLastMonthEndTime()

	// Get current time
	now := time.Now()

	// Create quotas with different statuses
	// Quota 1: Already expired quota
	_, err := createTestQuota(ctx, user.ID, 100, "VALID", lastMonthEnd)
	if err != nil {
		return TestResult{
			Passed:   false,
			Message:  fmt.Sprintf("Failed to create quota 1: %v", err),
			Duration: time.Since(startTime),
			TestName: "Mixed Status Quotas Test",
		}
	}

	// Quota 2: Not expired quota (valid until next month)
	nextMonthEnd := time.Date(now.Year(), now.Month()+2, 0, 23, 59, 59, 0, time.Local)
	_, err = createTestQuota(ctx, user.ID, 200, "VALID", nextMonthEnd)
	if err != nil {
		return TestResult{
			Passed:   false,
			Message:  fmt.Sprintf("Failed to create quota 2: %v", err),
			Duration: time.Since(startTime),
			TestName: "Mixed Status Quotas Test",
		}
	}

	// Quota 3: Expired but manually marked as processed (use slightly different expiry time to avoid unique constraint conflict)
	lastMonthEnd2 := lastMonthEnd.Add(-1 * time.Hour)
	_, err = createTestQuota(ctx, user.ID, 150, "EXPIRED", lastMonthEnd2)
	if err != nil {
		return TestResult{
			Passed:   false,
			Message:  fmt.Sprintf("Failed to create quota 3: %v", err),
			Duration: time.Since(startTime),
			TestName: "Mixed Status Quotas Test",
		}
	}

	// Quota 4: Expired but fully consumed quota (use slightly different expiry time to avoid unique constraint conflict)
	lastMonthEnd3 := lastMonthEnd.Add(-3 * time.Hour)
	_, err = createTestQuota(ctx, user.ID, 80, "EXPIRED", lastMonthEnd3)
	if err != nil {
		return TestResult{
			Passed:   false,
			Message:  fmt.Sprintf("Failed to create quota 4: %v", err),
			Duration: time.Since(startTime),
			TestName: "Mixed Status Quotas Test",
		}
	}

	// Quota 5: About to expire quota (expires 1 hour ago to ensure it gets expired)
	oneHourAgo := now.Add(-1 * time.Hour)
	_, err = createTestQuota(ctx, user.ID, 120, "VALID", oneHourAgo)
	if err != nil {
		return TestResult{
			Passed:   false,
			Message:  fmt.Sprintf("Failed to create quota 5: %v", err),
			Duration: time.Since(startTime),
			TestName: "Mixed Status Quotas Test",
		}
	}

	if err := executeExpireQuotasTask(ctx); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Execute expireQuotasTask for user %s failed: %v", user.ID, err)}
	}

	// Verify quota status in database
	var expiredCount int64
	if err := ctx.DB.Model(&models.Quota{}).Where("user_id = ? AND status = ?", user.ID, "EXPIRED").Count(&expiredCount).Error; err != nil {
		return TestResult{
			Passed:   false,
			Message:  fmt.Sprintf("Failed to query expired quota count: %v", err),
			Duration: time.Since(startTime),
			TestName: "Mixed Status Quotas Test",
		}
	}

	// Should have 4 expired quotas: quota1 (originally expired), quota3 (processed but remains expired), quota4 (expired), quota5 (newly expired)
	// quota2 should not be expired
	if expiredCount != 4 {
		return TestResult{
			Passed:   false,
			Message:  fmt.Sprintf("Expected 4 quotas to expire, got %d", expiredCount),
			Duration: time.Since(startTime),
			TestName: "Mixed Status Quotas Test",
		}
	}

	// Verify specific status of each quota
	var quotas []models.Quota
	if err := ctx.DB.Where("user_id = ?", user.ID).Find(&quotas).Error; err != nil {
		return TestResult{
			Passed:   false,
			Message:  fmt.Sprintf("Failed to query user quotas: %v", err),
			Duration: time.Since(startTime),
			TestName: "Mixed Status Quotas Test",
		}
	}

	// Verify specific status of each quota
	// Quota 2 (ID=2) should remain VALID as it expires in the future
	// All other quotas should be EXPIRED
	for _, quota := range quotas {
		if quota.ID == 2 {
			// This is quota 2 - should remain VALID (expires in future)
			if quota.Status != "VALID" {
				return TestResult{
					Passed:   false,
					Message:  fmt.Sprintf("Quota ID %d should remain VALID but is %s", quota.ID, quota.Status),
					Duration: time.Since(startTime),
					TestName: "Mixed Status Quotas Test",
				}
			}
		} else {
			// All other quotas should be EXPIRED
			if quota.Status != "EXPIRED" {
				return TestResult{
					Passed:   false,
					Message:  fmt.Sprintf("Quota ID %d should be expired but is %s", quota.ID, quota.Status),
					Duration: time.Since(startTime),
					TestName: "Mixed Status Quotas Test",
				}
			}
		}
	}

	return TestResult{
		Passed:   true,
		Message:  "Mixed Status Quotas Test Succeeded",
		Duration: time.Since(startTime),
		TestName: "Mixed Status Quotas Test",
	}
}
