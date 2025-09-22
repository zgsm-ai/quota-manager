package main

import (
	"fmt"
	"quota-manager/internal/config"
	"quota-manager/internal/models"
	"quota-manager/internal/services"
	"time"

	"github.com/google/uuid"
)

// testSyncWithoutQuotaCheckSetting tests sync without quota check settings
func testSyncWithoutQuotaCheckSetting(ctx *TestContext) TestResult {
	// Clear quota check data to avoid interference from previous tests
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

	quotaCheckPermissionService := services.NewQuotaCheckPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

	// Create test employee
	employee := &models.EmployeeDepartment{
		EmployeeNumber:     "311001",
		Username:           "no_quota_setting_sync_employee",
		DeptFullLevelNames: "Operations_Group,Support_Center,Customer_Service_Dept",
	}
	if err := ctx.DB.DB.Create(employee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
	}

	// Create corresponding auth user for mapping (employee_sync enabled)
	authUser := &models.UserInfo{
		ID:             uuid.NewString(),
		CreatedAt:      time.Now().Add(-time.Hour),
		UpdatedAt:      time.Now(),
		AccessTime:     time.Now(),
		Name:           employee.Username,
		EmployeeNumber: employee.EmployeeNumber,
		GithubID:       fmt.Sprintf("test_%s_%d", employee.EmployeeNumber, time.Now().UnixNano()),
		GithubName:     employee.Username,
		Devices:        "{}",
	}
	if err := ctx.DB.AuthDB.Create(authUser).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create auth user: %v", err)}
	}

	// Clear previous quota check calls
	mockStore.ClearQuotaCheckCalls()

	// Trigger permission update when no settings exist
	if err := quotaCheckPermissionService.UpdateEmployeeQuotaCheckPermissions(employee.EmployeeNumber); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to update quota check permissions: %v", err)}
	}

	// Should NOT notify Higress for new user with default (disabled) setting
	quotaCheckCalls := mockStore.GetQuotaCheckCalls()
	if len(quotaCheckCalls) != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 0 quota check calls for new user with default setting, got %d", len(quotaCheckCalls))}
	}

	// Verify effective setting is default (disabled) via UUID under EmployeeSync=true
	enabled, err := quotaCheckPermissionService.GetUserEffectiveQuotaCheckSetting(authUser.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective quota check setting: %v", err)}
	}

	if enabled {
		return TestResult{Passed: false, Message: "Expected default quota check setting to be disabled"}
	}

	return TestResult{Passed: true, Message: "Sync without quota check setting test succeeded"}
}

// testQuotaCheckNotificationOptimization tests quota check notification optimization
func testQuotaCheckNotificationOptimization(ctx *TestContext) TestResult {
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

	quotaCheckPermissionService := services.NewQuotaCheckPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

	// Clear permission data for test isolation
	if err := clearPermissionData(ctx); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to clear permission data: %v", err)}
	}

	// Create test employees
	employees := []models.EmployeeDepartment{
		{
			EmployeeNumber:     "qc_test001",
			Username:           "user_no_quota_settings",
			DeptFullLevelNames: "Tech_Group,Testing_Dept",
		},
		{
			EmployeeNumber:     "qc_test002",
			Username:           "user_with_quota_settings",
			DeptFullLevelNames: "Tech_Group,Testing_Dept",
		},
	}

	for _, emp := range employees {
		if err := ctx.DB.DB.Create(&emp).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
		}
	}

	// Clear quota check calls
	mockStore.ClearQuotaCheckCalls()

	// Scenario 1: New user with default setting - should NOT notify Higress
	if err := quotaCheckPermissionService.UpdateEmployeeQuotaCheckPermissions("qc_test001"); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to update quota check permissions for qc_test001: %v", err)}
	}

	calls := mockStore.GetQuotaCheckCalls()
	if len(calls) != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Scenario 1 failed: Expected 0 calls for new user with default setting, got %d", len(calls))}
	}

	// Scenario 2: New user gets enabled setting - should notify Higress (change from default false to true)
	mockStore.ClearQuotaCheckCalls()
	// Create auth mapping and use UUID for user-level setting under EmployeeSync=true
	uid2, errUID2 := createAuthUserForEmployee(ctx, "qc_test002", employees[1].Username)
	if errUID2 != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create auth user for qc_test002: %v", errUID2)}
	}
	if err := quotaCheckPermissionService.SetUserQuotaCheckSetting(uid2, true); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set quota check setting for qc_test002: %v", err)}
	}

	calls = mockStore.GetQuotaCheckCalls()
	if len(calls) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Scenario 2 failed: Expected 1 call for new user with enabled setting, got %d", len(calls))}
	}
	if calls[0].EmployeeNumber != "qc_test002" || !calls[0].Enabled {
		return TestResult{Passed: false, Message: "Scenario 2 failed: Incorrect Higress call content"}
	}

	// Scenario 3: Existing user setting changes - should notify Higress (change from true to false)
	mockStore.ClearQuotaCheckCalls()
	if err := quotaCheckPermissionService.SetUserQuotaCheckSetting(uid2, false); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to update quota check setting for qc_test002: %v", err)}
	}

	calls = mockStore.GetQuotaCheckCalls()
	if len(calls) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Scenario 3 failed: Expected 1 call for setting change, got %d", len(calls))}
	}
	if calls[0].Enabled {
		return TestResult{Passed: false, Message: "Scenario 3 failed: Expected disabled setting"}
	}

	// Scenario 4: User setting doesn't change - should NOT notify Higress
	mockStore.ClearQuotaCheckCalls()
	if err := quotaCheckPermissionService.UpdateEmployeeQuotaCheckPermissions("qc_test002"); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to update quota check permissions for qc_test002: %v", err)}
	}

	calls = mockStore.GetQuotaCheckCalls()
	if len(calls) != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Scenario 4 failed: Expected 0 calls for unchanged setting, got %d", len(calls))}
	}

	return TestResult{Passed: true, Message: "Quota check notification optimization test succeeded"}
}

