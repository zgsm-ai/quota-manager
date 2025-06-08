package main

import (
	"fmt"
	"strings"
	"time"

	"quota-manager/internal/models"
	"quota-manager/internal/services"
)

// testVoucherGenerationAndValidation test voucher generation and validation
func testVoucherGenerationAndValidation(ctx *TestContext) TestResult {
	// Test voucher data
	voucherData := &services.VoucherData{
		GiverID:     "giver123",
		GiverName:   "张三",
		GiverPhone:  "13800138000",
		GiverGithub: "zhangsan",
		ReceiverID:  "receiver456",
		QuotaList: []services.VoucherQuotaItem{
			{Amount: 10, ExpiryDate: time.Now().Truncate(time.Second).Add(30 * 24 * time.Hour)},
			{Amount: 20, ExpiryDate: time.Now().Truncate(time.Second).Add(60 * 24 * time.Hour)},
		},
	}

	// Generate voucher
	voucherCode, err := ctx.VoucherService.GenerateVoucher(voucherData)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Generate voucher failed: %v", err)}
	}

	if voucherCode == "" {
		return TestResult{Passed: false, Message: "Generated voucher code is empty"}
	}

	// Validate and decode voucher
	decodedData, err := ctx.VoucherService.ValidateAndDecodeVoucher(voucherCode)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Validate voucher failed: %v", err)}
	}

	// Verify decoded data
	if decodedData.GiverID != voucherData.GiverID ||
		decodedData.GiverName != voucherData.GiverName ||
		decodedData.ReceiverID != voucherData.ReceiverID ||
		len(decodedData.QuotaList) != len(voucherData.QuotaList) {
		return TestResult{Passed: false, Message: "Decoded voucher data mismatch"}
	}

	// Test invalid voucher
	_, err = ctx.VoucherService.ValidateAndDecodeVoucher("invalid-voucher-code")
	if err == nil {
		return TestResult{Passed: false, Message: "Invalid voucher should fail validation"}
	}

	return TestResult{Passed: true, Message: "Voucher Generation and Validation Test Succeeded"}
}

// testQuotaTransferOut test quota transfer out
func testQuotaTransferOut(ctx *TestContext) TestResult {
	// Create test users
	giver := &models.UserInfo{
		ID:             "giver_user",
		Name:           "Giver User",
		Phone:          "13800138000",
		GithubUsername: "giver",
		RegisterTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 24),
		AccessTime:     time.Now().Truncate(time.Second).Add(-time.Hour * 1),
	}
	if err := ctx.DB.Create(giver).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create giver user failed: %v", err)}
	}

	// Add initial quota for giver
	expiryDate := time.Now().Truncate(time.Second).Add(30 * 24 * time.Hour)
	quota := &models.Quota{
		UserID:     giver.ID,
		Amount:     100,
		ExpiryDate: expiryDate,
		Status:     models.StatusValid,
	}
	if err := ctx.DB.Create(quota).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create initial quota failed: %v", err)}
	}

	// Create AuthUser for giver
	giverAuth := &models.AuthUser{
		ID:      giver.ID,
		Name:    giver.Name,
		StaffID: "test_staff_id",
		Github:  giver.GithubUsername,
		Phone:   giver.Phone,
	}

	// Transfer out request
	transferReq := &services.TransferOutRequest{
		ReceiverID: "receiver_user",
		QuotaList: []services.TransferQuotaItem{
			{Amount: 30, ExpiryDate: expiryDate},
		},
	}

	// Execute transfer out
	response, err := ctx.QuotaService.TransferOut(giverAuth, transferReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer out failed: %v", err)}
	}

	if response.VoucherCode == "" {
		return TestResult{Passed: false, Message: "Voucher code is empty"}
	}

	// Verify giver's quota is reduced
	var updatedQuota models.Quota
	if err := ctx.DB.Where("user_id = ? AND expiry_date = ?", giver.ID, expiryDate).First(&updatedQuota).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get updated quota: %v", err)}
	}

	if updatedQuota.Amount != 70 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected remaining quota 70, got %d", updatedQuota.Amount)}
	}

	// Verify audit record
	var auditRecord models.QuotaAudit
	if err := ctx.DB.Where("user_id = ? AND operation = ?", giver.ID, models.OperationTransferOut).First(&auditRecord).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get audit record: %v", err)}
	}

	if auditRecord.Amount != -30 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected audit amount -30, got %d", auditRecord.Amount)}
	}

	return TestResult{Passed: true, Message: "Quota Transfer Out Test Succeeded"}
}

