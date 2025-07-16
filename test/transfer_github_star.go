package main

import (
	"fmt"
	"strings"
	"time"

	"quota-manager/internal/models"
	"quota-manager/internal/services"
)

// testTransferInGithubStarNotSet tests transfer-in when giver doesn't have star
func testTransferInGithubStarNotSet(ctx *TestContext) TestResult {
	// Create test users
	giver := createTestUser("giver-no-star", "Giver No Star", 0)
	receiver := createTestUser("receiver-1", "Receiver 1", 0)

	// Save users to database
	if err := ctx.DB.AuthDB.Create(&giver).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create giver: %v", err)}
	}
	if err := ctx.DB.AuthDB.Create(&receiver).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create receiver: %v", err)}
	}

	// Clear previous SetGithubStarProjects calls
	mockStore.ClearSetStarProjectsCalls()

	// Create voucher data without GitHub star projects
	voucherData := &services.VoucherData{
		GiverID:         giver.ID,
		GiverName:       giver.Name,
		GiverPhone:      "13800138000",
		GiverGithub:     "giver-no-star",
		GiverGithubStar: "", // giver does not have starred projects
		ReceiverID:      receiver.ID,
		QuotaList: []services.VoucherQuotaItem{
			{
				Amount:     100,
				ExpiryDate: time.Now().Add(30 * 24 * time.Hour),
			},
		},
		Timestamp: time.Now().Unix(),
	}

	// Generate voucher code
	voucherCode, err := ctx.VoucherService.GenerateVoucher(voucherData)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to generate voucher: %v", err)}
	}

	// Perform transfer-in
	_, err = ctx.QuotaService.TransferIn(&models.AuthUser{
		ID:     receiver.ID,
		Name:   receiver.Name,
		Github: "receiver-1",
		Phone:  "13900139000",
	}, &services.TransferInRequest{
		VoucherCode: voucherCode,
	})
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer-in failed: %v", err)}
	}

	// Verify that no SetGithubStarProjects call was made (since giver didn't have starred projects)
	setStarProjectsCalls := mockStore.GetSetStarProjectsCalls()
	if len(setStarProjectsCalls) != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected no SetGithubStarProjects calls, but got %d calls", len(setStarProjectsCalls))}
	}

	return TestResult{Passed: true, Message: "Transfer-in correctly does not set GitHub star projects when giver has no starred projects"}
}

// testTransferInGithubStarSet tests transfer-in when giver has star
func testTransferInGithubStarSet(ctx *TestContext) TestResult {
	// Create test users
	giver := createTestUser("giver-with-star", "Giver With Star", 0)
	receiver := createTestUser("receiver-2", "Receiver 2", 0)

	// Save users to database
	if err := ctx.DB.AuthDB.Create(&giver).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create giver: %v", err)}
	}
	if err := ctx.DB.AuthDB.Create(&receiver).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create receiver: %v", err)}
	}

	// Clear previous SetGithubStarProjects calls
	mockStore.ClearSetStarProjectsCalls()

	// Create voucher data with GitHub star projects
	voucherData := &services.VoucherData{
		GiverID:         giver.ID,
		GiverName:       giver.Name,
		GiverPhone:      "13800138001",
		GiverGithub:     "giver-with-star",
		GiverGithubStar: "zgsm-ai.zgsm,microsoft/vscode", // giver has starred projects
		ReceiverID:      receiver.ID,
		QuotaList: []services.VoucherQuotaItem{
			{
				Amount:     200,
				ExpiryDate: time.Now().Add(30 * 24 * time.Hour),
			},
		},
		Timestamp: time.Now().Unix(),
	}

	// Generate voucher code
	voucherCode, err := ctx.VoucherService.GenerateVoucher(voucherData)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to generate voucher: %v", err)}
	}

	// Perform transfer-in
	_, err = ctx.QuotaService.TransferIn(&models.AuthUser{
		ID:     receiver.ID,
		Name:   receiver.Name,
		Github: "receiver-2",
		Phone:  "13900139001",
	}, &services.TransferInRequest{
		VoucherCode: voucherCode,
	})
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer-in failed: %v", err)}
	}

	// Verify that SetGithubStarProjects call was made for receiver with the projects
	setStarProjectsCalls := mockStore.GetSetStarProjectsCalls()
	if len(setStarProjectsCalls) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 SetGithubStarProjects call, but got %d calls", len(setStarProjectsCalls))}
	}

	call := setStarProjectsCalls[0]
	if call.EmployeeNumber != receiver.ID {
		return TestResult{Passed: false, Message: fmt.Sprintf("SetGithubStarProjects called for wrong user: expected %s, got %s", receiver.ID, call.EmployeeNumber)}
	}
	if call.StarredProjects != "zgsm-ai.zgsm,microsoft/vscode" {
		return TestResult{Passed: false, Message: fmt.Sprintf("SetGithubStarProjects called with wrong projects: expected 'zgsm-ai.zgsm,microsoft/vscode', got '%s'", call.StarredProjects)}
	}

	return TestResult{Passed: true, Message: "Transfer-in correctly sets GitHub star projects when giver has starred projects"}
}

