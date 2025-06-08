package main

import (
	"fmt"
	"strings"
	"time"

	"quota-manager/internal/models"
	"quota-manager/internal/services"
)

// testQuotaTransferIn test quota transfer in
func testQuotaTransferIn(ctx *TestContext) TestResult {
	// Create test users
	receiver := &models.UserInfo{
		ID:           "receiver_user",
		Name:         "Receiver User",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}
	if err := ctx.DB.Create(receiver).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create receiver user failed: %v", err)}
	}

	// Generate a valid voucher
	expiryDate := time.Now().Add(30 * 24 * time.Hour)
	voucherData := &services.VoucherData{
		GiverID:     "giver_user",
		GiverName:   "Giver User",
		GiverPhone:  "13800138000",
		GiverGithub: "giver",
		ReceiverID:  receiver.ID,
		QuotaList: []services.VoucherQuotaItem{
			{Amount: 30, ExpiryDate: expiryDate},
		},
	}

	voucherCode, err := ctx.VoucherService.GenerateVoucher(voucherData)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Generate voucher failed: %v", err)}
	}

	// Create AuthUser for receiver
	receiverAuth := &models.AuthUser{
		ID:      receiver.ID,
		Name:    receiver.Name,
		StaffID: "test_staff_id",
		Github:  "receiver",
		Phone:   "13900139000",
	}

	// Transfer in request
	transferReq := &services.TransferInRequest{
		VoucherCode: voucherCode,
	}

	// Execute transfer in
	response, err := ctx.QuotaService.TransferIn(receiverAuth, transferReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer in failed: %v", err)}
	}

	if response.GiverID != voucherData.GiverID {
		return TestResult{Passed: false, Message: "Transfer in response giver ID mismatch"}
	}

	// Verify receiver's quota is added
	var quota models.Quota
	if err := ctx.DB.Where("user_id = ? AND expiry_date = ?", receiver.ID, expiryDate).First(&quota).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get receiver quota: %v", err)}
	}

	if quota.Amount != 30 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected receiver quota 30, got %d", quota.Amount)}
	}

	// Verify audit record
	var auditRecord models.QuotaAudit
	if err := ctx.DB.Where("user_id = ? AND operation = ?", receiver.ID, models.OperationTransferIn).First(&auditRecord).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get audit record: %v", err)}
	}

	if auditRecord.Amount != 30 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected audit amount 30, got %d", auditRecord.Amount)}
	}

	// Verify voucher redemption record
	var redemption models.VoucherRedemption
	if err := ctx.DB.Where("voucher_code = ?", voucherCode).First(&redemption).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get redemption record: %v", err)}
	}

	// Test duplicate redemption
	duplicateResp, err := ctx.QuotaService.TransferIn(receiverAuth, transferReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Duplicate redemption check failed: %v", err)}
	}
	if duplicateResp.Status != services.TransferStatusAlreadyRedeemed {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected ALREADY_REDEEMED status, got %s", duplicateResp.Status)}
	}

	return TestResult{Passed: true, Message: "Quota Transfer In Test Succeeded"}
}

