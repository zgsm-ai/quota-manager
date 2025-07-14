package main

import (
	"fmt"
	"quota-manager/internal/config"
	"quota-manager/internal/models"
	"quota-manager/internal/services"
)

// testDepartmentQuotaCheckSettingChange tests department quota check setting changes
func testDepartmentQuotaCheckSettingChange(ctx *TestContext) TestResult {
	// Create services
	aiGatewayConfig := &config.AiGatewayConfig{
		Host:       "localhost",
		Port:       8080,
		AdminPath:  "/model-permission",
		AuthHeader: "x-admin-key",
		AuthValue:  "test-key",
	}

	defaultEmployeeSyncConfig := &config.EmployeeSyncConfig{
		Enabled: false,
		HrURL:   "http://localhost:8099/api/hr/employees",
		HrKey:   "test-hr-key",
		DeptURL: "http://localhost:8099/api/hr/departments",
		DeptKey: "test-dept-key",
	}

	quotaCheckPermissionService := services.NewQuotaCheckPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

	// Create test employees
	employees := []models.EmployeeDepartment{
		{
			EmployeeNumber:     "350030",
			Username:           "dept_change_test_employee1",
			DeptFullLevelNames: "Tech_Group,R&D_Center,Testing_Dept,Testing_Team1",
		},
		{
			EmployeeNumber:     "350031",
			Username:           "dept_change_test_employee2",
			DeptFullLevelNames: "Tech_Group,R&D_Center,Testing_Dept,Testing_Team2",
		},
	}

	for _, emp := range employees {
		if err := ctx.DB.DB.Create(&emp).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
		}
	}

	// Initially set department quota check setting to disabled
	if err := quotaCheckPermissionService.SetDepartmentQuotaCheckSetting("Testing_Dept", false); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set initial department quota check setting: %v", err)}
	}

	// Clear quota check calls
	mockStore.ClearQuotaCheckCalls()

	// Change department quota check setting to enabled
	if err := quotaCheckPermissionService.SetDepartmentQuotaCheckSetting("Testing_Dept", true); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to update department quota check setting: %v", err)}
	}

	// Verify Higress calls were made for all employees in the department
	quotaCheckCalls := mockStore.GetQuotaCheckCalls()
	if len(quotaCheckCalls) != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 quota check calls for department change, got %d", len(quotaCheckCalls))}
	}

	// Verify all calls are for enabling quota check
	calledEmployees := make(map[string]bool)
	for _, call := range quotaCheckCalls {
		calledEmployees[call.EmployeeNumber] = true
		if !call.Enabled {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected quota check to be enabled for employee %s", call.EmployeeNumber)}
		}
	}

	// Verify all expected employees were notified
	expectedEmployees := []string{"350030", "350031"}
	for _, empNum := range expectedEmployees {
		if !calledEmployees[empNum] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected Higress call for employee %s", empNum)}
		}
	}

	// Verify effective settings for all employees
	for _, empNum := range expectedEmployees {
		enabled, err := quotaCheckPermissionService.GetUserEffectiveQuotaCheckSetting(empNum)
		if err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective quota check setting for %s: %v", empNum, err)}
		}
		if !enabled {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected quota check to be enabled for employee %s", empNum)}
		}
	}

	return TestResult{Passed: true, Message: "Department quota check setting change test succeeded"}
}

