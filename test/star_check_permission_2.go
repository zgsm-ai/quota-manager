package main

import (
	"fmt"
	"quota-manager/internal/config"
	"quota-manager/internal/models"
	"quota-manager/internal/services"
)

// testStarCheckNotificationOptimization tests star check notification optimization
func testStarCheckNotificationOptimization(ctx *TestContext) TestResult {
	// Create mock config
	aiGatewayConfig := &config.AiGatewayConfig{
		Host:       "localhost",
		Port:       8080,
		AdminPath:  "/model-permission",
		AuthHeader: "x-admin-key",
		AuthValue:  "test-key",
	}

	defaultEmployeeSyncConfig := &config.EmployeeSyncConfig{
		Enabled: true,
		HrURL:   "http://localhost:8099/api/hr/employees",
		HrKey:   "test-hr-key",
		DeptURL: "http://localhost:8099/api/hr/departments",
		DeptKey: "test-dept-key",
	}

	starCheckPermissionService := services.NewStarCheckPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

	// Clear permission data for test isolation
	if err := clearPermissionData(ctx); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to clear permission data: %v", err)}
	}

	// Create test employees
	employees := []models.EmployeeDepartment{
		{
			EmployeeNumber:     "sc_test001",
			Username:           "user_no_star_settings",
			DeptFullLevelNames: "Tech_Group,Testing_Dept",
		},
		{
			EmployeeNumber:     "sc_test002",
			Username:           "user_with_star_settings",
			DeptFullLevelNames: "Tech_Group,Testing_Dept",
		},
	}

	for _, emp := range employees {
		if err := ctx.DB.DB.Create(&emp).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
		}
	}

	// Create auth user mappings for UUID-based operations under EmployeeSync=true
	_, errUID1 := createAuthUserForEmployee(ctx, employees[0].EmployeeNumber, employees[0].Username)
	if errUID1 != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create auth user for %s: %v", employees[0].EmployeeNumber, errUID1)}
	}
	uid2, errUID2 := createAuthUserForEmployee(ctx, employees[1].EmployeeNumber, employees[1].Username)
	if errUID2 != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create auth user for %s: %v", employees[1].EmployeeNumber, errUID2)}
	}

	// Clear star check calls
	mockStore.ClearStarCheckCalls()

	// Scenario 1: New user with default setting - should NOT notify Higress
	if err := starCheckPermissionService.UpdateEmployeeStarCheckPermissions("sc_test001"); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to update star check permissions for sc_test001: %v", err)}
	}

	calls := mockStore.GetStarCheckCalls()
	if len(calls) != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Scenario 1 failed: Expected 0 calls for new user with default setting, got %d", len(calls))}
	}

	// Scenario 2: New user gets enabled setting - should notify Higress (change from default false to true)
	mockStore.ClearStarCheckCalls()
	if err := starCheckPermissionService.SetUserStarCheckSetting(uid2, true); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set star check setting for sc_test002: %v", err)}
	}

	calls = mockStore.GetStarCheckCalls()
	if len(calls) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Scenario 2 failed: Expected 1 call for new user with enabled setting, got %d", len(calls))}
	}
	if calls[0].EmployeeNumber != "sc_test002" || !calls[0].Enabled {
		return TestResult{Passed: false, Message: "Scenario 2 failed: Incorrect Higress call content"}
	}

	// Scenario 3: Existing user setting changes - should notify Higress (change from true to false)
	mockStore.ClearStarCheckCalls()
	if err := starCheckPermissionService.SetUserStarCheckSetting(uid2, false); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to update star check setting for sc_test002: %v", err)}
	}

	calls = mockStore.GetStarCheckCalls()
	if len(calls) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Scenario 3 failed: Expected 1 call for setting change, got %d", len(calls))}
	}
	if calls[0].Enabled {
		return TestResult{Passed: false, Message: "Scenario 3 failed: Expected disabled setting"}
	}

	// Scenario 4: User setting doesn't change - should NOT notify Higress
	mockStore.ClearStarCheckCalls()
	if err := starCheckPermissionService.UpdateEmployeeStarCheckPermissions("sc_test002"); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to update star check permissions for sc_test002: %v", err)}
	}

	calls = mockStore.GetStarCheckCalls()
	if len(calls) != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Scenario 4 failed: Expected 0 calls for unchanged setting, got %d", len(calls))}
	}

	return TestResult{Passed: true, Message: "Star check notification optimization test succeeded"}
}

