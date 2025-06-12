package main

import (
	"fmt"
	"log"
	"net/http/httptest"
	"time"

	"quota-manager/internal/condition"
	"quota-manager/internal/database"
	"quota-manager/internal/services"
	"quota-manager/pkg/aigateway"
)

// TestContext test context
type TestContext struct {
	DB              *database.DB
	StrategyService *services.StrategyService
	QuotaService    *services.QuotaService
	VoucherService  *services.VoucherService
	Gateway         *aigateway.Client
	MockServer      *httptest.Server
	FailServer      *httptest.Server
	quotaQuerier    condition.QuotaQuerier
}

// TestResult test result
type TestResult struct {
	TestName string
	Passed   bool
	Message  string
	Duration time.Duration
}

func main() {
	fmt.Println("=== Quota Manager Integration Tests ===")

	// Initialize test environment
	ctx, err := setupTestEnvironment()
	if err != nil {
		log.Fatalf("Failed to setup test environment: %v", err)
	}
	defer cleanupTestEnvironment(ctx)

	// Run all tests
	results := runAllTests(ctx)

	// Print test results
	printTestResults(results)
}

// runAllTests run all tests
func runAllTests(ctx *TestContext) []TestResult {
	var results []TestResult

	// Test case list - Currently testing only the first test case
	testCases := []struct {
		name string
		fn   func(*TestContext) TestResult
	}{
		{"Clear Data Test", testClearData},
		{"Condition Expression - Empty Condition Test", testEmptyCondition},
		{"Condition Expression - Match User Test", testMatchUserCondition},
		{"Condition Expression - Register Before Test", testRegisterBeforeCondition},
		{"Condition Expression - Access After Test", testAccessAfterCondition},
		{"Condition Expression - Github Star Test", testGithubStarCondition},
		{"Condition Expression - Quota LE Test", testQuotaLECondition},
		{"Condition Expression - Is VIP Test", testIsVipCondition},
		{"Condition Expression - Belong To Test", testBelongToCondition},
		{"Condition Expression - AND Nesting Test", testAndCondition},
		{"Condition Expression - OR Nesting Test", testOrCondition},
		{"Condition Expression - NOT Nesting Test", testNotCondition},
		{"Condition Expression - Complex Nesting Test", testComplexCondition},
		{"Condition Expression - AND + OR Nesting Test", testAndOrNestingCondition},
		{"Condition Expression - OR + NOT Nesting Test", testOrNotNestingCondition},
		{"Condition Expression - Three-Level Nesting Test", testThreeLevelNestingCondition},
		{"Condition Expression - Multiple Conditions Nesting Test", testMultipleConditionsNestingCondition},
		{"Condition Expression - Complex Nesting Test1", testComplexNestedConditions1},
		{"Condition Expression - Complex Nesting Test2", testComplexNestedConditions2},
		{"Condition Expression - Complex Nesting Test3", testComplexNestedConditions3},
		{"Single Recharge Strategy Test", testSingleTypeStrategy},
		{"Periodic Recharge Strategy Test", testPeriodicTypeStrategy},
		{"Strategy Status Control Test", testStrategyStatusControl},
		{"AiGateway Request Failure Test", testAiGatewayFailure},
		{"Batch User Processing Test", testBatchUserProcessing},
		{"Voucher Generation and Validation Test", testVoucherGenerationAndValidation},
		{"Quota Transfer Out Test", testQuotaTransferOut},
		{"Quota Transfer In Test", testQuotaTransferIn},
		{"Transfer In Status Cases Test", testTransferInStatusCases},
		{"Quota Expiry Test", testQuotaExpiry},
		{"Quota Audit Records Test", testQuotaAuditRecords},
		{"Strategy with Expiry Date Test", testStrategyWithExpiryDate},
		{"Multiple Operations Accuracy Test", testMultipleOperationsAccuracy},
		{"Transfer In User ID Mismatch Test", testTransferInUserIDMismatch},
		{"User Quota Consumption Order Test", testUserQuotaConsumptionOrder},
		{"Transfer Out Insufficient Available Quota Test", testTransferOutInsufficientAvailable},
		{"Transfer In Expired Quota Test", testTransferInExpiredQuota},
		{"Transfer In Invalid Voucher Test", testTransferInInvalidVoucher},
		{"Transfer In Quota Expiry Consistency Test", testTransferInQuotaExpiryConsistency},
		{"Strategy Expiry Date Coverage Test", testStrategyExpiryDateCoverage},
		{"Transfer Earliest Expiry Date Test", testTransferEarliestExpiryDate},
		{"Transfer Out Empty Receiver ID Test", testTransferOutEmptyReceiverID},
		{"Transfer Out With Expired Quota Test", testTransferOutWithExpiredQuota},
		{"Transfer In With Expired Voucher Test", testTransferInWithExpiredVoucher},
		{"Quota Expiry During Transfer Test", testQuotaExpiryDuringTransfer},
		{"Batch Quota Expiry Consistency Test", testBatchQuotaExpiryConsistency},
		{"Transfer Out Expiry Date Validation Test", testTransferOutExpiryDateValidation},
		{"Concurrent Operations Test", testConcurrentOperations},
	}

	for _, tc := range testCases {
		fmt.Printf("Running test: %s\n", tc.name)
		start := time.Now()
		result := tc.fn(ctx)
		result.Duration = time.Since(start)
		result.TestName = tc.name
		results = append(results, result)

		if result.Passed {
			fmt.Printf("✅ %s - PASSED (%.2fs)\n", tc.name, result.Duration.Seconds())
		} else {
			fmt.Printf("❌ %s - FAILED: %s (%.2fs)\n", tc.name, result.Message, result.Duration.Seconds())
		}
	}

	return results
}
