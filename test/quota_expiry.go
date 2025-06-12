package main

import (
	"fmt"
	"strings"
	"time"

	"quota-manager/internal/models"
	"quota-manager/internal/services"
)

// testTransferOutWithExpiredQuota tests the scenario when transferring out expired quota
func testTransferOutWithExpiredQuota(ctx *TestContext) TestResult {
	// Create test user
	giver := createTestUser("giver_expired", "Giver Expired", 0)
	if err := ctx.DB.AuthDB.Create(giver).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create giver user failed: %v", err)}
	}

	// Add expired quota
	expiredDate := time.Now().Truncate(time.Second).Add(-24 * time.Hour) // Expired 1 day ago
	expiredQuota := &models.Quota{
		UserID:     giver.ID,
		Amount:     100,
		ExpiryDate: expiredDate,
		Status:     models.StatusValid, // Initial status is valid, but time has expired
	}
	if err := ctx.DB.Create(expiredQuota).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create expired quota failed: %v", err)}
	}

	// Add valid quota
	validDate := time.Now().Truncate(time.Second).Add(30 * 24 * time.Hour)
	validQuota := &models.Quota{
		UserID:     giver.ID,
		Amount:     50,
		ExpiryDate: validDate,
		Status:     models.StatusValid,
	}
	if err := ctx.DB.Create(validQuota).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create valid quota failed: %v", err)}
	}

	// Set mock quota
	mockStore.SetQuota(giver.ID, 150)

	giverAuth := &models.AuthUser{
		ID: giver.ID, Name: giver.Name, Phone: "13800138000", Github: "giver_expired",
	}

	// First test: transfer out quota with expired date (should succeed before expiry processing, because status is still valid)
	transferReq := &services.TransferOutRequest{
		ReceiverID: "receiver_test",
		QuotaList: []services.TransferQuotaItem{
			{Amount: 50, ExpiryDate: expiredDate}, // Transfer out quota with expired date
		},
	}

	response, err := ctx.QuotaService.TransferOut(giverAuth, transferReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer out should succeed before expiry processing: %v", err)}
	}

	if response.VoucherCode == "" {
		return TestResult{Passed: false, Message: "Transfer should generate voucher code"}
	}

	// Verify that expired quota is reduced
	var updatedExpiredQuota models.Quota
	if err := ctx.DB.Where("user_id = ? AND expiry_date = ?", giver.ID, expiredDate).First(&updatedExpiredQuota).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get updated expired quota: %v", err)}
	}

	if updatedExpiredQuota.Amount != 50 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected remaining expired quota 50, got %d", updatedExpiredQuota.Amount)}
	}

	// Execute quota expiry processing
	if err := ctx.QuotaService.ExpireQuotas(); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expire quotas failed: %v", err)}
	}

	// Verify that expired quota status is updated
	if err := ctx.DB.Where("user_id = ? AND expiry_date = ?", giver.ID, expiredDate).First(&updatedExpiredQuota).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get expired quota: %v", err)}
	}
	if updatedExpiredQuota.Status != models.StatusExpired {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected expired status, got %s", updatedExpiredQuota.Status)}
	}

	// Now try to transfer out expired quota again(should fail, because the system only finds quotas with status=valid when searching for valid quota)
	transferReq2 := &services.TransferOutRequest{
		ReceiverID: "receiver_test2",
		QuotaList: []services.TransferQuotaItem{
			{Amount: 30, ExpiryDate: expiredDate}, // Try to transfer out remaining expired quota
		},
	}

	_, err = ctx.QuotaService.TransferOut(giverAuth, transferReq2)
	if err == nil {
		return TestResult{Passed: false, Message: "Transfer out expired quota should fail after expiry processing"}
	}

	// Verify error message
	if !strings.Contains(err.Error(), "quota not found") && !strings.Contains(err.Error(), "insufficient") {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected quota not found error for expired quota, got: %v", err)}
	}

	// Try to transfer out valid quota(should succeed)
	validTransferReq := &services.TransferOutRequest{
		ReceiverID: "receiver_test_valid",
		QuotaList: []services.TransferQuotaItem{
			{Amount: 30, ExpiryDate: validDate},
		},
	}

	validResponse, err := ctx.QuotaService.TransferOut(giverAuth, validTransferReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer out valid quota failed: %v", err)}
	}

	if validResponse.VoucherCode == "" {
		return TestResult{Passed: false, Message: "Valid transfer should generate voucher code"}
	}

	// Verify that valid quota is correctly reduced
	var updatedValidQuota models.Quota
	if err := ctx.DB.Where("user_id = ? AND expiry_date = ?", giver.ID, validDate).First(&updatedValidQuota).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get updated valid quota: %v", err)}
	}

	if updatedValidQuota.Amount != 20 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected remaining valid quota 20, got %d", updatedValidQuota.Amount)}
	}

	// Verify that user quota info only calculates valid quota
	quotaInfo, err := ctx.QuotaService.GetUserQuota(giver.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user quota failed: %v", err)}
	}

	expectedTotal := 20 // only valid quota minus the transferred part
	if quotaInfo.TotalQuota != expectedTotal {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected total quota %d, got %d", expectedTotal, quotaInfo.TotalQuota)}
	}

	// Verify audit records
	var auditRecords []models.QuotaAudit
	if err := ctx.DB.Where("user_id = ? AND operation = ?", giver.ID, models.OperationTransferOut).Find(&auditRecords).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get audit records: %v", err)}
	}

	if len(auditRecords) != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 transfer out audit records, got %d", len(auditRecords))}
	}

	// Verify the first transfer out (expired quota) audit record
	foundExpiredTransfer := false
	foundValidTransfer := false
	for _, record := range auditRecords {
		if record.Amount == -50 && record.ExpiryDate.Equal(expiredDate) {
			foundExpiredTransfer = true
		}
		if record.Amount == -30 && record.ExpiryDate.Equal(validDate) {
			foundValidTransfer = true
		}
	}

	if !foundExpiredTransfer {
		return TestResult{Passed: false, Message: "Expected audit record for expired quota transfer not found"}
	}

	if !foundValidTransfer {
		return TestResult{Passed: false, Message: "Expected audit record for valid quota transfer not found"}
	}

	return TestResult{Passed: true, Message: "Transfer Out With Expired Quota Test Succeeded"}
}