// testUserStarCheckSettingDistribution tests user star check setting distribution to Higress
func testUserStarCheckSettingDistribution(ctx *TestContext) TestResult {
	// Clear star check data to avoid interference from previous tests
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
		Enabled: true,
		HrURL:   "http://localhost:8099/api/hr/employees",
		HrKey:   "test-hr-key",
		DeptURL: "http://localhost:8099/api/hr/departments",
		DeptKey: "test-dept-key",
	}

	starCheckPermissionService := services.NewStarCheckPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

	// Create test employee
	employee := &models.EmployeeDepartment{
		EmployeeNumber:     "220001",
		Username:           "star_dist_test_user",
		DeptFullLevelNames: "Tech_Group,R&D_Center,Mobile_Dev_Dept,Mobile_Dev_Team1",
	}
	if err := ctx.DB.DB.Create(employee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
	}

	// Create auth mapping and use UUID for user-level operation
	uid, errUID := createAuthUserForEmployee(ctx, employee.EmployeeNumber, employee.Username)
	if errUID != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create auth user for %s: %v", employee.EmployeeNumber, errUID)}
	}

	// Clear previous star check calls
	mockStore.ClearStarCheckCalls()

	// Set user star check setting - this should trigger Higress sync
	if err := starCheckPermissionService.SetUserStarCheckSetting(uid, false); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set user star check setting: %v", err)}
	}

	// Verify the setting was synced to Higress (mock server)
	starCheckCalls := mockStore.GetStarCheckCalls()
	if len(starCheckCalls) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 star check call to Higress, got %d", len(starCheckCalls))}
	}

	call := starCheckCalls[0]
	if call.EmployeeNumber != "220001" {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected employee number 220001 in Higress call, got %s", call.EmployeeNumber)}
	}

	if call.Enabled {
		return TestResult{Passed: false, Message: "Expected star check to be disabled in Higress call"}
	}

	// Verify effective setting in database
	var effectiveSetting models.EffectiveStarCheckSetting
	if err := ctx.DB.DB.Where("employee_number = ?", "220001").First(&effectiveSetting).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to find effective star check setting: %v", err)}
	}

	if effectiveSetting.Enabled {
		return TestResult{Passed: false, Message: "Expected effective star check setting to be disabled"}
	}

	return TestResult{Passed: true, Message: "User star check setting distribution test succeeded"}
}

// testDepartmentStarCheckSettingDistribution tests department star check setting distribution to Higress
func testDepartmentStarCheckSettingDistribution(ctx *TestContext) TestResult {
	// Clear star check data to avoid interference from previous tests
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
		Enabled: true,
		HrURL:   "http://localhost:8099/api/hr/employees",
		HrKey:   "test-hr-key",
		DeptURL: "http://localhost:8099/api/hr/departments",
		DeptKey: "test-dept-key",
	}

	starCheckPermissionService := services.NewStarCheckPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

	// Create test employees in same department
	employees := []models.EmployeeDepartment{
		{
			EmployeeNumber:     "221002",
			Username:           "dept_star_test1",
			DeptFullLevelNames: "Tech_Group,R&D_Center,QA_Dept,QA_Team1",
		},
		{
			EmployeeNumber:     "221003",
			Username:           "dept_star_test2",
			DeptFullLevelNames: "Tech_Group,R&D_Center,QA_Dept,QA_Team1",
		},
		{
			EmployeeNumber:     "221004",
			Username:           "dept_star_test3",
			DeptFullLevelNames: "Tech_Group,R&D_Center,QA_Dept,QA_Team2",
		},
	}

	for _, emp := range employees {
		if err := ctx.DB.DB.Create(&emp).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
		}
	}

	// Clear previous star check calls
	mockStore.ClearStarCheckCalls()

	// Set department star check setting - this should trigger Higress sync for all employees
	if err := starCheckPermissionService.SetDepartmentStarCheckSetting("QA_Dept", false); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set department star check setting: %v", err)}
	}

	// Verify the settings were synced to Higress for all employees
	starCheckCalls := mockStore.GetStarCheckCalls()
	if len(starCheckCalls) != 3 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 3 star check calls to Higress, got %d", len(starCheckCalls))}
	}

	// Verify each call
	calledEmployees := make(map[string]bool)
	for _, call := range starCheckCalls {
		calledEmployees[call.EmployeeNumber] = true
		if call.Enabled {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected star check to be disabled for employee %s", call.EmployeeNumber)}
		}
	}

	// Verify all expected employees were called
	expectedEmployees := []string{"221002", "221003", "221004"}
	for _, empNum := range expectedEmployees {
		if !calledEmployees[empNum] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected Higress call for employee %s", empNum)}
		}
	}

	// Verify effective settings in database for all employees
	for _, empNum := range expectedEmployees {
		var effectiveSetting models.EffectiveStarCheckSetting
		if err := ctx.DB.DB.Where("employee_number = ?", empNum).First(&effectiveSetting).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Failed to find effective star check setting for employee %s: %v", empNum, err)}
		}

		if effectiveSetting.Enabled {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected effective star check setting to be disabled for employee %s", empNum)}
		}
	}

	return TestResult{Passed: true, Message: "Department star check setting distribution test succeeded"}
}