func testTransferOutInsufficientAvailable(ctx *TestContext) TestResult {
	// Create test users
	user1 := &models.UserInfo{
		ID:           "user_insufficient_1",
		Name:         "Insufficient User 1",
		RegisterTime: time.Now().Truncate(time.Second).Add(-time.Hour * 24),
		AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
	}
	user2 := &models.UserInfo{
		ID:           "user_insufficient_2",
		Name:         "Insufficient User 2",
		RegisterTime: time.Now().Truncate(time.Second).Add(-time.Hour * 24),
		AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
	}

	if err := ctx.DB.Create(user1).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user1 failed: %v", err)}
	}
	if err := ctx.DB.Create(user2).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user2 failed: %v", err)}
	}

	// Initialize mock quota
	mockStore.SetQuota(user1.ID, 200)

	// Add quota with different expiry dates
	now := time.Now().Truncate(time.Second)

	// Add 100 quota expiring in 10 days
	earlyExpiry := now.AddDate(0, 0, 10)
	if err := ctx.DB.Create(&models.Quota{
		UserID:     user1.ID,
		Amount:     100,
		ExpiryDate: earlyExpiry,
		Status:     models.StatusValid,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create early quota failed: %v", err)}
	}

	// Add 100 quota expiring in 30 days
	lateExpiry := now.AddDate(0, 0, 30)
	if err := ctx.DB.Create(&models.Quota{
		UserID:     user1.ID,
		Amount:     100,
		ExpiryDate: lateExpiry,
		Status:     models.StatusValid,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create late quota failed: %v", err)}
	}

	// Consume 120 quota (should consume all 100 from early + 20 from late)
	// This leaves 80 available in late-expiry quota
	ctx.QuotaService.DeltaUsedQuotaInAiGateway(user1.ID, 120)

	// Try to transfer 90 quota with early expiry date (should fail - only has 0 available with early expiry)
	transferOutReq := &services.TransferOutRequest{
		ReceiverID: user2.ID,
		QuotaList: []services.TransferQuotaItem{
			{Amount: 90, ExpiryDate: earlyExpiry},
		},
	}
	_, err := ctx.QuotaService.TransferOut(&models.AuthUser{
		ID: user1.ID, Name: user1.Name, Phone: "13800138000", Github: "user1",
	}, transferOutReq)

	if err == nil {
		return TestResult{Passed: false, Message: "Transfer out should have failed due to insufficient available quota for specific expiry date"}
	}

	// Verify the error message indicates insufficient available quota
	if !strings.Contains(err.Error(), "insufficient available quota") && !strings.Contains(err.Error(), "not enough quota") {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected insufficient quota error, got: %v", err)}
	}

	// Try to transfer 80 quota with late expiry date (should succeed)
	transferOutReq2 := &services.TransferOutRequest{
		ReceiverID: user2.ID,
		QuotaList: []services.TransferQuotaItem{
			{Amount: 80, ExpiryDate: lateExpiry},
		},
	}
	_, err = ctx.QuotaService.TransferOut(&models.AuthUser{
		ID: user1.ID, Name: user1.Name, Phone: "13800138000", Github: "user1",
	}, transferOutReq2)

	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer out with sufficient available quota should succeed: %v", err)}
	}

	// Verify user1's remaining quota
	quotaInfo1, err := ctx.QuotaService.GetUserQuota(user1.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user1 quota failed: %v", err)}
	}

	// Should have 0 remaining quota (all consumed or transferred)
	actualRemaining1 := quotaInfo1.TotalQuota - quotaInfo1.UsedQuota
	if actualRemaining1 != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 0 remaining quota, got %d", actualRemaining1)}
	}

	// Try to transfer 1 more quota (should fail - no remaining quota)
	transferOutReq3 := &services.TransferOutRequest{
		ReceiverID: user2.ID,
		QuotaList: []services.TransferQuotaItem{
			{Amount: 1, ExpiryDate: lateExpiry},
		},
	}
	_, err = ctx.QuotaService.TransferOut(&models.AuthUser{
		ID: user1.ID, Name: user1.Name, Phone: "13800138000", Github: "user1",
	}, transferOutReq3)

	if err == nil {
		return TestResult{Passed: false, Message: "Transfer out should have failed due to no remaining quota"}
	}

	return TestResult{Passed: true, Message: "Transfer out insufficient available quota test succeeded"}
}