// testUserQuotaCheckSettingChange tests user quota check setting changes
func testUserQuotaCheckSettingChange(ctx *TestContext) TestResult {
	// Create services
	aiGatewayConfig := &config.AiGatewayConfig{
		Host:       "localhost",
		Port:       8080,
		AdminPath:  "/model-permission",
		AuthHeader: "x-admin-key",
		AuthValue:  "test-key",
	}

	defaultEmployeeSyncConfig := &config.EmployeeSyncConfig{
		Enabled: false,
		HrURL:   "http://localhost:8099/api/hr/employees",
		HrKey:   "test-hr-key",
		DeptURL: "http://localhost:8099/api/hr/departments",
		DeptKey: "test-dept-key",
	}

	quotaCheckPermissionService := services.NewQuotaCheckPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

	// Create test employee
	employee := &models.EmployeeDepartment{
		EmployeeNumber:     "360040",
		Username:           "user_change_test_quota_employee",
		DeptFullLevelNames: "Tech_Group,Operations_Center,DevOps_Dept",
	}
	if err := ctx.DB.DB.Create(employee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
	}

	// Initially set user quota check setting to disabled
	if err := quotaCheckPermissionService.SetUserQuotaCheckSetting("360040", false); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set initial user quota check setting: %v", err)}
	}

	// Clear quota check calls
	mockStore.ClearQuotaCheckCalls()

	// Change user quota check setting to enabled
	if err := quotaCheckPermissionService.SetUserQuotaCheckSetting("360040", true); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to update user quota check setting: %v", err)}
	}

	// Verify Higress call was made
	quotaCheckCalls := mockStore.GetQuotaCheckCalls()
	if len(quotaCheckCalls) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 quota check call for user change, got %d", len(quotaCheckCalls))}
	}

	call := quotaCheckCalls[0]
	if call.EmployeeNumber != "360040" {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected employee number 360040, got %s", call.EmployeeNumber)}
	}

	if !call.Enabled {
		return TestResult{Passed: false, Message: "Expected quota check to be enabled in Higress call"}
	}

	// Verify effective setting in database
	enabled, err := quotaCheckPermissionService.GetUserEffectiveQuotaCheckSetting("360040")
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective quota check setting: %v", err)}
	}

	if !enabled {
		return TestResult{Passed: false, Message: "Expected effective quota check setting to be enabled"}
	}

	return TestResult{Passed: true, Message: "User quota check setting change test succeeded"}
}

// testUserDepartmentQuotaCheckChange tests user department changes affecting quota check settings
func testUserDepartmentQuotaCheckChange(ctx *TestContext) TestResult {
	// Create services
	aiGatewayConfig := &config.AiGatewayConfig{
		Host:       "localhost",
		Port:       8080,
		AdminPath:  "/model-permission",
		AuthHeader: "x-admin-key",
		AuthValue:  "test-key",
	}

	// Enable employee sync to properly test department changes
	employeeSyncConfig := &config.EmployeeSyncConfig{
		Enabled: true,
		HrURL:   ctx.MockServer.URL + "/api/test/employees",
		HrKey:   "TEST_EMP_KEY_32_BYTES_1234567890",
		DeptURL: ctx.MockServer.URL + "/api/test/departments",
		DeptKey: "TEST_DEPT_KEY_32_BYTES_123456789",
	}

	quotaCheckPermissionService := services.NewQuotaCheckPermissionService(ctx.DB, aiGatewayConfig, employeeSyncConfig, ctx.Gateway)
	permissionService := services.NewPermissionService(ctx.DB, aiGatewayConfig, employeeSyncConfig, ctx.Gateway)
	starCheckPermissionService := services.NewStarCheckPermissionService(ctx.DB, aiGatewayConfig, employeeSyncConfig, ctx.Gateway)
	employeeSyncService := services.NewEmployeeSyncService(ctx.DB, employeeSyncConfig, permissionService, starCheckPermissionService, quotaCheckPermissionService)

	// Clear permission data for test isolation
	if err := clearPermissionData(ctx); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to clear permission data: %v", err)}
	}

	// Setup mock HR data for testing
	ClearMockData()
	SetupDefaultDepartmentHierarchy()

	// Create test employee initially in one department
	AddMockEmployee("370001", "dept_change_test_employee", "dept_change@example.com", "13800370001", 1) // Tech_Group

	// Sync employees to get initial state
	if err := employeeSyncService.SyncEmployees(); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Initial employee sync failed: %v", err)}
	}

	// Set quota check setting for both departments
	if err := quotaCheckPermissionService.SetDepartmentQuotaCheckSetting("Tech_Group", false); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set Tech_Group quota check setting: %v", err)}
	}

	// Add employee to UX_Dept to ensure department exists before setting permissions
	AddMockEmployee("370002", "ux_dept_employee", "ux_dept@example.com", "13800370002", 3) // UX_Dept

	// Sync employees to ensure UX_Dept employee is in the database
	if err := employeeSyncService.SyncEmployees(); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to sync UX_Dept employee: %v", err)}
	}

	if err := quotaCheckPermissionService.SetDepartmentQuotaCheckSetting("UX_Dept", true); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set UX_Dept quota check setting: %v", err)}
	}

	// Clear quota check calls
	mockStore.ClearQuotaCheckCalls()

	// Change user's department from Tech_Group to UX_Dept
	UpdateMockEmployeeDepartment("370001", 3) // UX_Dept

	// Sync employees to reflect department change
	if err := employeeSyncService.SyncEmployees(); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Employee sync after department change failed: %v", err)}
	}

	// Verify quota check permission was updated due to department change
	enabled, err := quotaCheckPermissionService.GetUserEffectiveQuotaCheckSetting("370001")
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective quota check setting after department change: %v", err)}
	}

	if !enabled {
		return TestResult{Passed: false, Message: "Expected quota check to be enabled after moving to UX_Dept"}
	}

	// Verify Higress was notified about the change
	quotaCheckCalls := mockStore.GetQuotaCheckCalls()
	if len(quotaCheckCalls) == 0 {
		return TestResult{Passed: false, Message: "Expected at least one quota check call after department change"}
	}

	// Find the call for our employee
	var employeeCall *QuotaCheckCall
	for _, call := range quotaCheckCalls {
		if call.EmployeeNumber == "370001" {
			employeeCall = &call
			break
		}
	}

	if employeeCall == nil {
		return TestResult{Passed: false, Message: "Expected Higress call for employee 370001 after department change"}
	}

	if !employeeCall.Enabled {
		return TestResult{Passed: false, Message: "Expected quota check to be enabled in Higress call after department change"}
	}

	return TestResult{Passed: true, Message: "User department quota check change test succeeded"}
}