// testStarCheckSettingHierarchyLevel1 tests star check setting hierarchy with 1 level difference
func testStarCheckSettingHierarchyLevel1(ctx *TestContext) TestResult {
	return testStarCheckSettingHierarchyWithLevels(ctx, 1)
}

// testStarCheckSettingHierarchyLevel2 tests star check setting hierarchy with 2 level difference
func testStarCheckSettingHierarchyLevel2(ctx *TestContext) TestResult {
	return testStarCheckSettingHierarchyWithLevels(ctx, 2)
}

// testStarCheckSettingHierarchyLevel3 tests star check setting hierarchy with 3 level difference
func testStarCheckSettingHierarchyLevel3(ctx *TestContext) TestResult {
	return testStarCheckSettingHierarchyWithLevels(ctx, 3)
}

// testStarCheckSettingHierarchyLevel5 tests star check setting hierarchy with 5 level difference
func testStarCheckSettingHierarchyLevel5(ctx *TestContext) TestResult {
	return testStarCheckSettingHierarchyWithLevels(ctx, 5)
}

// testStarCheckSettingHierarchyWithLevels tests star check setting hierarchy with specified level difference
func testStarCheckSettingHierarchyWithLevels(ctx *TestContext, levelDiff int) TestResult {
	// Clear star check data to avoid interference from previous tests
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
		Enabled: true,
		HrURL:   "http://localhost:8099/api/hr/employees",
		HrKey:   "test-hr-key",
		DeptURL: "http://localhost:8099/api/hr/departments",
		DeptKey: "test-dept-key",
	}

	starCheckPermissionService := services.NewStarCheckPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

	// Create hierarchical department structure based on level difference
	var parentDept, childDept string
	var employeeNumber string

	switch levelDiff {
	case 1:
		parentDept = "R&D_Center"
		childDept = "StarCheck_Dev_Dept"
		employeeNumber = "230010"
	case 2:
		parentDept = "R&D_Center"
		childDept = "StarCheck_Dev_Dept_Team1"
		employeeNumber = "230011"
	case 3:
		parentDept = "R&D_Center"
		childDept = "StarCheck_Dev_Dept_Team1_SubTeam"
		employeeNumber = "230012"
	case 5:
		parentDept = "Tech_Group"
		childDept = "StarCheck_Dev_Dept_Team1_SubTeam_Alpha"
		employeeNumber = "230013"
	default:
		return TestResult{Passed: false, Message: fmt.Sprintf("Unsupported level difference: %d", levelDiff)}
	}

	// Create department hierarchy path
	var deptPath string
	switch levelDiff {
	case 1:
		deptPath = fmt.Sprintf("Tech_Group,R&D_Center,%s", childDept)
	case 2:
		deptPath = fmt.Sprintf("Tech_Group,R&D_Center,StarCheck_Dev_Dept,%s", childDept)
	case 3:
		deptPath = fmt.Sprintf("Tech_Group,R&D_Center,StarCheck_Dev_Dept,StarCheck_Dev_Dept_Team1,%s", childDept)
	case 5:
		deptPath = fmt.Sprintf("Tech_Group,R&D_Center,StarCheck_Dev_Dept,StarCheck_Dev_Dept_Team1,StarCheck_Dev_Dept_Team1_SubTeam,%s", childDept)
	}

	// Create test employee
	employee := &models.EmployeeDepartment{
		EmployeeNumber:     employeeNumber,
		Username:           fmt.Sprintf("star_hierarchy_test_%d", levelDiff),
		DeptFullLevelNames: deptPath,
	}
	if err := ctx.DB.DB.Create(employee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
	}

	// Set parent department star check setting
	if err := starCheckPermissionService.SetDepartmentStarCheckSetting(parentDept, false); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set parent department star check setting: %v", err)}
	}

	// Create auth mapping and query using UUID under EmployeeSync=true
	uid, errUID := createAuthUserForEmployee(ctx, employeeNumber, employee.Username)
	if errUID != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create auth user for %s: %v", employeeNumber, errUID)}
	}
	// Get effective star check setting - should inherit from parent department
	enabled, err := starCheckPermissionService.GetUserEffectiveStarCheckSetting(uid)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective star check setting: %v", err)}
	}

	if enabled {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected star check to be disabled (inherited from %s)", parentDept)}
	}

	// Set child department star check setting (should override parent)
	if err := starCheckPermissionService.SetDepartmentStarCheckSetting(childDept, true); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set child department star check setting: %v", err)}
	}

	// Get effective star check setting - should use child department setting
	childEnabled, err := starCheckPermissionService.GetUserEffectiveStarCheckSetting(uid)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get child effective star check setting: %v", err)}
	}

	if !childEnabled {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected star check to be enabled (from child department %s)", childDept)}
	}

	return TestResult{Passed: true, Message: fmt.Sprintf("Star check setting hierarchy level %d test succeeded", levelDiff)}
}