// testUserQuotaCheckSettingDistribution tests user quota check setting distribution to Higress
func testUserQuotaCheckSettingDistribution(ctx *TestContext) TestResult {
	// Clear quota check data to avoid interference from previous tests
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

	quotaCheckPermissionService := services.NewQuotaCheckPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

	// Create test employee
	employee := &models.EmployeeDepartment{
		EmployeeNumber:     "320001",
		Username:           "quota_dist_test_user",
		DeptFullLevelNames: "Tech_Group,R&D_Center,Mobile_Dev_Dept,Mobile_Dev_Team1",
	}
	if err := ctx.DB.DB.Create(employee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
	}

	// Create auth mapping and use UUID for user-level operation under EmployeeSync=true
	uid, errUID := createAuthUserForEmployee(ctx, employee.EmployeeNumber, employee.Username)
	if errUID != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create auth user for %s: %v", employee.EmployeeNumber, errUID)}
	}

	// Clear previous quota check calls
	mockStore.ClearQuotaCheckCalls()

	// Set user quota check setting using UUID - this should trigger Higress sync
	if err := quotaCheckPermissionService.SetUserQuotaCheckSetting(uid, false); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set user quota check setting: %v", err)}
	}

	// Verify quota check setting was distributed to Higress
	quotaCheckCalls := mockStore.GetQuotaCheckCalls()
	if len(quotaCheckCalls) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 quota check call, got %d", len(quotaCheckCalls))}
	}

	call := quotaCheckCalls[0]
	if call.EmployeeNumber != "320001" {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected employee number 320001, got %s", call.EmployeeNumber)}
	}

	if call.Operation != "set" {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected operation 'set', got %s", call.Operation)}
	}

	if call.Enabled {
		return TestResult{Passed: false, Message: "Expected quota check to be disabled in Higress call"}
	}

	// Verify effective setting in database
	var effectiveSetting models.EffectiveQuotaCheckSetting
	if err := ctx.DB.DB.Where("employee_number = ?", "320001").First(&effectiveSetting).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to find effective quota check setting: %v", err)}
	}

	if effectiveSetting.Enabled {
		return TestResult{Passed: false, Message: "Expected effective quota check setting to be disabled"}
	}

	return TestResult{Passed: true, Message: "User quota check setting distribution test succeeded"}
}

