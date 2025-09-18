package main

import (
	"fmt"
	"quota-manager/internal/config"
	"quota-manager/internal/models"
	"quota-manager/internal/services"
)

// testUserQuotaCheckSettingManagement tests user quota check setting management
func testUserQuotaCheckSettingManagement(ctx *TestContext) TestResult {
	// Create a mock config for testing
	aiGatewayConfig := &config.AiGatewayConfig{
		Host:       "localhost",
		Port:       8080,
		AdminPath:  "/model-permission",
		AuthHeader: "x-admin-key",
		AuthValue:  "test-key",
	}

	// Default employee sync config for compatibility
	defaultEmployeeSyncConfig := &config.EmployeeSyncConfig{
		Enabled: true, // Enable employee sync; user-level ops will use UUID
		HrURL:   "http://localhost:8099/api/hr/employees",
		HrKey:   "test-hr-key",
		DeptURL: "http://localhost:8099/api/hr/departments",
		DeptKey: "test-dept-key",
	}

	// Create services
	quotaCheckPermissionService := services.NewQuotaCheckPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

	// Create test employees for this test
	targetEmployee := &models.EmployeeDepartment{
		EmployeeNumber:     "300001",
		Username:           "quota_check_test_employee",
		DeptFullLevelNames: "Tech_Group,R&D_Center,Testing_Dept",
	}
	if err := ctx.DB.DB.Create(targetEmployee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create target employee: %v", err)}
	}

	// Create another employee to verify isolation
	otherEmployee := &models.EmployeeDepartment{
		EmployeeNumber:     "300002",
		Username:           "other_quota_test_employee",
		DeptFullLevelNames: "Tech_Group,R&D_Center,Development_Dept",
	}
	if err := ctx.DB.DB.Create(otherEmployee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create other employee: %v", err)}
	}

	// Create auth mappings and use UUID for user-level operations
	uidTarget, errUID1 := createAuthUserForEmployee(ctx, targetEmployee.EmployeeNumber, targetEmployee.Username)
	if errUID1 != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create auth user for %s: %v", targetEmployee.EmployeeNumber, errUID1)}
	}
	uidOther, errUID2 := createAuthUserForEmployee(ctx, otherEmployee.EmployeeNumber, otherEmployee.Username)
	if errUID2 != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create auth user for %s: %v", otherEmployee.EmployeeNumber, errUID2)}
	}

	// Test: Set user quota check to enabled (use UUID)
	if err := quotaCheckPermissionService.SetUserQuotaCheckSetting(uidTarget, true); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set user quota check setting: %v", err)}
	}

	// Verify the target employee has correct setting
	enabled, err := quotaCheckPermissionService.GetUserEffectiveQuotaCheckSetting(uidTarget)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get target employee quota check setting: %v", err)}
	}

	if !enabled {
		return TestResult{Passed: false, Message: "Expected quota check to be enabled for target employee"}
	}

	// Verify the other employee is NOT affected (should have default disabled)
	otherEnabled, err := quotaCheckPermissionService.GetUserEffectiveQuotaCheckSetting(uidOther)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get other employee quota check setting: %v", err)}
	}

	if otherEnabled {
		return TestResult{Passed: false, Message: "Expected quota check to remain disabled for other employee"}
	}

	return TestResult{Passed: true, Message: "User quota check setting management test succeeded"}
}

