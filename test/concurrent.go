package main

import (
	"fmt"
	"time"

	"quota-manager/internal/models"
	"quota-manager/internal/services"
)

func testConcurrentOperations(ctx *TestContext) TestResult {
	// Create test users using createTestUser function
	user1 := createTestUser("concurrent_1", "Concurrent User 1", 0)
	user2 := createTestUser("concurrent_2", "Concurrent User 2", 0)
	user3 := createTestUser("concurrent_3", "Concurrent User 3", 0)

	if err := ctx.DB.AuthDB.Create(user1).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user1 failed: %v", err)}
	}
	if err := ctx.DB.AuthDB.Create(user2).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user2 failed: %v", err)}
	}
	if err := ctx.DB.AuthDB.Create(user3).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Create user3 failed: %v", err)}
	}

	// Add initial quota for user1 (no need to set mock quota since AddQuotaForStrategy will handle AiGateway)
	if err := ctx.QuotaService.AddQuotaForStrategy(user1.ID, 500, "concurrent-test-strategy"); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Add initial quota failed: %v", err)}
	}

	// Create channels for synchronization
	resultChan := make(chan error, 10)
	startChan := make(chan struct{})

	// Concurrent operation 1: Multiple quota consumptions
	go func() {
		<-startChan
		for i := 0; i < 5; i++ {
			ctx.QuotaService.DeltaUsedQuotaInAiGateway(user1.ID, 10)
		}
		resultChan <- nil
	}()

	// Concurrent operation 2: Multiple transfer outs
	go func() {
		<-startChan
		// Use the same expiry date calculation as AddQuotaForStrategy
		now := time.Now().Truncate(time.Second)
		var expiry time.Time
		endOfMonth := time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, now.Location())
		if endOfMonth.Sub(now).Hours() < 24*30 {
			expiry = time.Date(now.Year(), now.Month()+2, 0, 23, 59, 59, 0, now.Location())
		} else {
			expiry = endOfMonth
		}

		for i := 0; i < 3; i++ {
			transferOutReq := &services.TransferOutRequest{
				ReceiverID: user2.ID,
				QuotaList: []services.TransferQuotaItem{
					{Amount: 30, ExpiryDate: expiry},
				},
			}
			_, err := ctx.QuotaService.TransferOut(&models.AuthUser{
				ID: user1.ID, Name: user1.Name, Phone: user1.Phone, Github: user1.GithubID,
			}, transferOutReq)
			resultChan <- err
		}
	}()

	// Concurrent operation 3: Multiple strategy executions
	go func() {
		<-startChan
		for i := 0; i < 2; i++ {
			err := ctx.QuotaService.AddQuotaForStrategy(user1.ID, 25, fmt.Sprintf("concurrent-strategy-%d", i))
			resultChan <- err
		}
	}()

	// Concurrent operation 4: Multiple quota queries
	go func() {
		<-startChan
		for i := 0; i < 5; i++ {
			_, err := ctx.QuotaService.GetUserQuota(user1.ID)
			if err != nil {
				resultChan <- err
				return
			}
		}
		resultChan <- nil
	}()

	// Start all operations simultaneously
	close(startChan)

	// Collect results
	var errors []error
	for i := 0; i < 7; i++ { // 1 + 3 + 2 + 1 operations
		if err := <-resultChan; err != nil {
			errors = append(errors, err)
		}
	}

	// Check if any operations failed
	if len(errors) > 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Concurrent operations had errors: %v", errors)}
	}

	// Verify final state consistency
	// Total quota should be: 500 (initial) + 50 (2 * 25 from strategies) - 90 (3 * 30 transfers) = 460
	// Used quota should be: 50 (5 * 10 consumption)
	// Remaining should be: 460 - 50 = 410

	finalQuotaInfo, err := ctx.QuotaService.GetUserQuota(user1.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get final quota info failed: %v", err)}
	}

	expectedTotal := 460 // 500 + 50 - 90
	expectedUsed := 50   // 5 * 10
	expectedRemaining := expectedTotal - expectedUsed

	if finalQuotaInfo.TotalQuota != expectedTotal {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected total quota %d, got %d", expectedTotal, finalQuotaInfo.TotalQuota)}
	}

	if finalQuotaInfo.UsedQuota != expectedUsed {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected used quota %d, got %d", expectedUsed, finalQuotaInfo.UsedQuota)}
	}

	actualRemaining := finalQuotaInfo.TotalQuota - finalQuotaInfo.UsedQuota
	if actualRemaining != expectedRemaining {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected remaining quota %d, got %d", expectedRemaining, actualRemaining)}
	}

	// Verify audit records consistency
	auditRecords, _, err := ctx.QuotaService.GetQuotaAuditRecords(user1.ID, 1, 100)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Get audit records failed: %v", err)}
	}

	// Should have 6 audit records: 1 initial + 2 strategies + 3 transfers
	expectedAuditCount := 6
	if len(auditRecords) != expectedAuditCount {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected %d audit records, got %d", expectedAuditCount, len(auditRecords))}
	}

	// Count operations by type
	rechargeCount := 0
	transferOutCount := 0
	for _, record := range auditRecords {
		switch record.Operation {
		case "RECHARGE":
			rechargeCount++
		case "TRANSFER_OUT":
			transferOutCount++
		}
	}

	if rechargeCount != 3 { // 1 initial + 2 concurrent strategies
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 3 recharge records, got %d", rechargeCount)}
	}

	if transferOutCount != 3 { // 3 concurrent transfers
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 3 transfer out records, got %d", transferOutCount)}
	}

	return TestResult{Passed: true, Message: "Concurrent operations test succeeded"}
}