// testUserQuotaCheckAdditionAndRemoval tests user addition and removal simulation
func testUserQuotaCheckAdditionAndRemoval(ctx *TestContext) TestResult {
	// Create services
	aiGatewayConfig := &config.AiGatewayConfig{
		Host:       "localhost",
		Port:       8080,
		AdminPath:  "/model-permission",
		AuthHeader: "x-admin-key",
		AuthValue:  "test-key",
	}

	// Enable employee sync for proper testing
	employeeSyncConfig := &config.EmployeeSyncConfig{
		Enabled: true,
		HrURL:   ctx.MockServer.URL + "/api/test/employees",
		HrKey:   "TEST_EMP_KEY_32_BYTES_1234567890",
		DeptURL: ctx.MockServer.URL + "/api/test/departments",
		DeptKey: "TEST_DEPT_KEY_32_BYTES_123456789",
	}

	quotaCheckPermissionService := services.NewQuotaCheckPermissionService(ctx.DB, aiGatewayConfig, employeeSyncConfig, ctx.Gateway)
	permissionService := services.NewPermissionService(ctx.DB, aiGatewayConfig, employeeSyncConfig, ctx.Gateway)
	starCheckPermissionService := services.NewStarCheckPermissionService(ctx.DB, aiGatewayConfig, employeeSyncConfig, ctx.Gateway)
	employeeSyncService := services.NewEmployeeSyncService(ctx.DB, employeeSyncConfig, permissionService, starCheckPermissionService, quotaCheckPermissionService)

	// Clear permission data for test isolation
	if err := clearPermissionData(ctx); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to clear permission data: %v", err)}
	}

	// Setup mock HR data for testing
	ClearMockData()
	SetupDefaultDepartmentHierarchy()

	// Create a temporary employee in the target department so we can set its quota check setting
	AddMockEmployee("380001", "temp_qa_employee", "temp_qa@example.com", "13800380001", 4) // UX_Dept_Team1

	// Sync employees to get initial state
	if err := employeeSyncService.SyncEmployees(); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Initial employee sync failed: %v", err)}
	}

	// Set department quota check setting
	if err := quotaCheckPermissionService.SetDepartmentQuotaCheckSetting("UX_Dept", true); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set department quota check setting: %v", err)}
	}

	// Remove the temporary employee
	RemoveMockEmployeeByNumber("380001")

	// Test Scenario 1: Add new employee to department with quota check setting
	AddMockEmployee("addition_001", "new_qa_employee", "new_qa@example.com", "13800380002", 4) // UX_Dept_Team1

	// Clear quota check calls
	mockStore.ClearQuotaCheckCalls()

	// Sync employees to add new employee
	if err := employeeSyncService.SyncEmployees(); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Employee sync for addition failed: %v", err)}
	}

	// Verify new employee inherits department quota check setting
	enabled, err := quotaCheckPermissionService.GetUserEffectiveQuotaCheckSetting("addition_001")
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective quota check setting for new employee: %v", err)}
	}

	if !enabled {
		return TestResult{Passed: false, Message: "Expected new employee to inherit enabled quota check setting from department"}
	}

	// Verify Higress was notified for new employee
	quotaCheckCalls := mockStore.GetQuotaCheckCalls()
	if len(quotaCheckCalls) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 quota check call for new employee, got %d", len(quotaCheckCalls))}
	}

	if quotaCheckCalls[0].EmployeeNumber != "addition_001" || !quotaCheckCalls[0].Enabled {
		return TestResult{Passed: false, Message: "Incorrect Higress call for new employee"}
	}

	// Test Scenario 2: Remove employee
	RemoveMockEmployeeByNumber("addition_001")

	// Clear quota check calls
	mockStore.ClearQuotaCheckCalls()

	// Sync employees to remove employee
	if err := employeeSyncService.SyncEmployees(); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Employee sync for removal failed: %v", err)}
	}

	// Verify employee data is cleaned up
	var effectiveSettingCount int64
	if err := ctx.DB.DB.Model(&models.EffectiveQuotaCheckSetting{}).Where("employee_number = ?", "addition_001").Count(&effectiveSettingCount).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to count effective quota check settings: %v", err)}
	}

	if effectiveSettingCount != 0 {
		return TestResult{Passed: false, Message: "Expected effective quota check setting to be removed for deleted employee"}
	}

	return TestResult{Passed: true, Message: "User quota check addition and removal test succeeded"}
}