// testDepartmentQuotaCheckSettingManagement tests department quota check setting management
func testDepartmentQuotaCheckSettingManagement(ctx *TestContext) TestResult {
	// Create a mock config for testing
	aiGatewayConfig := &config.AiGatewayConfig{
		Host:       "localhost",
		Port:       8080,
		AdminPath:  "/model-permission",
		AuthHeader: "x-admin-key",
		AuthValue:  "test-key",
	}

	// Default employee sync config for compatibility
	defaultEmployeeSyncConfig := &config.EmployeeSyncConfig{
		Enabled: true,
		HrURL:   "http://localhost:8099/api/hr/employees",
		HrKey:   "test-hr-key",
		DeptURL: "http://localhost:8099/api/hr/departments",
		DeptKey: "test-dept-key",
	}

	// Create services
	quotaCheckPermissionService := services.NewQuotaCheckPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

	// Create test employees - one in target department, one in different department
	targetEmployee := &models.EmployeeDepartment{
		EmployeeNumber:     "301001",
		Username:           "dept_quota_test_employee",
		DeptFullLevelNames: "Tech_Group,R&D_Center,Backend_Dev_Dept",
	}
	if err := ctx.DB.DB.Create(targetEmployee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create target employee: %v", err)}
	}

	// Create employee in a different department to verify isolation
	otherDeptEmployee := &models.EmployeeDepartment{
		EmployeeNumber:     "301002",
		Username:           "other_dept_quota_employee",
		DeptFullLevelNames: "Tech_Group,Product_Center,Product_Design_Dept",
	}
	if err := ctx.DB.DB.Create(otherDeptEmployee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create other department employee: %v", err)}
	}

	// Test: Set department quota check for "R&D_Center" to disabled
	if err := quotaCheckPermissionService.SetDepartmentQuotaCheckSetting("R&D_Center", false); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set department quota check setting: %v", err)}
	}

	// Set department quota check for "Product_Center" to enabled for comparison
	if err := quotaCheckPermissionService.SetDepartmentQuotaCheckSetting("Product_Center", true); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set Product_Center department quota check setting: %v", err)}
	}

	// Create auth mappings and verify settings via UUID
	uidTarget, errUID1 := createAuthUserForEmployee(ctx, targetEmployee.EmployeeNumber, targetEmployee.Username)
	if errUID1 != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create auth user for %s: %v", targetEmployee.EmployeeNumber, errUID1)}
	}
	uidOther, errUID2 := createAuthUserForEmployee(ctx, otherDeptEmployee.EmployeeNumber, otherDeptEmployee.Username)
	if errUID2 != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create auth user for %s: %v", otherDeptEmployee.EmployeeNumber, errUID2)}
	}

	// Verify the target employee (in R&D_Center) has disabled quota check
	enabled, err := quotaCheckPermissionService.GetUserEffectiveQuotaCheckSetting(uidTarget)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get target employee quota check setting: %v", err)}
	}

	if enabled {
		return TestResult{Passed: false, Message: "Expected quota check to be disabled for employee in R&D_Center"}
	}

	// Verify the other department employee is NOT affected
	otherEnabled, err := quotaCheckPermissionService.GetUserEffectiveQuotaCheckSetting(uidOther)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get other department employee quota check setting: %v", err)}
	}

	if !otherEnabled {
		return TestResult{Passed: false, Message: "Expected quota check to remain enabled for employee in Product_Center"}
	}

	return TestResult{Passed: true, Message: "Department quota check setting management test succeeded"}
}

// testQuotaCheckSettingPriorityAndInheritance tests quota check setting priority and inheritance
func testQuotaCheckSettingPriorityAndInheritance(ctx *TestContext) TestResult {
	// Create a mock config for testing
	aiGatewayConfig := &config.AiGatewayConfig{
		Host:       "localhost",
		Port:       8080,
		AdminPath:  "/model-permission",
		AuthHeader: "x-admin-key",
		AuthValue:  "test-key",
	}

	// Default employee sync config for compatibility
	defaultEmployeeSyncConfig := &config.EmployeeSyncConfig{
		Enabled: true,
		HrURL:   "http://localhost:8099/api/hr/employees",
		HrKey:   "test-hr-key",
		DeptURL: "http://localhost:8099/api/hr/departments",
		DeptKey: "test-dept-key",
	}

	// Create services
	quotaCheckPermissionService := services.NewQuotaCheckPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

	// Create test employee for this test
	employee := &models.EmployeeDepartment{
		EmployeeNumber:     "302003",
		Username:           "quota_priority_test_employee",
		DeptFullLevelNames: "Tech_Group,Product_Center,Product_R&D_Dept",
	}
	if err := ctx.DB.DB.Create(employee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
	}

	// Set department setting (parent department) to disabled
	if err := quotaCheckPermissionService.SetDepartmentQuotaCheckSetting("Product_Center", false); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set department quota check setting: %v", err)}
	}

	// Create auth mapping and set using UUID
	uid, errUID := createAuthUserForEmployee(ctx, employee.EmployeeNumber, employee.Username)
	if errUID != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create auth user for %s: %v", employee.EmployeeNumber, errUID)}
	}
	// Set user setting to enabled (should override department)
	if err := quotaCheckPermissionService.SetUserQuotaCheckSetting(uid, true); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set user quota check setting: %v", err)}
	}

	// Get effective setting - should be user setting (higher priority)
	enabled, err := quotaCheckPermissionService.GetUserEffectiveQuotaCheckSetting(uid)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective quota check setting: %v", err)}
	}

	if !enabled {
		return TestResult{Passed: false, Message: "Expected user setting (enabled) to override department setting (disabled)"}
	}

	return TestResult{Passed: true, Message: "Quota check setting priority and inheritance test succeeded"}
}