// testTransferInWithExpiredVoucher tests the scenario when transferring in with voucher containing expired quota
func testTransferInWithExpiredVoucher(ctx *TestContext) TestResult {
	// Create receiver user
	receiver := createTestUser("receiver_expired_voucher", "Receiver Expired Voucher", 0)
	if err := ctx.DB.AuthDB.Create(receiver).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create receiver user failed: %v", err)}
	}

	// Generate voucher containing expired quota
	expiredDate := time.Now().Truncate(time.Second).Add(-12 * time.Hour) // Expired 12 hours ago
	validDate := time.Now().Truncate(time.Second).Add(30 * 24 * time.Hour)

	voucherData := &services.VoucherData{
		GiverID:     "giver_expired_voucher",
		GiverName:   "Giver Expired Voucher",
		GiverPhone:  "13800138000",
		GiverGithub: "giver_expired",
		ReceiverID:  receiver.ID,
		QuotaList: []services.VoucherQuotaItem{
			{Amount: 40, ExpiryDate: expiredDate}, // expired quota
			{Amount: 60, ExpiryDate: validDate},   // valid quota
		},
	}

	voucherCode, err := ctx.VoucherService.GenerateVoucher(voucherData)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Generate voucher with expired quota failed: %v", err)}
	}

	// Try to transfer in
	receiverAuth := &models.AuthUser{
		ID: receiver.ID, Name: receiver.Name, Phone: "13900139000", Github: "receiver_expired",
	}

	transferReq := &services.TransferInRequest{
		VoucherCode: voucherCode,
	}

	response, err := ctx.QuotaService.TransferIn(receiverAuth, transferReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer in with expired voucher failed: %v", err)}
	}

	// Verify that only valid quota is transferred in
	var validQuota models.Quota
	if err := ctx.DB.Where("user_id = ? AND expiry_date = ?", receiver.ID, validDate).First(&validQuota).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get valid quota: %v", err)}
	}

	if validQuota.Amount != 60 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected valid quota 60, got %d", validQuota.Amount)}
	}

	// Verify that expired quota was not created
	var expiredQuota models.Quota
	err = ctx.DB.Where("user_id = ? AND expiry_date = ?", receiver.ID, expiredDate).First(&expiredQuota).Error
	if err == nil {
		return TestResult{Passed: false, Message: "Expired quota should not be created in transfer in"}
	}

	// Verify that audit records only record valid quota
	var auditRecord models.QuotaAudit
	if err := ctx.DB.Where("user_id = ? AND operation = ?", receiver.ID, models.OperationTransferIn).First(&auditRecord).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get audit record: %v", err)}
	}

	if auditRecord.Amount != 60 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected audit amount 60, got %d", auditRecord.Amount)}
	}

	// Verify transfer in status
	if response.Status != services.TransferStatusPartialSuccess {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected partial success status, got %s", response.Status)}
	}

	// Verify that message contains expiry information
	if !strings.Contains(response.Message, "expired") {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected message to contain 'expired', got: %s", response.Message)}
	}

	return TestResult{Passed: true, Message: "Transfer In With Expired Voucher Test Succeeded"}
}