func testTransferInUserIDMismatch(ctx *TestContext) TestResult {
	// Create test users
	user1 := &models.UserInfo{
		ID:           "user_mismatch_1",
		Name:         "Mismatch User 1",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}
	user2 := &models.UserInfo{
		ID:           "user_mismatch_2",
		Name:         "Mismatch User 2",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}
	user3 := &models.UserInfo{
		ID:           "user_mismatch_3",
		Name:         "Mismatch User 3",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}

	if err := ctx.DB.Create(user1).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user1 failed: %v", err)}
	}
	if err := ctx.DB.Create(user2).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user2 failed: %v", err)}
	}
	if err := ctx.DB.Create(user3).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user3 failed: %v", err)}
	}

	// Initialize mock quota
	mockStore.SetQuota(user1.ID, 100)

	// Add quota for user1
	if err := ctx.QuotaService.AddQuotaForStrategy(user1.ID, 100, "test-strategy"); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Add quota failed: %v", err)}
	}

	// Transfer quota from user1 to user2 - use same expiry date as created by strategy
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
			{Amount: 50, ExpiryDate: transferExpiryDate},
		},
	}
	transferOutResp, err := ctx.QuotaService.TransferOut(&models.AuthUser{
		ID: user1.ID, Name: user1.Name, Phone: "13800138000", Github: "user1",
	}, transferOutReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer out failed: %v", err)}
	}

	// Try to transfer in with user3 (should fail as voucher is for user2)
	transferInReq := &services.TransferInRequest{
		VoucherCode: transferOutResp.VoucherCode,
	}
	_, err = ctx.QuotaService.TransferIn(&models.AuthUser{
		ID: user3.ID, Name: user3.Name, Phone: "13700137000", Github: "user3",
	}, transferInReq)
	if err == nil {
		return TestResult{Passed: false, Message: "Transfer in should have failed with mismatched user ID"}
	}

	// Verify the error message contains appropriate information
	if !strings.Contains(err.Error(), "voucher is not for this user") {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 'voucher is not for this user' error, got: %v", err)}
	}

	// Verify user3 has no quota records
	quotaInfo3, err := ctx.QuotaService.GetUserQuota(user3.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user3 quota failed: %v", err)}
	}
	if quotaInfo3.TotalQuota != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User3 should have no quota, got %d", quotaInfo3.TotalQuota)}
	}

	// Verify no audit records for user3
	_, auditCount3, err := ctx.QuotaService.GetQuotaAuditRecords(user3.ID, 1, 100)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user3 audit records failed: %v", err)}
	}
	if auditCount3 != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User3 should have no audit records, got %d", auditCount3)}
	}

	// Verify the voucher is still available for the correct user (user2)
	_, err = ctx.QuotaService.TransferIn(&models.AuthUser{
		ID: user2.ID, Name: user2.Name, Phone: "13900139000", Github: "user2",
	}, transferInReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer in with correct user failed: %v", err)}
	}

	return TestResult{Passed: true, Message: "Transfer in user ID mismatch test succeeded"}
}

func testTransferInExpiredQuota(ctx *TestContext) TestResult {
	// Create test users
	user1 := &models.UserInfo{
		ID:           "user_expired_1",
		Name:         "Expired User 1",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}
	user2 := &models.UserInfo{
		ID:           "user_expired_2",
		Name:         "Expired User 2",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}

	if err := ctx.DB.Create(user1).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user1 failed: %v", err)}
	}
	if err := ctx.DB.Create(user2).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user2 failed: %v", err)}
	}

	// Initialize mock quota
	mockStore.SetQuota(user1.ID, 200)

	// Add quota with mixed expiry dates - some expired, some valid
	now := time.Now()

	// Add 100 quota that already expired (yesterday)
	expiredDate := now.AddDate(0, 0, -1)
	if err := ctx.DB.Create(&models.Quota{
		UserID:     user1.ID,
		Amount:     100,
		ExpiryDate: expiredDate,
		Status:     models.StatusExpired,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create expired quota failed: %v", err)}
	}

	// Add 100 quota that is still valid (expires in 30 days)
	validDate := now.AddDate(0, 0, 30)
	if err := ctx.DB.Create(&models.Quota{
		UserID:     user1.ID,
		Amount:     100,
		ExpiryDate: validDate,
		Status:     models.StatusValid,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create valid quota failed: %v", err)}
	}

	// Transfer out both quotas (including expired one)
	transferOutReq := &services.TransferOutRequest{
		ReceiverID: user2.ID,
		QuotaList: []services.TransferQuotaItem{
			{Amount: 100, ExpiryDate: expiredDate}, // Expired quota
			{Amount: 50, ExpiryDate: validDate},    // Valid quota
		},
	}
	transferOutResp, err := ctx.QuotaService.TransferOut(&models.AuthUser{
		ID: user1.ID, Name: user1.Name, Phone: "13800138000", Github: "user1",
	}, transferOutReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer out failed: %v", err)}
	}

	// Transfer in - should only get valid quota
	transferInReq := &services.TransferInRequest{
		VoucherCode: transferOutResp.VoucherCode,
	}
	transferInResp, err := ctx.QuotaService.TransferIn(&models.AuthUser{
		ID: user2.ID, Name: user2.Name, Phone: "13900139000", Github: "user2",
	}, transferInReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer in failed: %v", err)}
	}

	// Verify that only valid quota was transferred
	// Should only get 50 quota (expired quota should be ignored)
	if transferInResp.Amount != 50 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 50 transferred quota (excluding expired), got %d", transferInResp.Amount)}
	}

	// Verify user2's quota records
	var quotaRecords []models.Quota
	if err := ctx.DB.DB.Where("user_id = ?", user2.ID).Find(&quotaRecords).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user2 quota records failed: %v", err)}
	}

	// Should only have one quota record (the valid one)
	if len(quotaRecords) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 quota record for user2, got %d", len(quotaRecords))}
	}

	// Verify the quota record has the correct expiry date (should be the valid date)
	if !quotaRecords[0].ExpiryDate.Equal(validDate) {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected quota record expiry date to match valid date, got %v", quotaRecords[0].ExpiryDate)}
	}

	// Verify the audit record uses earliest expiry date from valid quotas only
	auditRecords, _, err := ctx.QuotaService.GetQuotaAuditRecords(user2.ID, 1, 100)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user2 audit records failed: %v", err)}
	}

	if len(auditRecords) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 audit record for user2, got %d", len(auditRecords))}
	}

	// The audit record should have the valid date as expiry date (not the expired date)
	if !auditRecords[0].ExpiryDate.Equal(validDate) {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected audit record expiry date to be valid date, got %v", auditRecords[0].ExpiryDate)}
	}

	// Verify user2's total quota
	quotaInfo2, err := ctx.QuotaService.GetUserQuota(user2.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user2 quota failed: %v", err)}
	}

	if quotaInfo2.TotalQuota != 50 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected user2 total quota 50, got %d", quotaInfo2.TotalQuota)}
	}

	return TestResult{Passed: true, Message: "Transfer in expired quota test succeeded"}
}