// testUserStarCheckSettingOverridesDepartment tests user star check setting overrides department
func testUserStarCheckSettingOverridesDepartment(ctx *TestContext) TestResult {
	// Clear star check data to avoid interference from previous tests
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
		Enabled: true,
		HrURL:   "http://localhost:8099/api/hr/employees",
		HrKey:   "test-hr-key",
		DeptURL: "http://localhost:8099/api/hr/departments",
		DeptKey: "test-dept-key",
	}

	starCheckPermissionService := services.NewStarCheckPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

	// Create test employees in same department
	employees := []models.EmployeeDepartment{
		{
			EmployeeNumber:     "240001",
			Username:           "override_test_user1",
			DeptFullLevelNames: "Tech_Group,Data_Center,Analytics_Dept",
		},
		{
			EmployeeNumber:     "240002",
			Username:           "override_test_user2",
			DeptFullLevelNames: "Tech_Group,Data_Center,Analytics_Dept",
		},
	}

	for _, emp := range employees {
		if err := ctx.DB.DB.Create(&emp).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
		}
	}

	// Set department star check setting to disabled
	if err := starCheckPermissionService.SetDepartmentStarCheckSetting("Analytics_Dept", false); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set department star check setting: %v", err)}
	}

	// Create auth mappings and verify both users inherit department setting (disabled) using UUID
	uid1, errUID1 := createAuthUserForEmployee(ctx, "240001", employees[0].Username)
	if errUID1 != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create auth user for 240001: %v", errUID1)}
	}
	uid2, errUID2 := createAuthUserForEmployee(ctx, "240002", employees[1].Username)
	if errUID2 != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create auth user for 240002: %v", errUID2)}
	}
	for _, uid := range []string{uid1, uid2} {
		enabled, err := starCheckPermissionService.GetUserEffectiveStarCheckSetting(uid)
		if err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective star check setting for %s: %v", uid, err)}
		}
		if enabled {
			return TestResult{Passed: false, Message: "Expected star check to be disabled for department setting"}
		}
	}

	// Set user star check setting to enabled for first user (should override department)
	if err := starCheckPermissionService.SetUserStarCheckSetting(uid1, true); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set user star check setting: %v", err)}
	}

	// Verify first user now has enabled star check (overrides department)
	user1Enabled, err := starCheckPermissionService.GetUserEffectiveStarCheckSetting(uid1)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get user1 effective star check setting: %v", err)}
	}
	if !user1Enabled {
		return TestResult{Passed: false, Message: "Expected user1 star check to be enabled (user setting overrides department)"}
	}

	// Verify second user still has department setting (disabled)
	user2Enabled, err := starCheckPermissionService.GetUserEffectiveStarCheckSetting(uid2)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get user2 effective star check setting: %v", err)}
	}
	if user2Enabled {
		return TestResult{Passed: false, Message: "Expected user2 star check to remain disabled (department setting)"}
	}

	return TestResult{Passed: true, Message: "User star check setting overrides department test succeeded"}
}