// testNonExistentUserAndDepartmentQuotaCheck tests non-existent user and department quota check scenarios
func testNonExistentUserAndDepartmentQuotaCheck(ctx *TestContext) TestResult {
	// Test both employee_sync enabled and disabled scenarios
	scenarios := []struct {
		name            string
		employeeEnabled bool
		expectUserError bool
		description     string
	}{
		{
			name:            "Employee sync disabled",
			employeeEnabled: false,
			expectUserError: false,
			description:     "When employee_sync.enabled is false, setting quota check for non-existent user should succeed",
		},
		{
			name:            "Employee sync enabled",
			employeeEnabled: true,
			expectUserError: true,
			description:     "When employee_sync.enabled is true, setting quota check for non-existent user should fail",
		},
	}

	for _, scenario := range scenarios {
		fmt.Printf("\n=== Testing %s ===\n", scenario.name)
		fmt.Printf("Description: %s\n", scenario.description)

		// Clear permission data before each scenario to ensure clean state
		if err := clearPermissionData(ctx); err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Failed to clear permission data before scenario %s: %v", scenario.name, err)}
		}

		result := testNonExistentUserQuotaCheckScenario(ctx, scenario.employeeEnabled, scenario.expectUserError)
		if !result.Passed {
			return TestResult{
				Passed:  false,
				Message: fmt.Sprintf("Failed at %s: %s", scenario.name, result.Message),
			}
		}

		fmt.Printf("✅ %s completed successfully\n", scenario.name)
	}

	return TestResult{Passed: true, Message: "Non-existent user and department quota check test - both employee_sync enabled and disabled scenarios succeeded"}
}