// testTransferInGithubStarEmptyField tests transfer-in when voucher has no GitHub star field
func testTransferInGithubStarEmptyField(ctx *TestContext) TestResult {
	// Create test users
	giver := createTestUser("giver-empty", "Giver Empty", 0)
	receiver := createTestUser("receiver-3", "Receiver 3", 0)

	// Save users to database
	if err := ctx.DB.AuthDB.Create(&giver).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create giver: %v", err)}
	}
	if err := ctx.DB.AuthDB.Create(&receiver).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create receiver: %v", err)}
	}

	// Clear previous SetGithubStarProjects calls
	mockStore.ClearSetStarProjectsCalls()

	// Create voucher data with default (empty) GitHub star field
	voucherData := &services.VoucherData{
		GiverID:         giver.ID,
		GiverName:       giver.Name,
		GiverPhone:      "13800138002",
		GiverGithub:     "giver-empty-field",
		GiverGithubStar: "", // default/empty value
		ReceiverID:      receiver.ID,
		QuotaList: []services.VoucherQuotaItem{
			{
				Amount:     150,
				ExpiryDate: time.Now().Add(30 * 24 * time.Hour),
			},
		},
		Timestamp: time.Now().Unix(),
	}

	// Generate voucher code
	voucherCode, err := ctx.VoucherService.GenerateVoucher(voucherData)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to generate voucher: %v", err)}
	}

	// Perform transfer-in
	_, err = ctx.QuotaService.TransferIn(&models.AuthUser{
		ID:     receiver.ID,
		Name:   receiver.Name,
		Github: "receiver-3",
		Phone:  "13900139002",
	}, &services.TransferInRequest{
		VoucherCode: voucherCode,
	})
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer-in failed: %v", err)}
	}

	// Verify that no SetGithubStarProjects call was made (empty field defaults to no projects)
	setStarProjectsCalls := mockStore.GetSetStarProjectsCalls()
	if len(setStarProjectsCalls) != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected no SetGithubStarProjects calls, but got %d calls", len(setStarProjectsCalls))}
	}

	return TestResult{Passed: true, Message: "Transfer-in correctly handles empty GitHub star field"}
}

// testTransferOutGithubStarNotSet tests transfer-out when giver doesn't have star - voucher field verification
func testTransferOutGithubStarNotSet(ctx *TestContext) TestResult {
	// Create test users
	giver := createTestUser("giver-out-no-star", "Giver Out No Star", 0)
	giver.GithubStar = "microsoft/vscode,google/tensorflow" // Other projects, not zgsm-ai.zgsm
	receiver := createTestUser("receiver-out-1", "Receiver Out 1", 0)

	// Save users to database
	if err := ctx.DB.AuthDB.Create(&giver).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create giver: %v", err)}
	}
	if err := ctx.DB.AuthDB.Create(&receiver).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create receiver: %v", err)}
	}

	// Calculate transfer expiry date (same as other transfer tests)
	now := time.Now().Truncate(time.Second)
	var transferExpiryDate time.Time
	endOfMonth := time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, now.Location())
	if endOfMonth.Sub(now).Hours() < 24*30 {
		transferExpiryDate = time.Date(now.Year(), now.Month()+2, 0, 23, 59, 59, 0, now.Location())
	} else {
		transferExpiryDate = endOfMonth
	}

	// Add quota with the same expiry date that will be used in transfer
	if err := ctx.QuotaService.AddQuotaForStrategy(giver.ID, 200, "test-strategy"); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to add quota: %v", err)}
	}

	// Perform transfer-out
	transferOutReq := &services.TransferOutRequest{
		ReceiverID: receiver.ID,
		QuotaList: []services.TransferQuotaItem{
			{
				Amount:     100,
				ExpiryDate: transferExpiryDate,
			},
		},
	}

	transferOutResp, err := ctx.QuotaService.TransferOut(&models.AuthUser{
		ID:     giver.ID,
		Name:   giver.Name,
		Github: "giver-out-no-star",
		Phone:  "13800138000",
	}, transferOutReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer-out failed: %v", err)}
	}

	// Validate and decode the voucher
	voucherData, err := ctx.VoucherService.ValidateAndDecodeVoucher(transferOutResp.VoucherCode)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to decode voucher: %v", err)}
	}

	// Verify voucher fields
	if voucherData.GiverID != giver.ID {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected giver ID %s, got %s", giver.ID, voucherData.GiverID)}
	}
	if voucherData.GiverName != giver.Name {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected giver name %s, got %s", giver.Name, voucherData.GiverName)}
	}
	if voucherData.GiverGithub != "giver-out-no-star" {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected giver github 'giver-out-no-star', got %s", voucherData.GiverGithub)}
	}
	if voucherData.GiverGithubStar != "microsoft/vscode,google/tensorflow" {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected GiverGithubStar to be 'microsoft/vscode,google/tensorflow' (giver's starred projects), got %v", voucherData.GiverGithubStar)}
	}
	if voucherData.ReceiverID != receiver.ID {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected receiver ID %s, got %s", receiver.ID, voucherData.ReceiverID)}
	}
	if len(voucherData.QuotaList) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 quota item, got %d", len(voucherData.QuotaList))}
	}
	if voucherData.QuotaList[0].Amount != 100 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected quota amount 100, got %g", voucherData.QuotaList[0].Amount)}
	}

	return TestResult{Passed: true, Message: "Transfer-out correctly sets GiverGithubStar with all giver's starred projects"}
}