func testTransferEarliestExpiryDate(ctx *TestContext) TestResult {
	// Create test users
	user1 := &models.UserInfo{
		ID:           "user_earliest_expiry_1",
		Name:         "Earliest Expiry User 1",
		RegisterTime: time.Now().Truncate(time.Second).Add(-time.Hour * 24),
		AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
	}
	user2 := &models.UserInfo{
		ID:           "user_earliest_expiry_2",
		Name:         "Earliest Expiry User 2",
		RegisterTime: time.Now().Truncate(time.Second).Add(-time.Hour * 24),
		AccessTime:   time.Now().Truncate(time.Second).Add(-time.Hour * 1),
	}

	if err := ctx.DB.Create(user1).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user1 failed: %v", err)}
	}
	if err := ctx.DB.Create(user2).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user2 failed: %v", err)}
	}

	// Initialize mock quota
	mockStore.SetQuota(user1.ID, 300)

	// Add quota with multiple expiry dates
	now := time.Now().Truncate(time.Second)

	expiry1 := now.AddDate(0, 0, 10) // Earliest
	expiry2 := now.AddDate(0, 0, 20) // Middle
	expiry3 := now.AddDate(0, 0, 30) // Latest

	// Add quotas in non-chronological order to test ordering
	if err := ctx.DB.Create(&models.Quota{
		UserID:     user1.ID,
		Amount:     50,
		ExpiryDate: expiry2, // Middle expiry
		Status:     models.StatusValid,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create quota2 failed: %v", err)}
	}

	if err := ctx.DB.Create(&models.Quota{
		UserID:     user1.ID,
		Amount:     100,
		ExpiryDate: expiry1, // Earliest expiry
		Status:     models.StatusValid,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create quota1 failed: %v", err)}
	}

	if err := ctx.DB.Create(&models.Quota{
		UserID:     user1.ID,
		Amount:     150,
		ExpiryDate: expiry3, // Latest expiry
		Status:     models.StatusValid,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create quota3 failed: %v", err)}
	}

	// Transfer out multiple quotas with different expiry dates
	transferOutReq := &services.TransferOutRequest{
		ReceiverID: user2.ID,
		QuotaList: []services.TransferQuotaItem{
			{Amount: 50, ExpiryDate: expiry2}, // Middle expiry
			{Amount: 80, ExpiryDate: expiry1}, // Earliest expiry
			{Amount: 70, ExpiryDate: expiry3}, // Latest expiry
		},
	}

	transferOutResp, err := ctx.QuotaService.TransferOut(&models.AuthUser{
		ID: user1.ID, Name: user1.Name, Phone: "13800138000", Github: "user1",
	}, transferOutReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer out failed: %v", err)}
	}

	// Verify the transfer out audit record uses the earliest expiry date
	auditRecords1, _, err := ctx.QuotaService.GetQuotaAuditRecords(user1.ID, 1, 100)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user1 audit records failed: %v", err)}
	}

	// Find the transfer out record
	var transferOutRecord *services.QuotaAuditRecord
	for i, record := range auditRecords1 {
		if record.Operation == "TRANSFER_OUT" {
			transferOutRecord = &auditRecords1[i]
			break
		}
	}

	if transferOutRecord == nil {
		return TestResult{Passed: false, Message: "Transfer out audit record not found"}
	}

	// The audit record should use the earliest expiry date (expiry1)
	if !transferOutRecord.ExpiryDate.Equal(expiry1) {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer out audit record should use earliest expiry date %v, got %v", expiry1, transferOutRecord.ExpiryDate)}
	}

	// Transfer in and verify the same logic
	transferInReq := &services.TransferInRequest{
		VoucherCode: transferOutResp.VoucherCode,
	}

	_, err = ctx.QuotaService.TransferIn(&models.AuthUser{
		ID: user2.ID, Name: user2.Name, Phone: "13900139000", Github: "user2",
	}, transferInReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer in failed: %v", err)}
	}

	// Verify the transfer in audit record also uses the earliest expiry date
	auditRecords2, _, err := ctx.QuotaService.GetQuotaAuditRecords(user2.ID, 1, 100)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user2 audit records failed: %v", err)}
	}

	if len(auditRecords2) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 audit record for user2, got %d", len(auditRecords2))}
	}

	// The transfer in audit record should also use the earliest expiry date
	if !auditRecords2[0].ExpiryDate.Equal(expiry1) {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer in audit record should use earliest expiry date %v, got %v", expiry1, auditRecords2[0].ExpiryDate)}
	}

	// Additional test: Transfer out with only non-earliest expiry dates
	// Add more quota to user1 (update mock store too)
	mockStore.DeltaQuota(user1.ID, 200) // Add 200 more to mock store
	if err := ctx.DB.Create(&models.Quota{
		UserID:     user1.ID,
		Amount:     200,
		ExpiryDate: expiry2, // Middle expiry
		Status:     models.StatusValid,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create additional quota failed: %v", err)}
	}

	// Transfer out only from middle and late expiry dates
	transferOutReq2 := &services.TransferOutRequest{
		ReceiverID: user2.ID,
		QuotaList: []services.TransferQuotaItem{
			{Amount: 30, ExpiryDate: expiry3}, // Latest expiry
			{Amount: 40, ExpiryDate: expiry2}, // Middle expiry
		},
	}

	_, err = ctx.QuotaService.TransferOut(&models.AuthUser{
		ID: user1.ID, Name: user1.Name, Phone: "13800138000", Github: "user1",
	}, transferOutReq2)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Second transfer out failed: %v", err)}
	}

	// Get the latest audit records for user1
	auditRecords1Again, _, err := ctx.QuotaService.GetQuotaAuditRecords(user1.ID, 1, 100)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user1 audit records again failed: %v", err)}
	}

	// Find the second transfer out record (should be the first in the list due to DESC order)
	secondTransferOut := auditRecords1Again[0]
	if secondTransferOut.Operation != "TRANSFER_OUT" {
		return TestResult{Passed: false, Message: "Expected first record to be the latest transfer out"}
	}

	// This transfer out should use the earliest among the transferred expiry dates (expiry2)
	if !secondTransferOut.ExpiryDate.Equal(expiry2) {
		return TestResult{Passed: false, Message: fmt.Sprintf("Second transfer out audit record should use earliest transferred expiry date %v, got %v", expiry2, secondTransferOut.ExpiryDate)}
	}

	return TestResult{Passed: true, Message: "Transfer earliest expiry date test succeeded"}
}