func testTransferInInvalidVoucher(ctx *TestContext) TestResult {
	// Create test user
	user := &models.UserInfo{
		ID:           "user_invalid_voucher",
		Name:         "Invalid Voucher User",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}

	if err := ctx.DB.Create(user).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user failed: %v", err)}
	}

	// Test case 1: Completely invalid voucher code (too short)
	transferInReq1 := &services.TransferInRequest{
		VoucherCode: "invalid",
	}
	_, err := ctx.QuotaService.TransferIn(&models.AuthUser{
		ID: user.ID, Name: user.Name, Phone: "13800138000", Github: "user",
	}, transferInReq1)

	if err == nil {
		return TestResult{Passed: false, Message: "Should fail with completely invalid voucher"}
	}

	// Test case 2: Voucher with invalid format (missing separators)
	transferInReq2 := &services.TransferInRequest{
		VoucherCode: "invalidvouchercodewithoutanyseparators",
	}
	_, err = ctx.QuotaService.TransferIn(&models.AuthUser{
		ID: user.ID, Name: user.Name, Phone: "13800138000", Github: "user",
	}, transferInReq2)

	if err == nil {
		return TestResult{Passed: false, Message: "Should fail with invalid format voucher"}
	}

	// Test case 3: Voucher with tampered signature
	// Create a valid voucher structure but with wrong signature
	tamperedVoucher := "user1|receiver1|100|2024-12-31T23:59:59Z|tampered_signature"
	transferInReq3 := &services.TransferInRequest{
		VoucherCode: tamperedVoucher,
	}
	_, err = ctx.QuotaService.TransferIn(&models.AuthUser{
		ID: user.ID, Name: user.Name, Phone: "13800138000", Github: "user",
	}, transferInReq3)

	if err == nil {
		return TestResult{Passed: false, Message: "Should fail with tampered signature voucher"}
	}

	// Verify that no quota was transferred to the user
	quotaInfo, err := ctx.QuotaService.GetUserQuota(user.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user quota failed: %v", err)}
	}

	if quotaInfo.TotalQuota != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User should have no quota after failed transfers, got %d", quotaInfo.TotalQuota)}
	}

	// Verify no audit records were created
	_, auditCount, err := ctx.QuotaService.GetQuotaAuditRecords(user.ID, 1, 100)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get audit records failed: %v", err)}
	}

	if auditCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Should have no audit records for failed transfers, got %d", auditCount)}
	}

	return TestResult{Passed: true, Message: "Transfer in invalid voucher test succeeded"}
}