// testDepartmentQuotaCheckSettingDistribution tests department quota check setting distribution
func testDepartmentQuotaCheckSettingDistribution(ctx *TestContext) TestResult {
	// Clear quota check data to avoid interference from previous tests
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

	quotaCheckPermissionService := services.NewQuotaCheckPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

	// Create test employees in the same department
	employees := []models.EmployeeDepartment{
		{
			EmployeeNumber:     "321002",
			Username:           "dept_quota_test_user1",
			DeptFullLevelNames: "Tech_Group,R&D_Center,Backend_Dev_Dept,Backend_Dev_Team1",
		},
		{
			EmployeeNumber:     "321003",
			Username:           "dept_quota_test_user2",
			DeptFullLevelNames: "Tech_Group,R&D_Center,Backend_Dev_Dept,Backend_Dev_Team2",
		},
		{
			EmployeeNumber:     "321004",
			Username:           "dept_quota_test_user3",
			DeptFullLevelNames: "Tech_Group,R&D_Center,Backend_Dev_Dept,Backend_Dev_Team3",
		},
	}

	for _, emp := range employees {
		if err := ctx.DB.DB.Create(&emp).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
		}
	}

	// Clear previous quota check calls
	mockStore.ClearQuotaCheckCalls()

	// Set department quota check setting - this should trigger Higress sync for all employees
	if err := quotaCheckPermissionService.SetDepartmentQuotaCheckSetting("Backend_Dev_Dept", false); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set department quota check setting: %v", err)}
	}

	// Verify the settings were synced to Higress for all employees
	quotaCheckCalls := mockStore.GetQuotaCheckCalls()
	if len(quotaCheckCalls) != 3 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 3 quota check calls to Higress, got %d", len(quotaCheckCalls))}
	}

	// Verify each call
	calledEmployees := make(map[string]bool)
	for _, call := range quotaCheckCalls {
		calledEmployees[call.EmployeeNumber] = true
		if call.Enabled {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected quota check to be disabled for employee %s", call.EmployeeNumber)}
		}
	}

	// Verify all expected employees were called
	expectedEmployees := []string{"321002", "321003", "321004"}
	for _, empNum := range expectedEmployees {
		if !calledEmployees[empNum] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected Higress call for employee %s", empNum)}
		}
	}

	// Verify effective settings in database for all employees
	for _, empNum := range expectedEmployees {
		var effectiveSetting models.EffectiveQuotaCheckSetting
		if err := ctx.DB.DB.Where("employee_number = ?", empNum).First(&effectiveSetting).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Failed to find effective quota check setting for employee %s: %v", empNum, err)}
		}

		if effectiveSetting.Enabled {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected effective quota check setting to be disabled for employee %s", empNum)}
		}
	}

	return TestResult{Passed: true, Message: "Department quota check setting distribution test succeeded"}
}

// testQuotaCheckSettingHierarchyLevel1 tests quota check setting hierarchy with 1 level difference
func testQuotaCheckSettingHierarchyLevel1(ctx *TestContext) TestResult {
	return testQuotaCheckSettingHierarchyWithLevels(ctx, 1)
}

// testQuotaCheckSettingHierarchyLevel2 tests quota check setting hierarchy with 2 level difference
func testQuotaCheckSettingHierarchyLevel2(ctx *TestContext) TestResult {
	return testQuotaCheckSettingHierarchyWithLevels(ctx, 2)
}

// testQuotaCheckSettingHierarchyLevel3 tests quota check setting hierarchy with 3 level difference
func testQuotaCheckSettingHierarchyLevel3(ctx *TestContext) TestResult {
	return testQuotaCheckSettingHierarchyWithLevels(ctx, 3)
}

// testQuotaCheckSettingHierarchyLevel5 tests quota check setting hierarchy with 5 level difference
func testQuotaCheckSettingHierarchyLevel5(ctx *TestContext) TestResult {
	return testQuotaCheckSettingHierarchyWithLevels(ctx, 5)
}

