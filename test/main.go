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
	MockQuotaStore  *MockQuotaStore
}

// TestResult test result
type TestResult struct {
	TestName  string
	Passed    bool
	Message   string
	Duration  time.Duration
	StartTime time.Time
	EndTime   time.Time
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
		{"Condition Expression - Match User Multiple IDs Test", testMatchUserMultipleIds},
		{"Condition Expression - Register Before Test", testRegisterBeforeCondition},
		{"Condition Expression - Access After Test", testAccessAfterCondition},
		{"Condition Expression - Github Star Test", testGithubStarCondition},
		{"Condition Expression - Quota LE Test", testQuotaLECondition},
		{"Condition Expression - Is VIP Test", testIsVipCondition},
		{"Condition Expression - Belong To Test", testBelongToCondition},
		{"Condition Expression - Belong To Employee Sync Test", testBelongToWithEmployeeSync},
		{"Condition Expression - Belong To Fallback Test", testBelongToFallbackToOriginal},
		{"Condition Expression - Belong To No Employee Number Test", testBelongToWithNoEmployeeNumber},
		{"Condition Expression - Belong To Multiple Organizations Test", testBelongToMultipleOrgs},
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

		// Quota Expiry Scheduler Tests
		{"Single User Quota Expiry Test", testExpireQuotasTaskBasic},
		{"Just Expired Quota Test", testExpireQuotasTaskJustExpired},
		{"Multiple Users Quota Expiry Test", testExpireQuotasTaskMultiple},
		{"Empty Dataset Test", testExpireQuotasTaskEmpty},
		{"AiGateway Communication Failure Test", testExpireQuotasTaskAiGatewayFail},
		{"Idempotency Test", testExpireQuotasTaskIdempotency},
		{"Month End Batch Expiry Test", testExpireQuotasTask_MonthEndBatchExpiry},
		{"Month End Day Differences Test", testExpireQuotasTask_MonthDayDifferences},
		{"Mixed Status Quotas Test", testExpireQuotasTask_MixedStatusQuotas},
		{"Expired Quota Greater Than Used Quota Test", testExpireQuotasTask_ExpiredQuotaGreaterThanUsedQuota},
		{"Expired Quota Less Than Used Quota Test", testExpireQuotasTask_ExpiredQuotaLessThanUsedQuota},
		{"Mixed Consumption and Expiry Scenarios Test", testExpireQuotasTask_MixedConsumptionAndExpiry},

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
		{"Empty Whitelist Fallback Test", testEmptyWhitelistFallback},
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

		// Star Check Permission Management Tests
		{"User Star Check Setting Management Test", testUserStarCheckSettingManagement},
		{"Department Star Check Setting Management Test", testDepartmentStarCheckSettingManagement},
		{"Star Check Setting Priority and Inheritance Test", testStarCheckSettingPriorityAndInheritance},
		{"Empty Star Check Setting Fallback Test", testEmptyStarCheckSettingFallback},
		{"Star Check Permission Distribution Test", testStarCheckPermissionDistribution},
		{"Sync Without Star Check Setting Test", testSyncWithoutStarCheckSetting},
		{"Star Check Notification Optimization Test", testStarCheckNotificationOptimization},
		{"User Star Check Setting Distribution Test", testUserStarCheckSettingDistribution},
		{"Department Star Check Setting Distribution Test", testDepartmentStarCheckSettingDistribution},
		{"Star Check Setting Hierarchy Level 1 Test", testStarCheckSettingHierarchyLevel1},
		{"Star Check Setting Hierarchy Level 2 Test", testStarCheckSettingHierarchyLevel2},
		{"Star Check Setting Hierarchy Level 3 Test", testStarCheckSettingHierarchyLevel3},
		{"Star Check Setting Hierarchy Level 5 Test", testStarCheckSettingHierarchyLevel5},
		{"User Star Check Setting Overrides Department Test", testUserStarCheckSettingOverridesDepartment},
		{"Department Star Check Setting Change Test", testDepartmentStarCheckSettingChange},
		{"User Star Check Setting Change Test", testUserStarCheckSettingChange},
		{"User Department Star Check Change Test", testUserDepartmentStarCheckChange},
		{"User Star Check Addition and Removal Test", testUserStarCheckAdditionAndRemoval},
		{"Non-existent User and Department Star Check Test", testNonExistentUserAndDepartmentStarCheck},
		{"Star Check Employee Data Integrity Test", testStarCheckEmployeeDataIntegrity},
		{"Unified Permission Queries Test", testUnifiedPermissionQueries},
		{"Star Check Employee Sync Test", testStarCheckEmployeeSync},
		{"GitHub Star Check Disabled Test", testGithubStarCheckDisabled},
		{"GitHub Star Check Enabled User Not Star Test", testGithubStarCheckEnabledUserNotStar},
		{"GitHub Star Check Enabled User Starred Test", testGithubStarCheckEnabledUserStarred},
		{"GitHub Star Check Enabled User Starred Other Test", testGithubStarCheckEnabledUserStarredOther},

		// Quota Check Permission Management Tests
		{"User Quota Check Setting Management Test", testUserQuotaCheckSettingManagement},
		{"Department Quota Check Setting Management Test", testDepartmentQuotaCheckSettingManagement},
		{"Quota Check Setting Priority and Inheritance Test", testQuotaCheckSettingPriorityAndInheritance},
		{"Empty Quota Check Setting Fallback Test", testEmptyQuotaCheckSettingFallback},
		{"Quota Check Permission Distribution Test", testQuotaCheckPermissionDistribution},
		{"Sync Without Quota Check Setting Test", testSyncWithoutQuotaCheckSetting},
		{"Quota Check Notification Optimization Test", testQuotaCheckNotificationOptimization},
		{"User Quota Check Setting Distribution Test", testUserQuotaCheckSettingDistribution},
		{"Department Quota Check Setting Distribution Test", testDepartmentQuotaCheckSettingDistribution},
		{"Quota Check Setting Hierarchy Level 1 Test", testQuotaCheckSettingHierarchyLevel1},
		{"Quota Check Setting Hierarchy Level 2 Test", testQuotaCheckSettingHierarchyLevel2},
		{"Quota Check Setting Hierarchy Level 3 Test", testQuotaCheckSettingHierarchyLevel3},
		{"Quota Check Setting Hierarchy Level 5 Test", testQuotaCheckSettingHierarchyLevel5},
		{"User Quota Check Setting Overrides Department Test", testUserQuotaCheckSettingOverridesDepartment},
		{"Department Quota Check Setting Change Test", testDepartmentQuotaCheckSettingChange},
		{"User Quota Check Setting Change Test", testUserQuotaCheckSettingChange},
		{"User Department Quota Check Change Test", testUserDepartmentQuotaCheckChange},
		{"User Quota Check Addition and Removal Test", testUserQuotaCheckAdditionAndRemoval},
		{"Non-existent User and Department Quota Check Test", testNonExistentUserAndDepartmentQuotaCheck},
		{"Quota Check Employee Data Integrity Test", testQuotaCheckEmployeeDataIntegrity},
		{"Quota Check Employee Sync Test", testQuotaCheckEmployeeSync},
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