// testTransferOutGithubStarSet tests transfer-out when giver has star - voucher field verification
func testTransferOutGithubStarSet(ctx *TestContext) TestResult {
	// Create test users
	giver := createTestUser("giver-out-with-star", "Giver Out With Star", 0)
	giver.GithubStar = "microsoft/vscode,zgsm-ai.zgsm,google/tensorflow" // Including zgsm-ai.zgsm
	receiver := createTestUser("receiver-out-2", "Receiver Out 2", 0)

	// Save users to database
	if err := ctx.DB.AuthDB.Create(&giver).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create giver: %v", err)}
	}
	if err := ctx.DB.AuthDB.Create(&receiver).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create receiver: %v", err)}
	}

	// Calculate transfer expiry date (same as other transfer tests)
	now := time.Now().Truncate(time.Second)
	var transferExpiryDate time.Time
	endOfMonth := time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, now.Location())
	if endOfMonth.Sub(now).Hours() < 24*30 {
		transferExpiryDate = time.Date(now.Year(), now.Month()+2, 0, 23, 59, 59, 0, now.Location())
	} else {
		transferExpiryDate = endOfMonth
	}

	// Add quota with the same expiry date that will be used in transfer
	if err := ctx.QuotaService.AddQuotaForStrategy(giver.ID, 200, "test-strategy"); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to add quota: %v", err)}
	}

	// Perform transfer-out
	transferOutReq := &services.TransferOutRequest{
		ReceiverID: receiver.ID,
		QuotaList: []services.TransferQuotaItem{
			{
				Amount:     100,
				ExpiryDate: transferExpiryDate,
			},
		},
	}

	transferOutResp, err := ctx.QuotaService.TransferOut(&models.AuthUser{
		ID:     giver.ID,
		Name:   giver.Name,
		Github: "giver-out-with-star",
		Phone:  "13800138000",
	}, transferOutReq)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Transfer-out failed: %v", err)}
	}

	// Validate and decode the voucher
	voucherData, err := ctx.VoucherService.ValidateAndDecodeVoucher(transferOutResp.VoucherCode)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to decode voucher: %v", err)}
	}

	// Verify voucher fields
	if voucherData.GiverID != giver.ID {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected giver ID %s, got %s", giver.ID, voucherData.GiverID)}
	}
	if voucherData.GiverName != giver.Name {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected giver name %s, got %s", giver.Name, voucherData.GiverName)}
	}
	if voucherData.GiverGithub != "giver-out-with-star" {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected giver github 'giver-out-with-star', got %s", voucherData.GiverGithub)}
	}
	if !strings.Contains(voucherData.GiverGithubStar, "zgsm-ai.zgsm") {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected GiverGithubStar to contain zgsm-ai.zgsm (giver stars zgsm-ai.zgsm), got %v", voucherData.GiverGithubStar)}
	}
	if voucherData.ReceiverID != receiver.ID {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected receiver ID %s, got %s", receiver.ID, voucherData.ReceiverID)}
	}
	if len(voucherData.QuotaList) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 quota item, got %d", len(voucherData.QuotaList))}
	}
	if voucherData.QuotaList[0].Amount != 100 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected quota amount 100, got %g", voucherData.QuotaList[0].Amount)}
	}

	return TestResult{Passed: true, Message: "Transfer-out correctly sets GiverGithubStar with projects when giver stars zgsm-ai.zgsm"}
}