// testNonExistentUserQuotaCheckScenario tests a specific employee_sync configuration scenario
func testNonExistentUserQuotaCheckScenario(ctx *TestContext, employeeEnabled, expectUserError bool) TestResult {
	// Create services with appropriate employee sync configuration
	aiGatewayConfig := &config.AiGatewayConfig{
		Host:       "localhost",
		Port:       8080,
		AdminPath:  "/model-permission",
		AuthHeader: "x-admin-key",
		AuthValue:  "test-key",
	}

	employeeSyncConfig := &config.EmployeeSyncConfig{
		Enabled: employeeEnabled,
		HrURL:   "http://localhost:8099/api/hr/employees",
		HrKey:   "test-hr-key",
		DeptURL: "http://localhost:8099/api/hr/departments",
		DeptKey: "test-dept-key",
	}

	quotaCheckPermissionService := services.NewQuotaCheckPermissionService(ctx.DB, aiGatewayConfig, employeeSyncConfig, ctx.Gateway)

	// Test 1: Set quota check for non-existent user
	err1 := quotaCheckPermissionService.SetUserQuotaCheckSetting("999999", true)

	if expectUserError {
		// When employee_sync is enabled, expect failure
		if err1 == nil {
			return TestResult{Passed: false, Message: "Expected error when setting quota check for non-existent user with employee_sync enabled, but got no error"}
		}
		// Check if it's the expected error message
		expectedUserErrorMsg := "user not found: employee number '999999' does not exist"
		if err1.Error() != expectedUserErrorMsg {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected error message '%s' for non-existent user, got '%s'", expectedUserErrorMsg, err1.Error())}
		}

		// Verify no quota check records were created for non-existent user
		var nonExistentUserSettingCount int64
		if err := ctx.DB.DB.Model(&models.QuotaCheckSetting{}).Where("target_type = ? AND target_identifier = ?", "user", "999999").Count(&nonExistentUserSettingCount).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Failed to count quota check settings for non-existent user: %v", err)}
		}

		if nonExistentUserSettingCount > 0 {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected no quota check settings for non-existent user when employee_sync enabled, got %d", nonExistentUserSettingCount)}
		}
	} else {
		// When employee_sync is disabled, expect success
		if err1 != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected no error when setting quota check for non-existent user with employee_sync disabled, but got error: %v", err1)}
		}

		// Verify quota check setting was created for non-existent user
		var nonExistentUserSettingCount int64
		if err := ctx.DB.DB.Model(&models.QuotaCheckSetting{}).Where("target_type = ? AND target_identifier = ?", "user", "999999").Count(&nonExistentUserSettingCount).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Failed to count quota check settings for non-existent user: %v", err)}
		}

		if nonExistentUserSettingCount != 1 {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 quota check setting for non-existent user when employee_sync disabled, got %d", nonExistentUserSettingCount)}
		}

		// Verify effective quota check setting can be retrieved (should be the set value)
		enabled, err := quotaCheckPermissionService.GetUserEffectiveQuotaCheckSetting("999999")
		if err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective quota check setting for non-existent user: %v", err)}
		}

		if !enabled {
			return TestResult{Passed: false, Message: "Expected effective quota check setting to be enabled for non-existent user"}
		}
	}

	// Test 2: Set quota check for non-existent department - should return error
	err2 := quotaCheckPermissionService.SetDepartmentQuotaCheckSetting("NonExistent_Dept", true)
	if err2 == nil {
		return TestResult{Passed: false, Message: "Expected error when setting quota check for non-existent department, but got no error"}
	}

	// Check if it's an expected error about department not found
	expectedDeptErrorMsg := "department not found: no employees belong to department 'NonExistent_Dept'"
	if err2.Error() != expectedDeptErrorMsg {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected error message '%s' for non-existent department, got '%s'", expectedDeptErrorMsg, err2.Error())}
	}

	// Verify no quota check records were created in database for non-existent department
	var nonExistentDeptSettingCount int64
	if err := ctx.DB.DB.Model(&models.QuotaCheckSetting{}).Where("target_type = ? AND target_identifier = ?", "department", "NonExistent_Dept").Count(&nonExistentDeptSettingCount).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to count quota check settings for non-existent department: %v", err)}
	}

	if nonExistentDeptSettingCount > 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected no quota check settings for non-existent department, got %d", nonExistentDeptSettingCount)}
	}

	// Test 3: Get quota check for non-existent user - should return default (false)
	enabled3, err3 := quotaCheckPermissionService.GetUserEffectiveQuotaCheckSetting("999998")
	if err3 != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get quota check setting for non-existent user: %v", err3)}
	}

	if enabled3 {
		return TestResult{Passed: false, Message: "Expected default quota check setting (false) for non-existent user"}
	}

	return TestResult{Passed: true, Message: "Non-existent user and department quota check scenario test succeeded"}
}

