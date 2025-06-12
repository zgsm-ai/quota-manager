package main

import (
	"fmt"
	"strings"
	"time"

	"quota-manager/internal/models"
	"quota-manager/internal/services"
)

// testTransferInStatusCases tests all different transfer in status cases
func testTransferInStatusCases(ctx *TestContext) TestResult {
	// Create test users
	user1 := createTestUser("user_status_1", "Status User 1", 0)
	user2 := createTestUser("user_status_2", "Status User 2", 0)

	if err := ctx.DB.AuthDB.Create(user1).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user1 failed: %v", err)}
	}
	if err := ctx.DB.AuthDB.Create(user2).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user2 failed: %v", err)}
	}

	// Initialize mock quota
	mockStore.SetQuota(user1.ID, 300)

	now := time.Now().Truncate(time.Second)

	// Test Case 1: Transfer In All Success
	// Add quota with valid expiry dates
	validExpiry1 := now.AddDate(0, 0, 30)
	validExpiry2 := now.AddDate(0, 0, 60)

	if err := ctx.DB.Create(&models.Quota{
		UserID:     user1.ID,
		Amount:     100,
		ExpiryDate: validExpiry1,
		Status:     models.StatusValid,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create quota1 failed: %v", err)}
	}

	if err := ctx.DB.Create(&models.Quota{
		UserID:     user1.ID,
		Amount:     200,
		ExpiryDate: validExpiry2,
		Status:     models.StatusValid,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create quota2 failed: %v", err)}
	}

	// Transfer out all valid quotas
	transferOutReq := &services.TransferOutRequest{
		ReceiverID: user2.ID,
		QuotaList: []services.TransferQuotaItem{
			{Amount: 80, ExpiryDate: validExpiry1},
			{Amount: 150, ExpiryDate: validExpiry2},
		},
	}

	transferOutResp, err := ctx.QuotaService.TransferOut(&models.AuthUser{
		ID: user1.ID, Name: user1.Name, Phone: "13800138000", Github: "user1",
	}, transferOutReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer out for all success test failed: %v", err)}
	}

	// Transfer in - should be all successful
	transferInReq := &services.TransferInRequest{
		VoucherCode: transferOutResp.VoucherCode,
	}

	transferInResp, err := ctx.QuotaService.TransferIn(&models.AuthUser{
		ID: user2.ID, Name: user2.Name, Phone: "13900139000", Github: "user2",
	}, transferInReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer in all success test failed: %v", err)}
	}

	if transferInResp.Status != services.TransferStatusSuccess {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected SUCCESS status, got %s", transferInResp.Status)}
	}

	if transferInResp.Amount != 230 { // 80 + 150
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 230 transferred amount, got %d", transferInResp.Amount)}
	}

	// Verify all quotas are marked as successful
	allSuccess := true
	for _, quotaResult := range transferInResp.QuotaList {
		if !quotaResult.Success {
			allSuccess = false
			break
		}
	}
	if !allSuccess {
		return TestResult{Passed: false, Message: "Not all quotas marked as successful in all success case"}
	}

	// Test Case 2: Transfer In with Expired Quota (Partial Success)
	// Create new users for this test
	user3 := createTestUser("user_status_3", "Status User 3", 0)
	user4 := createTestUser("user_status_4", "Status User 4", 0)

	if err := ctx.DB.AuthDB.Create(user3).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user3 failed: %v", err)}
	}
	if err := ctx.DB.AuthDB.Create(user4).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user4 failed: %v", err)}
	}

	mockStore.SetQuota(user3.ID, 200)

	// Add quota with mixed valid dates - we'll create a scenario where quota expires between transfer out and transfer in
	now2 := time.Now().Truncate(time.Second)
	shortValidDate := now2.Add(time.Second * 2) // Very short expiry - will expire quickly
	validDate := now2.AddDate(0, 0, 30)         // 30 days from now

	if err := ctx.DB.Create(&models.Quota{
		UserID:     user3.ID,
		Amount:     100,
		ExpiryDate: shortValidDate,
		Status:     models.StatusValid,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create short-term quota failed: %v", err)}
	}

	if err := ctx.DB.Create(&models.Quota{
		UserID:     user3.ID,
		Amount:     100,
		ExpiryDate: validDate,
		Status:     models.StatusValid,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create valid quota failed: %v", err)}
	}

	// Transfer out mixed quotas
	transferOutReq2 := &services.TransferOutRequest{
		ReceiverID: user4.ID,
		QuotaList: []services.TransferQuotaItem{
			{Amount: 80, ExpiryDate: shortValidDate}, // This will expire by transfer in time
			{Amount: 80, ExpiryDate: validDate},      // This will remain valid
		},
	}

	transferOutResp2, err := ctx.QuotaService.TransferOut(&models.AuthUser{
		ID: user3.ID, Name: user3.Name, Phone: "13800138001", Github: "user3",
	}, transferOutReq2)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer out for partial success test failed: %v", err)}
	}

	// Wait for short-term quota to expire
	time.Sleep(time.Second * 3)

	// Transfer in - should be partial success
	transferInReq2 := &services.TransferInRequest{
		VoucherCode: transferOutResp2.VoucherCode,
	}

	transferInResp2, err := ctx.QuotaService.TransferIn(&models.AuthUser{
		ID: user4.ID, Name: user4.Name, Phone: "13900139001", Github: "user4",
	}, transferInReq2)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer in partial success test failed: %v", err)}
	}

	if transferInResp2.Status != services.TransferStatusPartialSuccess {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected PARTIAL_SUCCESS status, got %s", transferInResp2.Status)}
	}

	if transferInResp2.Amount != 80 { // Only valid quota
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 80 transferred amount (excluding expired), got %d", transferInResp2.Amount)}
	}

	// Verify quota results contain failure reasons
	hasExpiredFailure := false
	hasSuccessQuota := false
	for _, quotaResult := range transferInResp2.QuotaList {
		if quotaResult.IsExpired && quotaResult.FailureReason != nil && *quotaResult.FailureReason == services.TransferFailureReasonExpired {
			hasExpiredFailure = true
		}
		if quotaResult.Success {
			hasSuccessQuota = true
		}
	}

	if !hasExpiredFailure {
		return TestResult{Passed: false, Message: "Expected expired quota to have EXPIRED failure reason"}
	}

	if !hasSuccessQuota {
		return TestResult{Passed: false, Message: "Expected at least one successful quota in partial success case"}
	}

	// Test Case 3: Transfer In Already Redeemed
	// Try to redeem the same voucher again
	transferInResp3, err := ctx.QuotaService.TransferIn(&models.AuthUser{
		ID: user4.ID, Name: user4.Name, Phone: "13900139001", Github: "user4",
	}, transferInReq2) // Same request as before
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer in already redeemed test failed: %v", err)}
	}

	if transferInResp3.Status != services.TransferStatusAlreadyRedeemed {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected ALREADY_REDEEMED status, got %s", transferInResp3.Status)}
	}

	// Test Case 4: Transfer In Invalid Voucher
	invalidTransferInReq := &services.TransferInRequest{
		VoucherCode: "invalid_voucher_code",
	}

	transferInResp4, err := ctx.QuotaService.TransferIn(&models.AuthUser{
		ID: user4.ID, Name: user4.Name, Phone: "13900139001", Github: "user4",
	}, invalidTransferInReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer in invalid voucher test failed: %v", err)}
	}

	if transferInResp4.Status != services.TransferStatusFailed {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected FAILED status for invalid voucher, got %s", transferInResp4.Status)}
	}

	if !strings.Contains(transferInResp4.Message, "Invalid voucher code") {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 'Invalid voucher code' message, got '%s'", transferInResp4.Message)}
	}

	return TestResult{Passed: true, Message: "All transfer in status cases tested successfully"}
}