// testQuotaExpiryDuringTransfer test the scenario when quota expires during transfer process
func testQuotaExpiryDuringTransfer(ctx *TestContext) TestResult {
	// Create test user
	giver := createTestUser("giver_expiry_during", "Giver Expiry During", 0)
	receiver := createTestUser("receiver_expiry_during", "Receiver Expiry During", 0)

	if err := ctx.DB.AuthDB.Create(giver).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create giver user failed: %v", err)}
	}
	if err := ctx.DB.AuthDB.Create(receiver).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create receiver user failed: %v", err)}
	}

	// Add already expired quota (expired 1 hour ago)
	soonExpireDate := time.Now().Truncate(time.Second).Add(-1 * time.Hour)
	quota := &models.Quota{
		UserID:     giver.ID,
		Amount:     80,
		ExpiryDate: soonExpireDate,
		Status:     models.StatusValid,
	}
	if err := ctx.DB.Create(quota).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create soon-expire quota failed: %v", err)}
	}

	// Set mock quota
	mockStore.SetQuota(giver.ID, 80)

	// Step 1: First create a valid quota and transfer out (generate voucher)
	validDate := time.Now().Truncate(time.Second).Add(30 * 24 * time.Hour)
	validQuota := &models.Quota{
		UserID:     giver.ID,
		Amount:     100,
		ExpiryDate: validDate,
		Status:     models.StatusValid,
	}
	if err := ctx.DB.Create(validQuota).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create valid quota failed: %v", err)}
	}

	// Set mock quota
	mockStore.SetQuota(giver.ID, 180) // 80(expired) + 100(valid)

	giverAuth := &models.AuthUser{
		ID: giver.ID, Name: giver.Name, Phone: "13800138000", Github: "giver_expiry",
	}

	// First transfer out valid quota, generate voucher
	transferOutReq := &services.TransferOutRequest{
		ReceiverID: receiver.ID,
		QuotaList: []services.TransferQuotaItem{
			{Amount: 50, ExpiryDate: validDate},
		},
	}

	_, err := ctx.QuotaService.TransferOut(giverAuth, transferOutReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer out failed: %v", err)}
	}

	// Now manually modify the expiry date in the voucher to expired time
	// We need to generate a new voucher containing expired quota
	voucherData := &services.VoucherData{
		GiverID:     giver.ID,
		GiverName:   giver.Name,
		GiverPhone:  "13800138000",
		GiverGithub: "giver_expiry",
		ReceiverID:  receiver.ID,
		QuotaList: []services.VoucherQuotaItem{
			{Amount: 50, ExpiryDate: soonExpireDate}, // expired quota
		},
	}

	expiredVoucherCode, err := ctx.VoucherService.GenerateVoucher(voucherData)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Generate expired voucher failed: %v", err)}
	}

	// Step 2: Try to transfer in the expired voucher
	receiverAuth := &models.AuthUser{
		ID: receiver.ID, Name: receiver.Name, Phone: "13900139000", Github: "receiver_expiry",
	}

	transferInReq := &services.TransferInRequest{
		VoucherCode: expiredVoucherCode,
	}

	transferInResp, err := ctx.QuotaService.TransferIn(receiverAuth, transferInReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer in failed: %v", err)}
	}

	// Verify that transfer in status should be failure, because all quota has expired
	if transferInResp.Status != services.TransferStatusFailed {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected failed status for expired quota, got %s", transferInResp.Status)}
	}

	// Verify that message contains expiry-related information
	if !strings.Contains(transferInResp.Message, "failed") {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected failure message, got: %s", transferInResp.Message)}
	}

	// Verify that receiver didn't get any quota
	var receiverQuota models.Quota
	err = ctx.DB.Where("user_id = ? AND expiry_date = ?", receiver.ID, soonExpireDate).First(&receiverQuota).Error
	if err == nil {
		return TestResult{Passed: false, Message: "Expired quota should not be added to receiver"}
	}

	// Verify that original transfer out user's expired quota still exists and is unmodified
	var finalQuota models.Quota
	if err := ctx.DB.Where("user_id = ? AND expiry_date = ?", giver.ID, soonExpireDate).First(&finalQuota).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get final quota: %v", err)}
	}

	if finalQuota.Amount != 80 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected original expired quota amount 80, got %d", finalQuota.Amount)}
	}

	// Verify expired quota status (may still be valid, because we didn't run expiry process)
	if finalQuota.Status != models.StatusValid {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected valid status for original quota, got %s", finalQuota.Status)}
	}

	return TestResult{Passed: true, Message: "Quota Expiry During Transfer Test Succeeded"}
}

