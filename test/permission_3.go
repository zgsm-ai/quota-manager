package main

import (
	"fmt"
	"quota-manager/internal/config"
	"quota-manager/internal/models"
	"quota-manager/internal/services"
	"time"

	"github.com/google/uuid"

	"gorm.io/gorm"
)

// testUserDepartmentChangeScenario tests a specific department change scenario
func testUserDepartmentChangeScenario(ctx *TestContext, permissionService *services.PermissionService, employeeSyncService *services.EmployeeSyncService, scenario struct {
	name                string
	employeeNumber      string
	originalDept        string
	targetDept          string
	originalWhitelist   []string
	targetWhitelist     []string
	expectPersonalClear bool
	expectedModels      []string
	description         string
}) TestResult {

	// 0. Cleanup: Clear all data to ensure test isolation between scenarios
	if err := clearPermissionData(ctx); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to clear permission data: %v", err)}
	}

	// 1. Setup: Create test employee in original department
	employee := &models.EmployeeDepartment{
		EmployeeNumber:     scenario.employeeNumber,
		Username:           fmt.Sprintf("test_employee_%s", scenario.employeeNumber),
		DeptFullLevelNames: fmt.Sprintf("Tech_Group,R&D_Center,%s,%s_Team1", scenario.originalDept, scenario.originalDept),
	}
	if err := ctx.DB.DB.Create(employee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
	}

	// 2. Setup: Configure original department whitelist (if specified)
	if scenario.originalWhitelist != nil {
		// Check if whitelist already exists
		var existingWhitelist models.ModelWhitelist
		err := ctx.DB.DB.Where("target_type = ? AND target_identifier = ?", "department", scenario.originalDept).First(&existingWhitelist).Error
		if err != nil {
			// Doesn't exist, create it
			if err := permissionService.SetDepartmentWhitelist(scenario.originalDept, scenario.originalWhitelist); err != nil {
				return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set original department whitelist: %v", err)}
			}
		}
	}

	// 3. Setup: Configure target department whitelist (if specified and different from original)
	if scenario.targetWhitelist != nil && scenario.targetDept != scenario.originalDept {
		// Create temporary employee in target department if needed to satisfy department existence check
		var tempEmployee *models.EmployeeDepartment
		var targetEmployeeCount int64
		ctx.DB.DB.Model(&models.EmployeeDepartment{}).Where("dept_full_level_names LIKE ?", "%"+scenario.targetDept+"%").Count(&targetEmployeeCount)

		if targetEmployeeCount == 0 {
			tempEmployee = &models.EmployeeDepartment{
				EmployeeNumber:     fmt.Sprintf("temp_%s", scenario.employeeNumber),
				Username:           "temp_employee",
				DeptFullLevelNames: fmt.Sprintf("Tech_Group,R&D_Center,%s,%s_Team1", scenario.targetDept, scenario.targetDept),
			}
			if err := ctx.DB.DB.Create(tempEmployee).Error; err != nil {
				return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create temporary employee: %v", err)}
			}
		}

		// Check if target whitelist already exists with same models
		var existingWhitelist models.ModelWhitelist
		err := ctx.DB.DB.Where("target_type = ? AND target_identifier = ?", "department", scenario.targetDept).First(&existingWhitelist).Error
		if err != nil || !slicesEqual(existingWhitelist.GetAllowedModelsAsSlice(), scenario.targetWhitelist) {
			if err := permissionService.SetDepartmentWhitelist(scenario.targetDept, scenario.targetWhitelist); err != nil {
				return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set target department whitelist: %v", err)}
			}
		}

		// Clean up temporary employee
		if tempEmployee != nil {
			if err := ctx.DB.DB.Delete(tempEmployee).Error; err != nil {
				return TestResult{Passed: false, Message: fmt.Sprintf("Failed to delete temporary employee: %v", err)}
			}
		}
	}

	// 4. Setup: Add personal whitelist for user (to test clearing functionality)
	if scenario.expectPersonalClear {
		userPersonalModels := []string{"text-davinci-003", "claude-2"}
		if err := permissionService.SetUserWhitelist(scenario.employeeNumber, userPersonalModels); err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set user personal whitelist: %v", err)}
		}
	}

	// 5. Action: Trigger department change via HR sync
	if scenario.targetDept != scenario.originalDept {
		// Add updated employee data to mock HR system using new data structure
		var deptID int
		switch fmt.Sprintf("%s_Team1", scenario.targetDept) {
		case "UX_Dept_Team1":
			deptID = 4
		case "QA_Dept_Team1":
			deptID = 6
		case "Testing_Dept_Team1":
			deptID = 8
		case "Operations_Dept_Team1":
			deptID = 10
		default:
			deptID = 4 // Default to UX_Dept_Team1
		}

		AddMockEmployee(
			scenario.employeeNumber,
			fmt.Sprintf("test_employee_%s", scenario.employeeNumber),
			fmt.Sprintf("test_%s@example.com", scenario.employeeNumber),
			fmt.Sprintf("13800%s", scenario.employeeNumber),
			deptID,
		)
	}

	// Clear permission calls BEFORE the action phase to isolate the test
	// This ensures we only count calls from the employee sync operation
	mockStore.ClearPermissionCalls()

	// Trigger employee sync
	if scenario.targetDept != scenario.originalDept {
		fmt.Printf("Triggering employee sync for department change...\n")
		if err := employeeSyncService.SyncEmployees(); err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Employee sync failed: %v", err)}
		}
	} else {
		// No department change, just update permissions to see current state
		if err := permissionService.UpdateEmployeePermissions(scenario.employeeNumber); err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Failed to update employee permissions: %v", err)}
		}
	}

	// 6. Validation: Check aigateway calls
	permissionCalls := mockStore.GetPermissionCalls()

	// Determine expected call count based on scenario specifics
	var expectedCallCount int
	if scenario.targetDept == scenario.originalDept {
		// No department change scenarios
		if len(scenario.expectedModels) == 0 {
			// User has no permissions and no department change - no call expected
			expectedCallCount = 0
		} else {
			// User has permissions but no department change - call expected for permission update
			expectedCallCount = 1
		}
	} else {
		// Department change scenarios - always expect 1 call for permission sync
		expectedCallCount = 1
	}

	if len(permissionCalls) != expectedCallCount {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected %d permission calls, got %d", expectedCallCount, len(permissionCalls))}
	}

	// Validate the aigateway call details (only if calls are expected)
	if expectedCallCount > 0 {
		call := permissionCalls[0]
		if call.EmployeeNumber != scenario.employeeNumber {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected employee number %s in aigateway call, got %s", scenario.employeeNumber, call.EmployeeNumber)}
		}

		if call.Operation != "set" {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected 'set' operation in aigateway call, got '%s'", call.Operation)}
		}

		if len(call.Models) != len(scenario.expectedModels) {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected %d models in aigateway call, got %d", len(scenario.expectedModels), len(call.Models))}
		}

		// Verify aigateway call models match expected (including empty list)
		expectedModelsMap := make(map[string]bool)
		for _, model := range scenario.expectedModels {
			expectedModelsMap[model] = true
		}
		for _, model := range call.Models {
			if !expectedModelsMap[model] {
				return TestResult{Passed: false, Message: fmt.Sprintf("Unexpected model %s in aigateway call", model)}
			}
		}
	}

	// 7. Validation: Check effective permissions via service
	effectiveModels, err := permissionService.GetUserEffectivePermissions(scenario.employeeNumber)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective permissions: %v", err)}
	}

	if len(effectiveModels) != len(scenario.expectedModels) {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected %d effective models, got %d", len(scenario.expectedModels), len(effectiveModels))}
	}

	// Verify effective models match expected
	effectiveModelsMap := make(map[string]bool)
	for _, model := range effectiveModels {
		effectiveModelsMap[model] = true
	}
	for _, expectedModel := range scenario.expectedModels {
		if !effectiveModelsMap[expectedModel] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected model %s not found in effective permissions", expectedModel)}
		}
	}

	// 8. Validation: Check personal whitelist clearing
	if scenario.expectPersonalClear && scenario.targetDept != scenario.originalDept {
		var userWhitelistAfter models.ModelWhitelist
		userWhitelistErr := ctx.DB.DB.Where("target_type = ? AND target_identifier = ?", "user", scenario.employeeNumber).First(&userWhitelistAfter).Error
		if userWhitelistErr == nil {
			return TestResult{Passed: false, Message: "User personal whitelist should be removed after department change, but it still exists"}
		}

		if userWhitelistErr != gorm.ErrRecordNotFound {
			return TestResult{Passed: false, Message: fmt.Sprintf("Unexpected database error when checking user whitelist: %v", userWhitelistErr)}
		}

		fmt.Printf("✅ User personal whitelist correctly cleared after department change\n")
	}

	// 9. Validation: Check database consistency
	if scenario.targetDept != scenario.originalDept {
		var updatedEmployee models.EmployeeDepartment
		if err := ctx.DB.DB.Where("employee_number = ?", scenario.employeeNumber).First(&updatedEmployee).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Failed to query updated employee: %v", err)}
		}

		expectedDeptPath := fmt.Sprintf("Tech_Group,R&D_Center,%s,%s_Team1", scenario.targetDept, scenario.targetDept)
		if updatedEmployee.DeptFullLevelNames != expectedDeptPath {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected department path '%s', got '%s'", expectedDeptPath, updatedEmployee.DeptFullLevelNames)}
		}
	}

	// Clean up for next scenario - remove employee from mock HR if added
	if scenario.targetDept != scenario.originalDept {
		RemoveMockEmployeeByNumber(scenario.employeeNumber)
	}

	return TestResult{Passed: true, Message: fmt.Sprintf("Scenario %s completed successfully", scenario.name)}
}