func testTransferInQuotaExpiryConsistency(ctx *TestContext) TestResult {
	// Create test users
	user1 := &models.UserInfo{
		ID:           "user_expiry_consistency_1",
		Name:         "Expiry Consistency User 1",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
	}
	user2 := &models.UserInfo{
		ID:           "user_expiry_consistency_2",
		Name:         "Expiry Consistency User 2",
		RegisterTime: time.Now().Add(-time.Hour * 24),
		AccessTime:   time.Now().Add(-time.Hour * 1),
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
	now := time.Now()

	// Add quota expiring in 15 days
	earlyExpiry := now.AddDate(0, 0, 15)
	if err := ctx.DB.Create(&models.Quota{
		UserID:     user1.ID,
		Amount:     50,
		ExpiryDate: earlyExpiry,
		Status:     models.StatusValid,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create early quota failed: %v", err)}
	}

	// Add quota expiring in 45 days
	lateExpiry := now.AddDate(0, 0, 45)
	if err := ctx.DB.Create(&models.Quota{
		UserID:     user1.ID,
		Amount:     150,
		ExpiryDate: lateExpiry,
		Status:     models.StatusValid,
	}).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create late quota failed: %v", err)}
	}

	// Transfer out with specific expiry dates
	transferOutReq := &services.TransferOutRequest{
		ReceiverID: user2.ID,
		QuotaList: []services.TransferQuotaItem{
			{Amount: 30, ExpiryDate: earlyExpiry}, // Early expiry
			{Amount: 70, ExpiryDate: lateExpiry},  // Late expiry
		},
	}
	transferOutResp, err := ctx.QuotaService.TransferOut(&models.AuthUser{
		ID: user1.ID, Name: user1.Name, Phone: "13800138000", Github: "user1",
	}, transferOutReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer out failed: %v", err)}
	}

	// Transfer in
	transferInReq := &services.TransferInRequest{
		VoucherCode: transferOutResp.VoucherCode,
	}
	_, err = ctx.QuotaService.TransferIn(&models.AuthUser{
		ID: user2.ID, Name: user2.Name, Phone: "13900139000", Github: "user2",
	}, transferInReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer in failed: %v", err)}
	}

	// Verify the audit record for user2 has the earliest expiry date (earlyExpiry)
	auditRecords2, _, err := ctx.QuotaService.GetQuotaAuditRecords(user2.ID, 1, 100)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user2 audit records failed: %v", err)}
	}

	if len(auditRecords2) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 audit record for user2, got %d", len(auditRecords2))}
	}

	// The audit record should have the earliest expiry date
	if !auditRecords2[0].ExpiryDate.Equal(earlyExpiry) {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected audit record expiry date to be %v, got %v", earlyExpiry, auditRecords2[0].ExpiryDate)}
	}

	// Verify user2's quota records have correct individual expiry dates
	var quotaRecords []models.Quota
	if err := ctx.DB.DB.Where("user_id = ?", user2.ID).Order("expiry_date ASC").Find(&quotaRecords).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user2 quota records failed: %v", err)}
	}

	if len(quotaRecords) != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 quota records for user2, got %d", len(quotaRecords))}
	}

	// First record should have early expiry
	if !quotaRecords[0].ExpiryDate.Equal(earlyExpiry) {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected first quota record expiry to be %v, got %v", earlyExpiry, quotaRecords[0].ExpiryDate)}
	}

	// Second record should have late expiry
	if !quotaRecords[1].ExpiryDate.Equal(lateExpiry) {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected second quota record expiry to be %v, got %v", lateExpiry, quotaRecords[1].ExpiryDate)}
	}

	// Verify the audit record for user1 (transfer out) also has the earliest expiry date
	auditRecords1, _, err := ctx.QuotaService.GetQuotaAuditRecords(user1.ID, 1, 100)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get user1 audit records failed: %v", err)}
	}

	// Find the transfer out record (should be the first one for transfer out)
	transferOutRecord := auditRecords1[0]
	if transferOutRecord.Operation != "TRANSFER_OUT" {
		// Find the transfer out record if not the first
		for _, record := range auditRecords1 {
			if record.Operation == "TRANSFER_OUT" {
				transferOutRecord = record
				break
			}
		}
	}

	// The transfer out audit record should also have the earliest expiry date
	if !transferOutRecord.ExpiryDate.Equal(earlyExpiry) {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected transfer out audit record expiry date to be %v, got %v", earlyExpiry, transferOutRecord.ExpiryDate)}
	}

	return TestResult{Passed: true, Message: "Transfer in quota expiry consistency test succeeded"}
}