// testBatchQuotaExpiryConsistency test consistency in batch quota operations with partial expiry
func testBatchQuotaExpiryConsistency(ctx *TestContext) TestResult {
	// Create test user
	user := createTestUser("user_batch_expiry", "User Batch Expiry", 0)
	if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Create multiple quotas with different expiry times
	now := time.Now().Truncate(time.Second)
	quotas := []*models.Quota{
		{UserID: user.ID, Amount: 100, ExpiryDate: now.Add(-2 * time.Hour), Status: models.StatusValid},    // expired
		{UserID: user.ID, Amount: 150, ExpiryDate: now.Add(-30 * time.Minute), Status: models.StatusValid}, // expired
		{UserID: user.ID, Amount: 200, ExpiryDate: now.Add(24 * time.Hour), Status: models.StatusValid},    // valid
		{UserID: user.ID, Amount: 120, ExpiryDate: now.Add(48 * time.Hour), Status: models.StatusValid},    // valid
	}

	for i, quota := range quotas {
		if err := ctx.DB.Create(quota).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Create quota %d failed: %v", i, err)}
		}
	}

	// Set mock quota total amount
	mockStore.SetQuota(user.ID, 570)

	// Execute quota expiry processing
	if err := ctx.QuotaService.ExpireQuotas(); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expire quotas failed: %v", err)}
	}

	// Verifyexpired quotastatus
	expiredCount := 0
	validCount := 0
	var allQuotas []models.Quota
	if err := ctx.DB.Where("user_id = ?", user.ID).Find(&allQuotas).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get all quotas: %v", err)}
	}

	for _, quota := range allQuotas {
		if quota.Status == models.StatusExpired {
			expiredCount++
			// Verify that expired quota time is indeed expired
			if quota.ExpiryDate.After(now) {
				return TestResult{Passed: false, Message: fmt.Sprintf("Quota with future expiry should not be expired: %v", quota.ExpiryDate)}
			}
		} else if quota.Status == models.StatusValid {
			validCount++
			// Verify that valid quota time is indeed not expired
			if quota.ExpiryDate.Before(now) {
				return TestResult{Passed: false, Message: fmt.Sprintf("Quota with past expiry should be expired: %v", quota.ExpiryDate)}
			}
		}
	}

	if expiredCount != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 expired quotas, got %d", expiredCount)}
	}

	if validCount != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 valid quotas, got %d", validCount)}
	}

	// Get user quota info, verify only calculates valid quota
	quotaInfo, err := ctx.QuotaService.GetUserQuota(user.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user quota failed: %v", err)}
	}

	expectedValidTotal := 200 + 120 // only non-expired quota
	if quotaInfo.TotalQuota != expectedValidTotal {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected total valid quota %d, got %d", expectedValidTotal, quotaInfo.TotalQuota)}
	}

	return TestResult{Passed: true, Message: "Batch Quota Expiry Consistency Test Succeeded"}
}