// testUserAdditionAndRemoval tests user addition and removal via employee sync
func testUserAdditionAndRemoval(ctx *TestContext) TestResult {
	// Create services
	aiGatewayConfig := &config.AiGatewayConfig{
		Host:       "localhost",
		Port:       8080,
		AdminPath:  "/model-permission",
		AuthHeader: "x-admin-key",
		AuthValue:  "test-key",
	}

	// Default employee sync config for compatibility
	defaultEmployeeSyncConfig := &config.EmployeeSyncConfig{
		Enabled: false, // Default to disabled for existing tests
		HrURL:   "http://localhost:8099/api/hr/employees",
		HrKey:   "test-hr-key",
		DeptURL: "http://localhost:8099/api/hr/departments",
		DeptKey: "test-dept-key",
	}

	permissionService := services.NewPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)
	starCheckPermissionService := services.NewStarCheckPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

	// Create a temporary employee in the target department so we can set its whitelist
	tempEmployee := &models.EmployeeDepartment{
		EmployeeNumber:     "000061",
		Username:           "temp_ux_employee",
		DeptFullLevelNames: "Tech_Group,R&D_Center,UX_Dept,UX_Dept_Team1",
	}
	if err := ctx.DB.DB.Create(tempEmployee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create temporary employee for department: %v", err)}
	}

	// Set department whitelist
	deptModels := []string{"gpt-3.5-turbo", "deepseek-v3"}
	if err := permissionService.SetDepartmentWhitelist("UX_Dept", deptModels); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set department whitelist: %v", err)}
	}

	// Remove the temporary employee
	if err := ctx.DB.DB.Delete(tempEmployee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to delete temporary employee: %v", err)}
	}

	// === Create employee sync service ===
	employeeSyncConfig := &config.EmployeeSyncConfig{
		Enabled: true,
		HrURL:   ctx.MockServer.URL + "/api/test/employees",
		HrKey:   "TEST_EMP_KEY_32_BYTES_1234567890",
		DeptURL: ctx.MockServer.URL + "/api/test/departments",
		DeptKey: "TEST_DEPT_KEY_32_BYTES_123456789",
	}
	quotaCheckPermissionService := services.NewQuotaCheckPermissionService(ctx.DB, aiGatewayConfig, employeeSyncConfig, ctx.Gateway)
	employeeSyncService := services.NewEmployeeSyncService(ctx.DB, config.NewManager(&config.Config{EmployeeSync: *employeeSyncConfig}), permissionService, starCheckPermissionService, quotaCheckPermissionService)

	// Add department hierarchy data to mock HR system using new structure
	SetupDefaultDepartmentHierarchy()

	// === Test 1: Simulate employee addition ===
	// Add new employee to mock HR data (simulating HR system detecting new hire)
	AddMockEmployee("000060", "new_test_employee", "new_test@example.com", "13800000060", 4) // UX_Dept_Team1

	// Clear permission calls
	mockStore.ClearPermissionCalls()

	// Trigger new employee addition via employee sync service
	fmt.Printf("Triggering employee sync for user addition (simulating timer)...\n")
	if err := employeeSyncService.SyncEmployees(); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Employee sync for addition failed: %v", err)}
	}
	fmt.Printf("Employee sync for addition completed successfully\n")

	// === Part 1: Verify aigateway and database state after new user addition ===

	// 1. Verify aigateway calls (new user should have 1 call)
	permissionCalls := mockStore.GetPermissionCalls()
	if len(permissionCalls) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 permission call for new user, got %d", len(permissionCalls))}
	}

	call := permissionCalls[0]
	if call.EmployeeNumber != "000060" {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected employee number 000060 in aigateway call, got %s", call.EmployeeNumber)}
	}

	if call.Operation != "set" {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 'set' operation in aigateway call, got '%s'", call.Operation)}
	}

	if len(call.Models) != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 models in aigateway call for new user, got %d", len(call.Models))}
	}

	// Verify aigateway call model content (should be department models)
	expectedDeptModels := map[string]bool{"gpt-3.5-turbo": true, "deepseek-v3": true}
	for _, model := range call.Models {
		if !expectedDeptModels[model] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Unexpected model %s in aigateway call for new user", model)}
		}
	}

	// 2. Verify employee record exists in database
	var newEmployeeRecord models.EmployeeDepartment
	if err := ctx.DB.DB.Where("employee_number = ?", "000060").First(&newEmployeeRecord).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to find new employee in database: %v", err)}
	}

	if newEmployeeRecord.Username != "new_test_employee" {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected username 'new_test_employee', got '%s'", newEmployeeRecord.Username)}
	}

	// 3. Verify effective permission record in database
	var dbEffectiveRecord models.EffectivePermission
	if err := ctx.DB.DB.Where("employee_number = ?", "000060").First(&dbEffectiveRecord).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to query effective permissions for new user from database: %v", err)}
	}

	dbEffectiveModels := dbEffectiveRecord.GetEffectiveModelsAsSlice()
	if len(dbEffectiveModels) != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 models in database effective permissions for new user, got %d", len(dbEffectiveModels))}
	}

	dbEffectiveModelsMap := make(map[string]bool)
	for _, model := range dbEffectiveModels {
		dbEffectiveModelsMap[model] = true
	}
	for _, expectedModel := range deptModels {
		if !dbEffectiveModelsMap[expectedModel] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected model %s not found in database effective permissions for new user", expectedModel)}
		}
	}

	// 4. Verify effective permissions via service
	effectiveModels, err := permissionService.GetUserEffectivePermissions("000060")
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective permissions for new user via service: %v", err)}
	}

	if len(effectiveModels) != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 effective models for new user via service, got %d", len(effectiveModels))}
	}

	// Verify service returned model content is correct
	serviceModelsMap := make(map[string]bool)
	for _, model := range effectiveModels {
		serviceModelsMap[model] = true
	}
	for _, expectedModel := range deptModels {
		if !serviceModelsMap[expectedModel] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected model %s not found in service effective permissions for new user", expectedModel)}
		}
	}

	// === Test 2: Simulate employee removal ===
	// Remove employee from mock HR data (simulating HR system detecting employee departure)
	RemoveMockEmployeeByNumber("000060")

	// Clear permission calls
	mockStore.ClearPermissionCalls()

	// Trigger employee removal via employee sync service
	fmt.Printf("Triggering employee sync for user removal (simulating timer)...\n")
	if err := employeeSyncService.SyncEmployees(); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Employee sync for removal failed: %v", err)}
	}
	fmt.Printf("Employee sync for removal completed successfully\n")

	// === Part 2: Verify database state after user removal ===

	// 1. Verify employee record has been deleted from database
	var deletedEmployeeCount int64
	if err := ctx.DB.DB.Model(&models.EmployeeDepartment{}).Where("employee_number = ?", "000060").Count(&deletedEmployeeCount).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to count deleted employee records: %v", err)}
	}

	if deletedEmployeeCount > 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected no employee records for deleted user, got %d", deletedEmployeeCount)}
	}

	// 2. Verify effective permission records have been cleaned from database
	var effectivePermCount int64
	if err := ctx.DB.DB.Model(&models.EffectivePermission{}).Where("employee_number = ?", "000060").Count(&effectivePermCount).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to count effective permissions for removed user: %v", err)}
	}

	if effectivePermCount > 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected no effective permissions for removed user, got %d", effectivePermCount)}
	}

	// 3. Verify user whitelist records should also be cleaned (if any)
	var userWhitelistCount int64
	if err := ctx.DB.DB.Model(&models.ModelWhitelist{}).Where("target_type = ? AND target_identifier = ?", "user", "000060").Count(&userWhitelistCount).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to count user whitelist records for removed user: %v", err)}
	}

	if userWhitelistCount > 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected no user whitelist records for removed user, got %d", userWhitelistCount)}
	}

	// 4. Verify service returns error when getting permissions for deleted user
	deletedUserModels, err := permissionService.GetUserEffectivePermissions("000060")
	if err == nil {
		return TestResult{Passed: false, Message: "Expected error when getting permissions for deleted user, but got none"}
	}
	_ = deletedUserModels

	// 5. Verify database integrity: ensure other employee data is not affected
	// Check if department whitelist record still exists
	var deptWhitelistCount int64
	if err := ctx.DB.DB.Model(&models.ModelWhitelist{}).Where("target_type = ? AND target_identifier = ?", "department", "UX_Dept").Count(&deptWhitelistCount).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to count department whitelist records: %v", err)}
	}

	if deptWhitelistCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 department whitelist record to remain, got %d", deptWhitelistCount)}
	}

	// === Clean up mock HR data ===
	// Clear all mock data to prepare for next test
	ClearMockData()

	return TestResult{Passed: true, Message: "User addition and removal test via employee sync with comprehensive verification succeeded"}
}

