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

// runAllTests run all tests (service layer + API layer)
func runAllTests(ctx *TestContext) []TestResult {
	var results []TestResult

	// Test case list - All tests (service layer + API layer)
	testCases := []struct {
		name string
		fn   func(*TestContext) TestResult
	}{
		{"Clear Data Test", testClearData},

		// Validation tests
		{"Validation Utils Test", testValidationUtils},
		{"Page Params Validation Test", testValidatePageParams},
		{"API Validation Create Strategy Test", testAPIValidationCreateStrategy},
		{"API Validation Transfer Out Test", testAPIValidationTransferOut},
		{"API Validation User ID Test", testAPIValidationUserID},
		{"Schema Validation Test", testSchemaValidation},
		{"Transfer Request Validation Test", testTransferRequestValidation},
		{"Custom Validators Test", testCustomValidators},
		{"Error Messages Test", testErrorMessages},

		// Permission Validation Tests
		{"Permission Validation Test", testPermissionValidation},
		{"Permission Custom Validators Test", testPermissionCustomValidators},
		{"Permission Validation Error Messages Test", testPermissionValidationErrorMessages},

		// Condition Expression Tests
		{"Condition Expression - True Condition Test", testTrueCondition},
		{"Condition Expression - Empty Condition Test", testEmptyConditionProhibited},
		{"Condition Expression - False Condition Test", testFalseCondition},
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

		// Quota Tests
		{"Single Recharge Strategy Test", testSingleTypeStrategy},
		{"Periodic Recharge Strategy Test", testPeriodicTypeStrategy},
		{"Strategy Status Control Test", testStrategyStatusControl},
		{"AiGateway Request Failure Test", testAiGatewayFailure},
		{"Batch User Processing Test", testBatchUserProcessing},
		{"Voucher Generation and Validation Test", testVoucherGenerationAndValidation},
		{"Quota Expiry Test", testQuotaExpiry},
		{"Quota Audit Records Test", testQuotaAuditRecords},
		{"Strategy with Expiry Date Test", testStrategyWithExpiryDate},
		{"Multiple Operations Accuracy Test", testMultipleOperationsAccuracy},
		{"User Quota Consumption Order Test", testUserQuotaConsumptionOrder},
		{"Quota Transfer Out Test", testQuotaTransferOut},
		{"Quota Transfer In Test", testQuotaTransferIn},
		{"Quota Expiry During Transfer Test", testQuotaExpiryDuringTransfer},
		{"Batch Quota Expiry Consistency Test", testBatchQuotaExpiryConsistency},

		// Transfer Tests
		{"Transfer In Status Cases Test", testTransferInStatusCases},
		{"Transfer In User ID Mismatch Test", testTransferInUserIDMismatch},
		{"Transfer Out Insufficient Available Quota Test", testTransferOutInsufficientAvailable},
		{"Transfer In Expired Quota Test", testTransferInExpiredQuota},
		{"Transfer In Invalid Voucher Test", testTransferInInvalidVoucher},
		{"Transfer In Quota Expiry Consistency Test", testTransferInQuotaExpiryConsistency},
		{"Transfer In GitHub Star Not Set Test", testTransferInGithubStarNotSet},
		{"Transfer In GitHub Star Set Test", testTransferInGithubStarSet},
		{"Transfer In GitHub Star Empty Field Test", testTransferInGithubStarEmptyField},
		{"Transfer Out GitHub Star Not Set Test", testTransferOutGithubStarNotSet},
		{"Transfer Out GitHub Star Set Test", testTransferOutGithubStarSet},
		{"Strategy Expiry Date Coverage Test", testStrategyExpiryDateCoverage},
		{"Transfer Earliest Expiry Date Test", testTransferEarliestExpiryDate},
		{"Transfer Out Empty Receiver ID Test", testTransferOutEmptyReceiverID},
		{"Transfer Out With Expired Quota Test", testTransferOutWithExpiredQuota},
		{"Transfer In With Expired Voucher Test", testTransferInWithExpiredVoucher},
		{"Transfer Out Expiry Date Validation Test", testTransferOutExpiryDateValidation},

		// Periodic Strategy Tests
		{"Periodic Strategy Execution Test", testPeriodicStrategyExecution},
		{"Periodic Cron Registration Test", testPeriodicStrategyCronRegistration},
		{"Periodic Cron Expression Validation Test", testPeriodicCronExpressionValidation},
		{"Periodic Strategy CRUD Operations Test", testPeriodicStrategyCRUDOperations},
		{"Periodic Strategy Field Modifications Test", testPeriodicStrategyFieldModifications},
		{"Periodic Strategy Invalid Cron Expressions Test", testPeriodicStrategyInvalidCronExpressions},
		{"Periodic Strategy Concurrent Modifications Test", testPeriodicStrategyConcurrentModifications},
		{"Periodic Strategy Edge Cases Test", testPeriodicStrategyEdgeCases},

		// API layer tests
		{"API Health Check", testAPIHealthCheck},
		{"API Create Strategy", testAPICreateStrategy},
		{"API Create Strategy Invalid Data", testAPICreateStrategyInvalidData},
		{"API Create Strategy Invalid Condition", testAPICreateStrategyInvalidCondition},
		{"API Get Strategy Not Found", testAPIGetStrategyNotFound},
		{"API Invalid Strategy ID", testAPIInvalidStrategyID},
		{"API Get Strategies", testAPIGetStrategies},
		{"API Quota Unauthorized", testAPIQuotaUnauthorized},

		// Sanity Tests
		{"Concurrent Operations Test", testConcurrentOperations},

		// Permission Management Tests
		{"User Whitelist Management Test", testUserWhitelistManagement},
		{"Department Whitelist Management Test", testDepartmentWhitelistManagement},
		{"Permission Priority and Inheritance Test", testPermissionPriorityAndInheritance},
		{"Aigateway Permission Sync Test", testAigatewayPermissionSync},
		{"Sync Without Whitelist Test", testSyncWithoutWhitelist},
		{"Aigateway Notification Optimization Test", testAigatewayNotificationOptimization},
		{"User Whitelist Distribution Test", testUserWhitelistDistribution},
		{"Department Whitelist Distribution Test", testDepartmentWhitelistDistribution},
		{"Permission Hierarchy Level 1 Test", testPermissionHierarchyLevel1},
		{"Permission Hierarchy Level 2 Test", testPermissionHierarchyLevel2},
		{"Permission Hierarchy Level 3 Test", testPermissionHierarchyLevel3},
		{"Permission Hierarchy Level 5 Test", testPermissionHierarchyLevel5},
		{"User Overrides Department Test", testUserOverridesDepartment},
		{"Department Whitelist Change Test", testDepartmentWhitelistChange},
		{"User Whitelist Change Test", testUserWhitelistChange},
		{"User Department Change Test", testUserDepartmentChange},
		{"User Addition and Removal Test", testUserAdditionAndRemoval},
		{"Non-existent User and Department Test", testNonExistentUserAndDepartment},
		{"Employee Data Integrity Test", testEmployeeDataIntegrity},
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