// testQuotaCheckEmployeeDataIntegrity tests quota check employee data integrity
func testQuotaCheckEmployeeDataIntegrity(ctx *TestContext) TestResult {
	// Clear permission data for test isolation
	if err := clearPermissionData(ctx); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to clear permission data: %v", err)}
	}

	// Create services
	aiGatewayConfig := &config.AiGatewayConfig{
		Host:       "localhost",
		Port:       8080,
		AdminPath:  "/model-permission",
		AuthHeader: "x-admin-key",
		AuthValue:  "test-key",
	}

	defaultEmployeeSyncConfig := &config.EmployeeSyncConfig{
		Enabled: false,
		HrURL:   "http://localhost:8099/api/hr/employees",
		HrKey:   "test-hr-key",
		DeptURL: "http://localhost:8099/api/hr/departments",
		DeptKey: "test-dept-key",
	}

	quotaCheckPermissionService := services.NewQuotaCheckPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

	// Create test employee
	employee := &models.EmployeeDepartment{
		EmployeeNumber:     "390001",
		Username:           "data_integrity_test_employee",
		DeptFullLevelNames: "Company,Tech_Group,R&D_Center,Backend_Dev_Dept",
	}
	if err := ctx.DB.DB.Create(employee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
	}

	// Test 1: Set user quota check setting and verify database integrity
	if err := quotaCheckPermissionService.SetUserQuotaCheckSetting("390001", true); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set user quota check setting: %v", err)}
	}

	// Verify quota check setting record
	var quotaCheckSetting models.QuotaCheckSetting
	if err := ctx.DB.DB.Where("target_type = ? AND target_identifier = ?", "user", "390001").First(&quotaCheckSetting).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to find quota check setting: %v", err)}
	}

	if !quotaCheckSetting.Enabled {
		return TestResult{Passed: false, Message: "Expected quota check setting to be enabled"}
	}

	// Verify effective quota check setting record
	var effectiveQuotaCheckSetting models.EffectiveQuotaCheckSetting
	if err := ctx.DB.DB.Where("employee_number = ?", "390001").First(&effectiveQuotaCheckSetting).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to find effective quota check setting: %v", err)}
	}

	if !effectiveQuotaCheckSetting.Enabled {
		return TestResult{Passed: false, Message: "Expected effective quota check setting to be enabled"}
	}

	// Test 2: Update setting and verify consistency
	if err := quotaCheckPermissionService.SetUserQuotaCheckSetting("390001", false); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to update user quota check setting: %v", err)}
	}

	// Verify updated quota check setting
	if err := ctx.DB.DB.Where("target_type = ? AND target_identifier = ?", "user", "390001").First(&quotaCheckSetting).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to find updated quota check setting: %v", err)}
	}

	if quotaCheckSetting.Enabled {
		return TestResult{Passed: false, Message: "Expected quota check setting to be disabled after update"}
	}

	// Verify updated effective quota check setting
	if err := ctx.DB.DB.Where("employee_number = ?", "390001").First(&effectiveQuotaCheckSetting).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to find updated effective quota check setting: %v", err)}
	}

	if effectiveQuotaCheckSetting.Enabled {
		return TestResult{Passed: false, Message: "Expected effective quota check setting to be disabled after update"}
	}

	// Test 3: Test department quota check setting integrity
	if err := quotaCheckPermissionService.SetDepartmentQuotaCheckSetting("Backend_Dev_Dept", true); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set department quota check setting: %v", err)}
	}

	// Verify department quota check setting
	var deptQuotaCheckSetting models.QuotaCheckSetting
	if err := ctx.DB.DB.Where("target_type = ? AND target_identifier = ?", "department", "Backend_Dev_Dept").First(&deptQuotaCheckSetting).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to find department quota check setting: %v", err)}
	}

	if !deptQuotaCheckSetting.Enabled {
		return TestResult{Passed: false, Message: "Expected department quota check setting to be enabled"}
	}

	return TestResult{Passed: true, Message: "Quota check employee data integrity test succeeded"}
}