// testDepartmentStarCheckSettingChange tests department star check setting changes
func testDepartmentStarCheckSettingChange(ctx *TestContext) TestResult {
	// Create services
	aiGatewayConfig := &config.AiGatewayConfig{
		Host:       "localhost",
		Port:       8080,
		AdminPath:  "/model-permission",
		AuthHeader: "x-admin-key",
		AuthValue:  "test-key",
	}

	defaultEmployeeSyncConfig := &config.EmployeeSyncConfig{
		Enabled: true,
		HrURL:   "http://localhost:8099/api/hr/employees",
		HrKey:   "test-hr-key",
		DeptURL: "http://localhost:8099/api/hr/departments",
		DeptKey: "test-dept-key",
	}

	starCheckPermissionService := services.NewStarCheckPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

	// Create test employees
	employees := []models.EmployeeDepartment{
		{
			EmployeeNumber:     "250030",
			Username:           "change_test_star_employee1",
			DeptFullLevelNames: "Tech_Group,R&D_Center,Platform_Dept,Platform_Team1",
		},
		{
			EmployeeNumber:     "250031",
			Username:           "change_test_star_employee2",
			DeptFullLevelNames: "Tech_Group,R&D_Center,Platform_Dept,Platform_Team1",
		},
	}

	for _, emp := range employees {
		if err := ctx.DB.DB.Create(&emp).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
		}
	}

	// Initially set department star check setting to disabled
	if err := starCheckPermissionService.SetDepartmentStarCheckSetting("Platform_Dept", false); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set initial department star check setting: %v", err)}
	}

	// Clear star check calls and wait a moment
	mockStore.ClearStarCheckCalls()

	// Change department star check setting to enabled
	if err := starCheckPermissionService.SetDepartmentStarCheckSetting("Platform_Dept", true); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to update department star check setting: %v", err)}
	}

	// Verify Higress calls were made for all affected employees
	starCheckCalls := mockStore.GetStarCheckCalls()
	if len(starCheckCalls) != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 star check calls for department change, got %d", len(starCheckCalls))}
	}

	// Verify all calls are for enabling star check
	calledEmployees := make(map[string]bool)
	for _, call := range starCheckCalls {
		calledEmployees[call.EmployeeNumber] = true
		if !call.Enabled {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected star check to be enabled for employee %s", call.EmployeeNumber)}
		}
	}

	// Verify all expected employees were notified
	expectedEmployees := []string{"250030", "250031"}
	for _, empNum := range expectedEmployees {
		if !calledEmployees[empNum] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected Higress call for employee %s", empNum)}
		}
	}

	// Verify effective settings for all employees (use UUID under EmployeeSync=true)
	for _, empNum := range expectedEmployees {
		uid, errUID := createAuthUserForEmployee(ctx, empNum, "dept_change_user")
		if errUID != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create auth user for %s: %v", empNum, errUID)}
		}
		enabled, err := starCheckPermissionService.GetUserEffectiveStarCheckSetting(uid)
		if err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective star check setting for %s: %v", empNum, err)}
		}
		if !enabled {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected star check to be enabled for employee %s", empNum)}
		}
	}

	return TestResult{Passed: true, Message: "Department star check setting change test succeeded"}
}