// testQuotaCheckSettingHierarchyWithLevels tests quota check setting hierarchy with specified level difference
func testQuotaCheckSettingHierarchyWithLevels(ctx *TestContext, levelDiff int) TestResult {
	// Clear quota check data to avoid interference from previous tests
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

	quotaCheckPermissionService := services.NewQuotaCheckPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

	// Create hierarchical department structure based on level difference
	var parentDept, childDept string
	var employeeNumber string

	switch levelDiff {
	case 1:
		parentDept = "R&D_Center"
		childDept = "QuotaCheck_Dev_Dept"
		employeeNumber = "330010"
	case 2:
		parentDept = "R&D_Center"
		childDept = "QuotaCheck_Dev_Dept_Team1"
		employeeNumber = "330011"
	case 3:
		parentDept = "R&D_Center"
		childDept = "QuotaCheck_Dev_Dept_Team1_SubTeam"
		employeeNumber = "330012"
	case 5:
		parentDept = "Tech_Group"
		childDept = "QuotaCheck_Dev_Dept_Team1_SubTeam_Alpha"
		employeeNumber = "330013"
	default:
		return TestResult{Passed: false, Message: fmt.Sprintf("Unsupported level difference: %d", levelDiff)}
	}

	// Create department hierarchy path
	var deptPath string
	switch levelDiff {
	case 1:
		deptPath = fmt.Sprintf("Tech_Group,R&D_Center,%s", childDept)
	case 2:
		deptPath = fmt.Sprintf("Tech_Group,R&D_Center,QuotaCheck_Dev_Dept,%s", childDept)
	case 3:
		deptPath = fmt.Sprintf("Tech_Group,R&D_Center,QuotaCheck_Dev_Dept,QuotaCheck_Dev_Dept_Team1,%s", childDept)
	case 5:
		deptPath = fmt.Sprintf("Tech_Group,R&D_Center,QuotaCheck_Dev_Dept,QuotaCheck_Dev_Dept_Team1,QuotaCheck_Dev_Dept_Team1_SubTeam,%s", childDept)
	}

	// Create test employee
	employee := &models.EmployeeDepartment{
		EmployeeNumber:     employeeNumber,
		Username:           fmt.Sprintf("quota_hierarchy_test_%d", levelDiff),
		DeptFullLevelNames: deptPath,
	}
	if err := ctx.DB.DB.Create(employee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
	}

	// Create corresponding auth user and use UUID for user-level queries under EmployeeSync=true
	authUser := &models.UserInfo{
		ID:             uuid.NewString(),
		CreatedAt:      time.Now().Add(-time.Hour),
		UpdatedAt:      time.Now(),
		AccessTime:     time.Now(),
		Name:           employee.Username,
		EmployeeNumber: employee.EmployeeNumber,
		GithubID:       fmt.Sprintf("test_%s_%d", employee.EmployeeNumber, time.Now().UnixNano()),
		GithubName:     employee.Username,
		Devices:        "{}",
	}
	if err := ctx.DB.AuthDB.Create(authUser).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create auth user: %v", err)}
	}

	// Set parent department quota check setting
	if err := quotaCheckPermissionService.SetDepartmentQuotaCheckSetting(parentDept, false); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set parent department quota check setting: %v", err)}
	}

	// Get effective quota check setting - should inherit from parent department
	enabled, err := quotaCheckPermissionService.GetUserEffectiveQuotaCheckSetting(authUser.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective quota check setting: %v", err)}
	}

	if enabled {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected quota check to be disabled (inherited from %s)", parentDept)}
	}

	// Set child department quota check setting (should override parent)
	if err := quotaCheckPermissionService.SetDepartmentQuotaCheckSetting(childDept, true); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set child department quota check setting: %v", err)}
	}

	// Get effective quota check setting - should use child department setting
	childEnabled, err := quotaCheckPermissionService.GetUserEffectiveQuotaCheckSetting(authUser.ID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective quota check setting after child dept update: %v", err)}
	}

	if !childEnabled {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected quota check to be enabled (overridden by %s)", childDept)}
	}

	return TestResult{Passed: true, Message: fmt.Sprintf("Quota check setting hierarchy level %d test succeeded", levelDiff)}
}

// testUserQuotaCheckSettingOverridesDepartment tests user quota check setting overriding department setting
func testUserQuotaCheckSettingOverridesDepartment(ctx *TestContext) TestResult {
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

	quotaCheckPermissionService := services.NewQuotaCheckPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

	// Create test employee
	employee := &models.EmployeeDepartment{
		EmployeeNumber:     "340020",
		Username:           "dept_override_test_employee",
		DeptFullLevelNames: "Tech_Group,R&D_Center,Mobile_Dev_Dept,Mobile_Dev_Team1",
	}
	if err := ctx.DB.DB.Create(employee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
	}

	// Create auth mapping and use UUID for user-level calls under EmployeeSync=true
	uid, errUID := createAuthUserForEmployee(ctx, employee.EmployeeNumber, employee.Username)
	if errUID != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create auth user for %s: %v", employee.EmployeeNumber, errUID)}
	}

	// Set department quota check setting to disabled
	if err := quotaCheckPermissionService.SetDepartmentQuotaCheckSetting("Mobile_Dev_Dept", false); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set department quota check setting: %v", err)}
	}

	// Set user quota check setting to enabled (should override department)
	if err := quotaCheckPermissionService.SetUserQuotaCheckSetting(uid, true); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set user quota check setting: %v", err)}
	}

	// Verify employee has user setting (higher priority)
	enabled, err := quotaCheckPermissionService.GetUserEffectiveQuotaCheckSetting(uid)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective quota check setting: %v", err)}
	}

	if !enabled {
		return TestResult{Passed: false, Message: "Expected user setting (enabled) to override department setting (disabled)"}
	}

	return TestResult{Passed: true, Message: "User quota check setting overrides department test succeeded"}
}