// testQuotaCheckEmployeeSync tests quota check employee synchronization
func testQuotaCheckEmployeeSync(ctx *TestContext) TestResult {
	// Enable employee sync for proper testing
	employeeSyncConfig := &config.EmployeeSyncConfig{
		Enabled: true,
		HrURL:   ctx.MockServer.URL + "/api/test/employees",
		HrKey:   "TEST_EMP_KEY_32_BYTES_1234567890",
		DeptURL: ctx.MockServer.URL + "/api/test/departments",
		DeptKey: "TEST_DEPT_KEY_32_BYTES_123456789",
	}

	// Create services
	aiGatewayConfig := &config.AiGatewayConfig{
		Host:       "localhost",
		Port:       8080,
		AdminPath:  "/model-permission",
		AuthHeader: "x-admin-key",
		AuthValue:  "test-key",
	}

	quotaCheckPermissionService := services.NewQuotaCheckPermissionService(ctx.DB, aiGatewayConfig, employeeSyncConfig, ctx.Gateway)
	permissionService := services.NewPermissionService(ctx.DB, aiGatewayConfig, employeeSyncConfig, ctx.Gateway)
	starCheckPermissionService := services.NewStarCheckPermissionService(ctx.DB, aiGatewayConfig, employeeSyncConfig, ctx.Gateway)
	employeeSyncService := services.NewEmployeeSyncService(ctx.DB, employeeSyncConfig, permissionService, starCheckPermissionService, quotaCheckPermissionService)

	// Clear permission data for test isolation
	if err := clearPermissionData(ctx); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to clear permission data: %v", err)}
	}

	// Setup mock HR data
	ClearMockData()
	SetupDefaultDepartmentHierarchy()
	AddMockEmployee("400001", "sync_test_employee", "sync_test@example.com", "13800400001", 2) // R&D_Center

	// First sync employees to ensure they exist in the database
	if err := employeeSyncService.SyncEmployees(); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Employee sync failed: %v", err)}
	}

	// Clear quota check calls before setting department quota check setting
	mockStore.ClearQuotaCheckCalls()

	// Set department quota check setting (now that employees exist in database)
	// This should trigger quota check notifications for all employees in the department
	if err := quotaCheckPermissionService.SetDepartmentQuotaCheckSetting("R&D_Center", true); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set department quota check setting: %v", err)}
	}

	// Verify employee was synced and inherited quota check setting
	enabled, err := quotaCheckPermissionService.GetUserEffectiveQuotaCheckSetting("400001")
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective quota check setting: %v", err)}
	}

	if !enabled {
		return TestResult{Passed: false, Message: "Expected employee to inherit enabled quota check setting from department"}
	}

	// Verify Higress was notified
	quotaCheckCalls := mockStore.GetQuotaCheckCalls()
	if len(quotaCheckCalls) == 0 {
		return TestResult{Passed: false, Message: "Expected at least one quota check call after setting department quota check setting"}
	}

	// Find the call for our employee
	var employeeCall *QuotaCheckCall
	for _, call := range quotaCheckCalls {
		if call.EmployeeNumber == "400001" {
			employeeCall = &call
			break
		}
	}

	if employeeCall == nil {
		return TestResult{Passed: false, Message: "Expected Higress call for employee 400001"}
	}

	if !employeeCall.Enabled {
		return TestResult{Passed: false, Message: "Expected quota check to be enabled in Higress call"}
	}

	return TestResult{Passed: true, Message: "Quota check employee sync test succeeded"}
}