// testEmployeeDataIntegrity tests employee data integrity in PostgreSQL
func testEmployeeDataIntegrity(ctx *TestContext) TestResult {
	// Create services
	aiGatewayConfig := &config.AiGatewayConfig{
		Host:       "localhost",
		Port:       8080,
		AdminPath:  "/model-permission",
		AuthHeader: "x-admin-key",
		AuthValue:  "test-key",
	}

	employeeSyncConfig := &config.EmployeeSyncConfig{
		Enabled: true,
		HrURL:   ctx.MockServer.URL + "/api/test/employees",
		HrKey:   "TEST_EMP_KEY_32_BYTES_1234567890",
		DeptURL: ctx.MockServer.URL + "/api/test/departments",
		DeptKey: "TEST_DEPT_KEY_32_BYTES_123456789",
	}

	// Use the existing employee sync config for permission service
	permissionService := services.NewPermissionService(ctx.DB, aiGatewayConfig, employeeSyncConfig, ctx.Gateway)
	starCheckPermissionService := services.NewStarCheckPermissionService(ctx.DB, aiGatewayConfig, employeeSyncConfig, ctx.Gateway)
	quotaCheckPermissionService := services.NewQuotaCheckPermissionService(ctx.DB, aiGatewayConfig, employeeSyncConfig, ctx.Gateway)
	employeeSyncService := services.NewEmployeeSyncService(ctx.DB, config.NewManager(&config.Config{EmployeeSync: *employeeSyncConfig}), permissionService, starCheckPermissionService, quotaCheckPermissionService)

	// Setup department hierarchy in Mock HR for our test using new structure
	ClearMockData()                                // Clear any existing data first
	AddMockDepartment(1, 0, "Tech_Group", 1, 1)    // Root department
	AddMockDepartment(2, 1, "AI_Lab", 2, 1)        // Second level
	AddMockDepartment(3, 2, "ML_Dept", 3, 1)       // Third level - ML
	AddMockDepartment(4, 3, "ML_Dept_Team1", 4, 1) // Fourth level - ML Team
	AddMockDepartment(5, 2, "DL_Dept", 3, 1)       // Third level - DL
	AddMockDepartment(6, 5, "DL_Dept_Team1", 4, 1) // Fourth level - DL Team

	// Create initial employee in database
	employee := &models.EmployeeDepartment{
		EmployeeNumber:     "000070",
		Username:           "data_integrity_test_employee",
		DeptFullLevelNames: "Tech_Group,AI_Lab,ML_Dept,ML_Dept_Team1",
	}
	if err := ctx.DB.DB.Create(employee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create initial employee: %v", err)}
	}

	// Add initial employee data to Mock HR system to ensure sync consistency
	AddMockEmployee("000070", "data_integrity_test_employee", "test_070@example.com", "13800000070", 4) // ML_Dept_Team1

	// Set initial permissions
	// When employee sync is enabled, service will validate employee existence.
	// We have created the employee record above, but to avoid dependency issues, temporarily disable validation.
	prevEnabled := employeeSyncConfig.Enabled
	employeeSyncConfig.Enabled = false
	if err := permissionService.SetUserWhitelist("000070", []string{"gpt-4", "claude-3-opus"}); err != nil {
		// Restore flag before returning
		employeeSyncConfig.Enabled = prevEnabled
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set initial user whitelist: %v", err)}
	}
	// Restore employee sync flag
	employeeSyncConfig.Enabled = prevEnabled

	// Create temporary employee in target department to enable department whitelist setting
	tempEmployee := &models.EmployeeDepartment{
		EmployeeNumber:     "temp_000070",
		Username:           "temp_employee",
		DeptFullLevelNames: "Tech_Group,AI_Lab,DL_Dept,DL_Dept_Team1",
	}
	if err := ctx.DB.DB.Create(tempEmployee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create temporary employee: %v", err)}
	}

	// Set department whitelist for target department to ensure user has permissions after department change
	if err := permissionService.SetDepartmentWhitelist("DL_Dept", []string{"gpt-4", "claude-3-opus"}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set department whitelist: %v", err)}
	}

	// Clean up temporary employee
	if err := ctx.DB.DB.Delete(tempEmployee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to delete temporary employee: %v", err)}
	}

	// Simulate employee data changes by updating Mock HR system first
	// Update the employee data in Mock HR to simulate real HR system changes
	RemoveMockEmployeeByNumber("000070")                                                                                // Remove old employee data
	AddMockEmployee("000070", "data_integrity_test_employee_updated", "test_070_updated@example.com", "13800000070", 6) // DL_Dept_Team1 (department change)

	// Clear any previous aigateway calls from setup
	mockStore.ClearPermissionCalls()

	// Trigger HR sync to process the employee data changes
	if err := employeeSyncService.SyncEmployees(); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Employee sync failed: %v", err)}
	}

	// Also trigger permission recalculation to test data consistency
	if err := permissionService.UpdateEmployeePermissions("000070"); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to update employee permissions: %v", err)}
	}

	// 1. Verify employee basic information is correctly updated in database
	var updatedEmployee models.EmployeeDepartment
	if err := ctx.DB.DB.Where("employee_number = ?", "000070").First(&updatedEmployee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to find updated employee: %v", err)}
	}

	if updatedEmployee.Username != "data_integrity_test_employee_updated" {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected updated username 'data_integrity_test_employee_updated', got '%s'", updatedEmployee.Username)}
	}

	if updatedEmployee.EmployeeNumber != "000070" {
		return TestResult{Passed: false, Message: fmt.Sprintf("Employee number should remain unchanged, expected '000070', got '%s'", updatedEmployee.EmployeeNumber)}
	}

	// Verify department hierarchy information
	deptLevels := updatedEmployee.GetDeptFullLevelNamesAsSlice()
	expectedDeptLevels := []string{"Tech_Group", "AI_Lab", "DL_Dept", "DL_Dept_Team1"}
	if len(deptLevels) != len(expectedDeptLevels) {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected %d department levels, got %d", len(expectedDeptLevels), len(deptLevels))}
	}

	for i, expectedLevel := range expectedDeptLevels {
		if deptLevels[i] != expectedLevel {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected department level '%s' at position %d, got '%s'", expectedLevel, i, deptLevels[i])}
		}
	}

	// 2. Verify user personal whitelist is cleared (due to department change)
	var userWhitelistRecord models.ModelWhitelist
	userWhitelistErr := ctx.DB.DB.Where("target_type = ? AND target_identifier = ?", "user", "000070").First(&userWhitelistRecord).Error
	if userWhitelistErr == nil {
		return TestResult{Passed: false, Message: "User personal whitelist should be cleared after department change, but it still exists"}
	}
	if userWhitelistErr != gorm.ErrRecordNotFound {
		return TestResult{Passed: false, Message: fmt.Sprintf("Unexpected database error when checking user whitelist: %v", userWhitelistErr)}
	}

	// Verify department whitelist exists for target department
	var deptWhitelistRecord models.ModelWhitelist
	if err := ctx.DB.DB.Where("target_type = ? AND target_identifier = ?", "department", "DL_Dept").First(&deptWhitelistRecord).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to find department whitelist record: %v", err)}
	}

	deptWhitelistModels := deptWhitelistRecord.GetAllowedModelsAsSlice()
	expectedModels := []string{"gpt-4", "claude-3-opus"}
	if len(deptWhitelistModels) != len(expectedModels) {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected %d models in department whitelist, got %d", len(expectedModels), len(deptWhitelistModels))}
	}

	deptModelsMap := make(map[string]bool)
	for _, model := range deptWhitelistModels {
		deptModelsMap[model] = true
	}
	for _, expectedModel := range expectedModels {
		if !deptModelsMap[expectedModel] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected model %s not found in department whitelist", expectedModel)}
		}
	}

	// 3. Verify effective permission records are correctly maintained in database
	var effectivePermRecord models.EffectivePermission
	if err := ctx.DB.DB.Where("employee_number = ?", "000070").First(&effectivePermRecord).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to find effective permissions record after employee update: %v", err)}
	}

	dbEffectiveModels := effectivePermRecord.GetEffectiveModelsAsSlice()
	if len(dbEffectiveModels) != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 models in database effective permissions after update, got %d", len(dbEffectiveModels))}
	}

	dbEffectiveModelsMap := make(map[string]bool)
	for _, model := range dbEffectiveModels {
		dbEffectiveModelsMap[model] = true
	}
	for _, expectedModel := range expectedModels {
		if !dbEffectiveModelsMap[expectedModel] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected model %s not found in database effective permissions after employee update", expectedModel)}
		}
	}

	// 4. Verify effective permissions from service match database
	// Under employee_sync enabled, service expects user_id; create auth user and query by ID
	authUser := &models.UserInfo{
		ID:             uuid.NewString(),
		CreatedAt:      time.Now().Add(-time.Hour),
		UpdatedAt:      time.Now(),
		AccessTime:     time.Now(),
		Name:           "data_integrity_test_employee_updated",
		EmployeeNumber: "000070",
		GithubID:       fmt.Sprintf("test_%s_%d", "000070", time.Now().UnixNano()),
		GithubName:     "data_integrity_test_employee_updated",
		Devices:        "{}",
	}
	if err := ctx.DB.AuthDB.Create(authUser).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create auth user for data integrity test: %v", err)}
	}

	effectiveModels, err := permissionService.GetUserEffectivePermissions(authUser.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective permissions after update via service: %v", err)}
	}

	if len(effectiveModels) != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 effective models after update via service, got %d", len(effectiveModels))}
	}

	serviceModelsMap := make(map[string]bool)
	for _, model := range effectiveModels {
		serviceModelsMap[model] = true
	}
	for _, expectedModel := range expectedModels {
		if !serviceModelsMap[expectedModel] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected model %s not found in service effective permissions after employee update", expectedModel)}
		}
	}

	// 5. Verify database record timestamps are correctly updated
	if updatedEmployee.UpdateTime.IsZero() {
		return TestResult{Passed: false, Message: "Employee update_time should not be zero after update"}
	}

	if effectivePermRecord.UpdateTime.IsZero() {
		return TestResult{Passed: false, Message: "Effective permission update_time should not be zero after update"}
	}

	// 6. Verify database foreign key relationship integrity
	// Check relationship between employee records and effective permission records
	var employeeEffectivePermCount int64
	if err := ctx.DB.DB.Raw(`
		SELECT COUNT(*) FROM effective_permissions ep
		INNER JOIN employee_department ed ON ep.employee_number = ed.employee_number
		WHERE ep.employee_number = ?
	`, "000070").Scan(&employeeEffectivePermCount).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to verify foreign key relationship: %v", err)}
	}

	if employeeEffectivePermCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 linked employee-effective permission record, got %d", employeeEffectivePermCount)}
	}

	// 7. Verify no redundant records are created in database
	var employeeRecordCount int64
	if err := ctx.DB.DB.Model(&models.EmployeeDepartment{}).Where("employee_number = ?", "000070").Count(&employeeRecordCount).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to count employee records: %v", err)}
	}

	if employeeRecordCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected exactly 1 employee record, got %d", employeeRecordCount)}
	}

	var effectivePermRecordCount int64
	if err := ctx.DB.DB.Model(&models.EffectivePermission{}).Where("employee_number = ?", "000070").Count(&effectivePermRecordCount).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to count effective permission records: %v", err)}
	}

	if effectivePermRecordCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected exactly 1 effective permission record, got %d", effectivePermRecordCount)}
	}

	// Clean up Mock HR data
	ClearMockData()

	return TestResult{Passed: true, Message: "Employee data integrity test with comprehensive database verification succeeded"}
}