// testTransferOutExpiryDateValidation test expiry date validation when transferring out
func testTransferOutExpiryDateValidation(ctx *TestContext) TestResult {
	// Create test user
	giver := createTestUser("giver_expiry_validation", "Giver Expiry Validation", 0)
	if err := ctx.DB.AuthDB.Create(giver).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create giver user failed: %v", err)}
	}

	// Addquota
	validDate := time.Now().Truncate(time.Second).Add(30 * 24 * time.Hour)
	quota := &models.Quota{
		UserID:     giver.ID,
		Amount:     200,
		ExpiryDate: validDate,
		Status:     models.StatusValid,
	}
	if err := ctx.DB.Create(quota).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create quota failed: %v", err)}
	}

	mockStore.SetQuota(giver.ID, 200)

	giverAuth := &models.AuthUser{
		ID: giver.ID, Name: giver.Name, Phone: "13800138000", Github: "giver_validation",
	}

	// Test case 1: Try to transfer out with past expiry date
	pastDate := time.Now().Truncate(time.Second).Add(-24 * time.Hour)
	transferReqPast := &services.TransferOutRequest{
		ReceiverID: "receiver_validation",
		QuotaList: []services.TransferQuotaItem{
			{Amount: 50, ExpiryDate: pastDate},
		},
	}

	_, err := ctx.QuotaService.TransferOut(giverAuth, transferReqPast)
	if err == nil {
		return TestResult{Passed: false, Message: "Transfer out with past expiry date should fail"}
	}

	// Test case 2: Try to transfer out with future date that exceeds existing quota expiry date
	futureDate := validDate.Add(24 * time.Hour)
	transferReqFuture := &services.TransferOutRequest{
		ReceiverID: "receiver_validation",
		QuotaList: []services.TransferQuotaItem{
			{Amount: 50, ExpiryDate: futureDate},
		},
	}

	_, err = ctx.QuotaService.TransferOut(giverAuth, transferReqFuture)
	if err == nil {
		return TestResult{Passed: false, Message: "Transfer out with expiry date beyond available quota should fail"}
	}

	// Test case 3: Transfer out with correct expiry date (should succeed)
	transferReqValid := &services.TransferOutRequest{
		ReceiverID: "receiver_validation",
		QuotaList: []services.TransferQuotaItem{
			{Amount: 50, ExpiryDate: validDate},
		},
	}

	response, err := ctx.QuotaService.TransferOut(giverAuth, transferReqValid)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Valid transfer out should succeed: %v", err)}
	}

	if response.VoucherCode == "" {
		return TestResult{Passed: false, Message: "Valid transfer should generate voucher"}
	}

	// Verify that quota is correctly reduced
	var updatedQuota models.Quota
	if err := ctx.DB.Where("user_id = ? AND expiry_date = ?", giver.ID, validDate).First(&updatedQuota).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get updated quota: %v", err)}
	}

	if updatedQuota.Amount != 150 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected remaining quota 150, got %d", updatedQuota.Amount)}
	}

	return TestResult{Passed: true, Message: "Transfer Out Expiry Date Validation Test Succeeded"}
}