// testQuotaCheckPermissionDistribution tests quota check permission distribution to Higress
func testQuotaCheckPermissionDistribution(ctx *TestContext) TestResult {
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
		EmployeeNumber:     "303001",
		Username:           "quota_distribution_test",
		DeptFullLevelNames: "Tech_Group,R&D_Center,Frontend_Dev_Dept,Frontend_Dev_Dept_Team1",
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

	// Set user quota check setting using UUID
	if err := quotaCheckPermissionService.SetUserQuotaCheckSetting(uid, false); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set user quota check setting: %v", err)}
	}

	// Verify quota check setting was distributed to Higress
	quotaCheckCalls := mockStore.GetQuotaCheckCalls()
	if len(quotaCheckCalls) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 quota check call, got %d", len(quotaCheckCalls))}
	}

	call := quotaCheckCalls[0]
	if call.EmployeeNumber != "303001" {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected employee number 303001, got %s", call.EmployeeNumber)}
	}

	if call.Enabled {
		return TestResult{Passed: false, Message: "Expected quota check to be disabled in Higress call"}
	}

	return TestResult{Passed: true, Message: "Quota check permission distribution test succeeded"}
}

// testEmptyQuotaCheckSettingFallback tests that empty quota check settings fallback to default
func testEmptyQuotaCheckSettingFallback(ctx *TestContext) TestResult {
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
		HrURL:   "http://localhost:8099/api/hr/employees",
		HrKey:   "test-hr-key",
		DeptURL: "http://localhost:8099/api/hr/departments",
		DeptKey: "test-dept-key",
	}

	quotaCheckPermissionService := services.NewQuotaCheckPermissionService(ctx.DB, aiGatewayConfig, employeeSyncConfig, ctx.Gateway)

	// Test Case 1: User with no setting should fallback to default (disabled)
	employee1 := &models.EmployeeDepartment{
		EmployeeNumber:     "E301",
		Username:           "test_user_1",
		DeptFullLevelNames: "Company,Tech_Group,Dev_Team",
	}
	if err := ctx.DB.DB.Create(employee1).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee1: %v", err)}
	}

	// Create auth mapping and check effective setting via UUID - should be default (disabled)
	uid1, errUID1 := createAuthUserForEmployee(ctx, employee1.EmployeeNumber, employee1.Username)
	if errUID1 != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create auth user for %s: %v", employee1.EmployeeNumber, errUID1)}
	}
	enabled, err := quotaCheckPermissionService.GetUserEffectiveQuotaCheckSetting(uid1)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective quota check setting: %v", err)}
	}

	if enabled {
		return TestResult{Passed: false, Message: "Expected default quota check setting to be disabled"}
	}

	// Test Case 2: User with department setting should inherit from department
	employee2 := &models.EmployeeDepartment{
		EmployeeNumber:     "E302",
		Username:           "test_user_2",
		DeptFullLevelNames: "Company,Tech_Group,QA_Team",
	}
	if err := ctx.DB.DB.Create(employee2).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee2: %v", err)}
	}

	// Set department setting to enabled
	if err := quotaCheckPermissionService.SetDepartmentQuotaCheckSetting("QA_Team", true); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set department quota check setting: %v", err)}
	}

	// Create auth mapping for employee2 and check via UUID - should inherit from department
	uid2, errUID2 := createAuthUserForEmployee(ctx, employee2.EmployeeNumber, employee2.Username)
	if errUID2 != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create auth user for %s: %v", employee2.EmployeeNumber, errUID2)}
	}
	enabled2, err := quotaCheckPermissionService.GetUserEffectiveQuotaCheckSetting(uid2)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective quota check setting for employee2: %v", err)}
	}

	if !enabled2 {
		return TestResult{Passed: false, Message: "Expected quota check setting to inherit from department (enabled)"}
	}

	return TestResult{Passed: true, Message: "Empty quota check setting fallback test succeeded"}
}
