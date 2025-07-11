package main

import (
	"fmt"
	"quota-manager/internal/config"
	"quota-manager/internal/models"
	"quota-manager/internal/services"
)

// testUserWhitelistManagement tests user whitelist management
func testUserWhitelistManagement(ctx *TestContext) TestResult {
	// Create a mock config for testing
	aiGatewayConfig := &config.AiGatewayConfig{
		Host:       "localhost",
		Port:       8080,
		AdminPath:  "/model-permission",
		AuthHeader: "x-admin-key",
		AuthValue:  "test-key",
	}

	// Create services
	permissionService := services.NewPermissionService(ctx.DB, aiGatewayConfig, ctx.Gateway)

	// Create test employees for this test
	targetEmployee := &models.EmployeeDepartment{
		EmployeeNumber:     "100001",
		Username:           "user_whitelist_test_employee",
		DeptFullLevelNames: "Tech_Group,R&D_Center,Testing_Dept",
	}
	if err := ctx.DB.DB.Create(targetEmployee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create target employee: %v", err)}
	}

	// Create another employee to verify isolation
	otherEmployee := &models.EmployeeDepartment{
		EmployeeNumber:     "100002",
		Username:           "other_test_employee",
		DeptFullLevelNames: "Tech_Group,R&D_Center,Development_Dept",
	}
	if err := ctx.DB.DB.Create(otherEmployee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create other employee: %v", err)}
	}

	// Test: Set user whitelist for target employee only
	if err := permissionService.SetUserWhitelist("100001", []string{"gpt-4", "claude-3-opus"}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set user whitelist: %v", err)}
	}

	// Verify the target employee has correct whitelist
	effectiveModels, err := permissionService.GetUserEffectivePermissions("100001")
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get target employee permissions: %v", err)}
	}

	if len(effectiveModels) != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 models for target employee, got %d", len(effectiveModels))}
	}

	// Verify the correct models are set for target employee
	expectedModels := map[string]bool{"gpt-4": true, "claude-3-opus": true}
	for _, model := range effectiveModels {
		if !expectedModels[model] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Unexpected model %s in target employee whitelist", model)}
		}
	}

	// Verify the other employee is NOT affected (should have no permissions)
	otherModels, err := permissionService.GetUserEffectivePermissions("100002")
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get other employee permissions: %v", err)}
	}

	if len(otherModels) != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 0 models for other employee, got %d", len(otherModels))}
	}

	return TestResult{Passed: true, Message: "User whitelist management test succeeded"}
}

// testDepartmentWhitelistManagement tests department whitelist management
func testDepartmentWhitelistManagement(ctx *TestContext) TestResult {
	// Create a mock config for testing
	aiGatewayConfig := &config.AiGatewayConfig{
		Host:       "localhost",
		Port:       8080,
		AdminPath:  "/model-permission",
		AuthHeader: "x-admin-key",
		AuthValue:  "test-key",
	}

	// Create services
	permissionService := services.NewPermissionService(ctx.DB, aiGatewayConfig, ctx.Gateway)

	// Create test employees - one in target department, one in different department
	targetEmployee := &models.EmployeeDepartment{
		EmployeeNumber:     "002001",
		Username:           "dept_test_employee",
		DeptFullLevelNames: "Tech_Group,R&D_Center,Backend_Dev_Dept",
	}
	if err := ctx.DB.DB.Create(targetEmployee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create target employee: %v", err)}
	}

	// Create employee in a different department to verify isolation
	otherDeptEmployee := &models.EmployeeDepartment{
		EmployeeNumber:     "002002",
		Username:           "other_dept_employee",
		DeptFullLevelNames: "Tech_Group,Product_Center,Product_Design_Dept",
	}
	if err := ctx.DB.DB.Create(otherDeptEmployee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create other department employee: %v", err)}
	}

	// Test: Set department whitelist for "R&D_Center" only
	if err := permissionService.SetDepartmentWhitelist("R&D_Center", []string{"gpt-3.5-turbo", "deepseek-v3"}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set department whitelist: %v", err)}
	}

	// Verify the target employee (in R&D_Center) has correct whitelist
	effectiveModels, err := permissionService.GetUserEffectivePermissions("002001")
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get target employee permissions: %v", err)}
	}

	if len(effectiveModels) != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 models for target employee, got %d", len(effectiveModels))}
	}

	// Verify the correct models are set for target employee
	expectedModels := map[string]bool{"gpt-3.5-turbo": true, "deepseek-v3": true}
	for _, model := range effectiveModels {
		if !expectedModels[model] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Unexpected model %s in target employee whitelist", model)}
		}
	}

	// Verify the employee in different department (Product_Center) is NOT affected
	otherModels, err := permissionService.GetUserEffectivePermissions("002002")
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get other department employee permissions: %v", err)}
	}

	if len(otherModels) != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 0 models for other department employee, got %d", len(otherModels))}
	}

	return TestResult{Passed: true, Message: "Department whitelist management test succeeded"}
}

// testPermissionPriorityAndInheritance tests permission priority and inheritance
func testPermissionPriorityAndInheritance(ctx *TestContext) TestResult {
	// Create a mock config for testing
	aiGatewayConfig := &config.AiGatewayConfig{
		Host:       "localhost",
		Port:       8080,
		AdminPath:  "/model-permission",
		AuthHeader: "x-admin-key",
		AuthValue:  "test-key",
	}

	// Create services
	permissionService := services.NewPermissionService(ctx.DB, aiGatewayConfig, ctx.Gateway)

	// Create test employee for this test
	employee := &models.EmployeeDepartment{
		EmployeeNumber:     "100003",
		Username:           "permission_priority_test_employee",
		DeptFullLevelNames: "Tech_Group,Product_Center,Product_R&D_Dept",
	}
	if err := ctx.DB.DB.Create(employee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
	}

	// Set department whitelist (parent department)
	if err := permissionService.SetDepartmentWhitelist("Product_Center", []string{"gpt-3.5-turbo", "deepseek-v3"}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set department whitelist: %v", err)}
	}

	// Set user whitelist (should override department)
	if err := permissionService.SetUserWhitelist("100003", []string{"gpt-4", "claude-3-opus"}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set user whitelist: %v", err)}
	}

	// Get effective permissions - should be user permissions (higher priority)
	models, err := permissionService.GetUserEffectivePermissions("100003")
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective permissions: %v", err)}
	}

	// Should have user permissions (gpt-4, claude-3-opus), not department permissions
	if len(models) != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 models, got %d", len(models))}
	}

	// Verify the specific values - should be user whitelist models, NOT department models
	expectedUserModels := map[string]bool{"gpt-4": true, "claude-3-opus": true}
	unexpectedDeptModels := map[string]bool{"gpt-3.5-turbo": true, "deepseek-v3": true}

	for _, model := range models {
		if !expectedUserModels[model] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Unexpected model %s - should be user permissions", model)}
		}
		if unexpectedDeptModels[model] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Found department model %s - user permissions should override", model)}
		}
	}

	// Ensure all expected user models are present
	modelSet := make(map[string]bool)
	for _, model := range models {
		modelSet[model] = true
	}

	if !modelSet["gpt-4"] || !modelSet["claude-3-opus"] {
		return TestResult{Passed: false, Message: fmt.Sprintf("Missing expected user models. Got: %v, Expected: gpt-4, claude-3-opus", models)}
	}

	return TestResult{Passed: true, Message: "Permission priority and inheritance test succeeded"}
}

// testAigatewayPermissionSync tests synchronization with aigateway
func testAigatewayPermissionSync(ctx *TestContext) TestResult {
	// Create a mock config for testing
	aiGatewayConfig := &config.AiGatewayConfig{
		Host:       "localhost",
		Port:       8080,
		AdminPath:  "/model-permission",
		AuthHeader: "x-admin-key",
		AuthValue:  "test-key",
	}

	// Create services
	permissionService := services.NewPermissionService(ctx.DB, aiGatewayConfig, ctx.Gateway)

	// Create test employee for this test
	employee := &models.EmployeeDepartment{
		EmployeeNumber:     "100004",
		Username:           "aigateway_sync_test_employee",
		DeptFullLevelNames: "Tech_Group,R&D_Center,R&D_Dept2",
	}
	if err := ctx.DB.DB.Create(employee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
	}

	// Clear previous permission calls to isolate this test
	mockStore.ClearPermissionCalls()

	// Set user whitelist - this should trigger aigateway sync
	if err := permissionService.SetUserWhitelist("100004", []string{"gpt-4", "deepseek-v3"}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set user whitelist: %v", err)}
	}

	// Verify the permission was synced to aigateway (mock server)
	permissionCalls := mockStore.GetPermissionCalls()
	if len(permissionCalls) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 permission call to aigateway, got %d", len(permissionCalls))}
	}

	call := permissionCalls[0]
	if call.EmployeeNumber != "100004" {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected employee number 100004 in aigateway call, got %s", call.EmployeeNumber)}
	}

	if call.Operation != "set" {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected operation 'set' in aigateway call, got %s", call.Operation)}
	}

	if len(call.Models) != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 models in aigateway call, got %d", len(call.Models))}
	}

	// Verify the specific models were synced correctly
	expectedModels := map[string]bool{"gpt-4": true, "deepseek-v3": true}
	for _, model := range call.Models {
		if !expectedModels[model] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Unexpected model %s in aigateway sync", model)}
		}
	}

	// Verify the permission was also stored in database
	effectiveModels, err := permissionService.GetUserEffectivePermissions("100004")
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective permissions from database: %v", err)}
	}

	if len(effectiveModels) != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 effective models in database, got %d", len(effectiveModels))}
	}

	// Verify database and aigateway have same permissions
	dbModelSet := make(map[string]bool)
	for _, model := range effectiveModels {
		dbModelSet[model] = true
	}

	for _, model := range call.Models {
		if !dbModelSet[model] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Model %s in aigateway but not in database", model)}
		}
	}

	return TestResult{Passed: true, Message: "aigateway permission sync test succeeded"}
}

// testSyncWithoutWhitelist tests sync when no whitelist is configured
func testSyncWithoutWhitelist(ctx *TestContext) TestResult {
	// Create mock config
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

	// Create services
	permissionService := services.NewPermissionService(ctx.DB, aiGatewayConfig, ctx.Gateway)
	employeeSyncService := services.NewEmployeeSyncService(ctx.DB, employeeSyncConfig, permissionService)

	// Clear any previous permission calls from earlier tests
	mockStore.ClearPermissionCalls()

	// Clear any existing employee department records from previous tests
	if err := ctx.DB.DB.Exec("DELETE FROM employee_department").Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to clear existing employee_department records: %v", err)}
	}

	// Clear any existing effective permissions from previous tests
	if err := ctx.DB.DB.Exec("DELETE FROM effective_permissions").Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to clear existing effective permissions: %v", err)}
	}

	// Clear any existing model whitelists from previous tests
	if err := ctx.DB.DB.Exec("DELETE FROM model_whitelist").Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to clear existing model whitelists: %v", err)}
	}

	// Setup minimal mock HR data for testing (even for "without whitelist" test we need some data)
	ClearMockData()
	SetupDefaultDepartmentHierarchy()
	AddMockEmployee("000001", "test_employee", "test@example.com", "13800000001", 4) // UX_Dept_Team1

	// Sync employees first
	if err := employeeSyncService.SyncEmployees(); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Employee sync failed: %v", err)}
	}

	// Verify NO permissions were sent to aigateway (no whitelist means no permissions and no notification needed)
	permissionCalls := mockStore.GetPermissionCalls()
	if len(permissionCalls) != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 0 permission calls for user without whitelist, got %d", len(permissionCalls))}
	}

	// Verify effective permissions exist but are empty (no models without whitelist)
	var effectivePermCount int64
	if err := ctx.DB.DB.Model(&models.EffectivePermission{}).Count(&effectivePermCount).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to count effective permissions: %v", err)}
	}

	if effectivePermCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 effective permission record, got %d", effectivePermCount)}
	}

	// Verify the effective permission record has no models
	var effectivePermission models.EffectivePermission
	if err := ctx.DB.DB.Where("employee_number = ?", "000001").First(&effectivePermission).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to find effective permissions: %v", err)}
	}

	effectiveModels := effectivePermission.GetEffectiveModelsAsSlice()
	if len(effectiveModels) != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected no effective models without whitelist, got %d models: %v", len(effectiveModels), effectiveModels)}
	}

	return TestResult{Passed: true, Message: "Sync without whitelist test succeeded"}
}

// testAigatewayNotificationOptimization tests that aigateway is only notified when permissions actually change
func testAigatewayNotificationOptimization(ctx *TestContext) TestResult {
	// Create mock config
	aiGatewayConfig := &config.AiGatewayConfig{
		Host:       "localhost",
		Port:       8080,
		AdminPath:  "/model-permission",
		AuthHeader: "x-admin-key",
		AuthValue:  "test-key",
	}

	// Create services
	permissionService := services.NewPermissionService(ctx.DB, aiGatewayConfig, ctx.Gateway)

	// Clear any existing data
	if err := ctx.DB.DB.Exec("DELETE FROM employee_department").Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to clear employee data: %v", err)}
	}
	if err := ctx.DB.DB.Exec("DELETE FROM effective_permissions").Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to clear effective permissions: %v", err)}
	}
	if err := ctx.DB.DB.Exec("DELETE FROM model_whitelist").Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to clear whitelists: %v", err)}
	}

	// Create test employees
	employees := []models.EmployeeDepartment{
		{
			EmployeeNumber:     "test001",
			Username:           "user_no_permissions",
			DeptFullLevelNames: "Tech_Group,Testing_Dept",
		},
		{
			EmployeeNumber:     "test002",
			Username:           "user_with_permissions",
			DeptFullLevelNames: "Tech_Group,Testing_Dept",
		},
	}

	for _, emp := range employees {
		if err := ctx.DB.DB.Create(&emp).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
		}
	}

	// Clear permission calls
	mockStore.ClearPermissionCalls()

	// Scenario 1: New user with no permissions - should NOT notify aigateway
	if err := permissionService.UpdateEmployeePermissions("test001"); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to update permissions for test001: %v", err)}
	}

	calls := mockStore.GetPermissionCalls()
	if len(calls) != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Scenario 1 failed: Expected 0 calls for new user with no permissions, got %d", len(calls))}
	}

	// Scenario 2: New user gets permissions - should notify aigateway
	mockStore.ClearPermissionCalls()
	if err := permissionService.SetUserWhitelist("test002", []string{"gpt-4"}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set whitelist for test002: %v", err)}
	}

	calls = mockStore.GetPermissionCalls()
	if len(calls) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Scenario 2 failed: Expected 1 call for new user with permissions, got %d", len(calls))}
	}
	if calls[0].EmployeeNumber != "test002" || len(calls[0].Models) != 1 || calls[0].Models[0] != "gpt-4" {
		return TestResult{Passed: false, Message: "Scenario 2 failed: Incorrect aigateway call content"}
	}

	// Scenario 3: Existing user permissions change - should notify aigateway
	mockStore.ClearPermissionCalls()
	if err := permissionService.SetUserWhitelist("test002", []string{"gpt-4", "claude-3"}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to update whitelist for test002: %v", err)}
	}

	calls = mockStore.GetPermissionCalls()
	if len(calls) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Scenario 3 failed: Expected 1 call for permission change, got %d", len(calls))}
	}
	if len(calls[0].Models) != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Scenario 3 failed: Expected 2 models, got %d", len(calls[0].Models))}
	}

	// Scenario 4: User permissions don't change - should NOT notify aigateway
	mockStore.ClearPermissionCalls()
	if err := permissionService.UpdateEmployeePermissions("test002"); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to update permissions for test002: %v", err)}
	}

	calls = mockStore.GetPermissionCalls()
	if len(calls) != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Scenario 4 failed: Expected 0 calls when permissions don't change, got %d", len(calls))}
	}

	// Scenario 5: User permissions cleared - should notify aigateway
	mockStore.ClearPermissionCalls()
	// Remove the user whitelist to clear permissions
	if err := ctx.DB.DB.Exec("DELETE FROM model_whitelist WHERE target_type = 'user' AND target_identifier = 'test002'").Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to clear user whitelist: %v", err)}
	}
	if err := permissionService.UpdateEmployeePermissions("test002"); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to update permissions for test002 after clearing whitelist: %v", err)}
	}

	calls = mockStore.GetPermissionCalls()
	if len(calls) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Scenario 5 failed: Expected 1 call to clear permissions, got %d", len(calls))}
	}
	if len(calls[0].Models) != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Scenario 5 failed: Expected empty models list to clear permissions, got %d models", len(calls[0].Models))}
	}

	return TestResult{Passed: true, Message: "Aigateway notification optimization test succeeded - all scenarios work correctly"}
}

// testUserWhitelistDistribution tests user whitelist distribution to AI gateway
func testUserWhitelistDistribution(ctx *TestContext) TestResult {
	// Create services
	aiGatewayConfig := &config.AiGatewayConfig{
		Host:       "localhost",
		Port:       8080,
		AdminPath:  "/model-permission",
		AuthHeader: "x-admin-key",
		AuthValue:  "test-key",
	}

	permissionService := services.NewPermissionService(ctx.DB, aiGatewayConfig, ctx.Gateway)

	// Create test employee
	employee := &models.EmployeeDepartment{
		EmployeeNumber:     "000001",
		Username:           "zhang_wei",
		DeptFullLevelNames: "Tech_Group,R&D_Center,Frontend_Dev_Dept,Frontend_Dev_Dept_Team1",
	}
	if err := ctx.DB.DB.Create(employee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
	}

	// Clear previous permission calls
	mockStore.ClearPermissionCalls()

	// Set user whitelist
	testModels := []string{"gpt-4", "claude-3-opus", "deepseek-v3"}
	if err := permissionService.SetUserWhitelist("000001", testModels); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set user whitelist: %v", err)}
	}

	// Verify permission was distributed to AI gateway
	permissionCalls := mockStore.GetPermissionCalls()
	if len(permissionCalls) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 permission call, got %d", len(permissionCalls))}
	}

	call := permissionCalls[0]
	if call.EmployeeNumber != "000001" {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected employee number 000001, got %s", call.EmployeeNumber)}
	}

	if call.Operation != "set" {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected operation 'set', got %s", call.Operation)}
	}

	if len(call.Models) != 3 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 3 models, got %d", len(call.Models))}
	}

	// Verify effective permissions in database
	var effectivePermission models.EffectivePermission
	if err := ctx.DB.DB.Where("employee_number = ?", "000001").First(&effectivePermission).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to find effective permissions: %v", err)}
	}

	if len(effectivePermission.GetEffectiveModelsAsSlice()) != 3 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 3 effective models, got %d", len(effectivePermission.GetEffectiveModelsAsSlice()))}
	}

	return TestResult{Passed: true, Message: "User whitelist distribution test succeeded"}
}

// testDepartmentWhitelistDistribution tests department whitelist distribution to AI gateway
func testDepartmentWhitelistDistribution(ctx *TestContext) TestResult {
	// Clear permission data to avoid interference from previous tests
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

	permissionService := services.NewPermissionService(ctx.DB, aiGatewayConfig, ctx.Gateway)

	// Create test employees in same department
	employees := []models.EmployeeDepartment{
		{
			EmployeeNumber:     "000002",
			Username:           "li_fang",
			DeptFullLevelNames: "Tech_Group,R&D_Center,Backend_Dev_Dept,Backend_Dev_Dept_Team1",
		},
		{
			EmployeeNumber:     "000003",
			Username:           "wang_na",
			DeptFullLevelNames: "Tech_Group,R&D_Center,Backend_Dev_Dept,Backend_Dev_Dept_Team1",
		},
		{
			EmployeeNumber:     "000004",
			Username:           "zhao_min",
			DeptFullLevelNames: "Tech_Group,R&D_Center,Backend_Dev_Dept,Backend_Dev_Dept_Team2",
		},
	}

	for _, emp := range employees {
		if err := ctx.DB.DB.Create(&emp).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
		}
	}

	// Clear previous permission calls
	mockStore.ClearPermissionCalls()

	// Set department whitelist
	testModels := []string{"gpt-3.5-turbo", "deepseek-v3"}
	if err := permissionService.SetDepartmentWhitelist("Backend_Dev_Dept", testModels); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set department whitelist: %v", err)}
	}

	// 1. Verify aigateway call completeness
	permissionCalls := mockStore.GetPermissionCalls()
	if len(permissionCalls) != 3 { // 3 employees in backend department
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 3 permission calls to aigateway, got %d", len(permissionCalls))}
	}

	// 2. Verify specific content of aigateway calls
	expectedEmployees := map[string]bool{"000002": true, "000003": true, "000004": true}
	expectedModels := map[string]bool{"gpt-3.5-turbo": true, "deepseek-v3": true}

	for _, call := range permissionCalls {
		// Verify operation type
		if call.Operation != "set" {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected operation 'set', got %s", call.Operation)}
		}

		// Verify employee number is within expected range
		if !expectedEmployees[call.EmployeeNumber] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Unexpected employee number %s in aigateway call", call.EmployeeNumber)}
		}

		// Verify model count
		if len(call.Models) != 2 {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 models for employee %s, got %d", call.EmployeeNumber, len(call.Models))}
		}

		// Verify model content is correct
		for _, model := range call.Models {
			if !expectedModels[model] {
				return TestResult{Passed: false, Message: fmt.Sprintf("Unexpected model %s for employee %s in aigateway call", model, call.EmployeeNumber)}
			}
		}
	}

	// 3. Verify department whitelist record in database
	var deptWhitelist models.ModelWhitelist
	if err := ctx.DB.DB.Where("target_type = ? AND target_identifier = ?", "department", "Backend_Dev_Dept").First(&deptWhitelist).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to find department whitelist in database: %v", err)}
	}

	// Verify specific content of department whitelist
	deptModelsInDB := deptWhitelist.GetAllowedModelsAsSlice()
	if len(deptModelsInDB) != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 models in department whitelist database, got %d", len(deptModelsInDB))}
	}

	dbDeptModelSet := make(map[string]bool)
	for _, model := range deptModelsInDB {
		dbDeptModelSet[model] = true
	}
	if !dbDeptModelSet["gpt-3.5-turbo"] || !dbDeptModelSet["deepseek-v3"] {
		return TestResult{Passed: false, Message: fmt.Sprintf("Department whitelist models incorrect in database. Got: %v", deptModelsInDB)}
	}

	// 4. Verify effective permission records for each employee in database
	for _, emp := range employees {
		var effectivePermission models.EffectivePermission
		if err := ctx.DB.DB.Where("employee_number = ?", emp.EmployeeNumber).First(&effectivePermission).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Failed to find effective permissions for employee %s in database: %v", emp.EmployeeNumber, err)}
		}

		// Verify specific content of effective permissions
		effectiveModelsInDB := effectivePermission.GetEffectiveModelsAsSlice()
		if len(effectiveModelsInDB) != 2 {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 effective models for employee %s in database, got %d", emp.EmployeeNumber, len(effectiveModelsInDB))}
		}

		// Verify effective permission models are correct
		dbEffectiveModelSet := make(map[string]bool)
		for _, model := range effectiveModelsInDB {
			dbEffectiveModelSet[model] = true
		}
		if !dbEffectiveModelSet["gpt-3.5-turbo"] || !dbEffectiveModelSet["deepseek-v3"] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Effective permissions incorrect for employee %s in database. Got: %v", emp.EmployeeNumber, effectiveModelsInDB)}
		}
	}

	// 5. Verify consistency between aigateway and database data
	for _, call := range permissionCalls {
		// Get effective permissions for this employee from database
		var effectivePermission models.EffectivePermission
		if err := ctx.DB.DB.Where("employee_number = ?", call.EmployeeNumber).First(&effectivePermission).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Failed to find database record for employee %s referenced in aigateway call: %v", call.EmployeeNumber, err)}
		}

		effectiveModelsInDB := effectivePermission.GetEffectiveModelsAsSlice()

		// Verify aigateway call models match database effective permissions
		if len(call.Models) != len(effectiveModelsInDB) {
			return TestResult{Passed: false, Message: fmt.Sprintf("Model count mismatch for employee %s: aigateway=%d, database=%d", call.EmployeeNumber, len(call.Models), len(effectiveModelsInDB))}
		}

		dbModelSet := make(map[string]bool)
		for _, model := range effectiveModelsInDB {
			dbModelSet[model] = true
		}

		for _, model := range call.Models {
			if !dbModelSet[model] {
				return TestResult{Passed: false, Message: fmt.Sprintf("Model %s found in aigateway call but not in database for employee %s", model, call.EmployeeNumber)}
			}
		}
	}

	return TestResult{Passed: true, Message: "Department whitelist distribution test succeeded with comprehensive verification"}
}

// testPermissionHierarchyLevel1 tests permission hierarchy with 1 level difference
func testPermissionHierarchyLevel1(ctx *TestContext) TestResult {
	return testPermissionHierarchyWithLevels(ctx, 1)
}

// testPermissionHierarchyLevel2 tests permission hierarchy with 2 level difference
func testPermissionHierarchyLevel2(ctx *TestContext) TestResult {
	return testPermissionHierarchyWithLevels(ctx, 2)
}

// testPermissionHierarchyLevel3 tests permission hierarchy with 3 level difference
func testPermissionHierarchyLevel3(ctx *TestContext) TestResult {
	return testPermissionHierarchyWithLevels(ctx, 3)
}

// testPermissionHierarchyLevel5 tests permission hierarchy with 5 level difference
func testPermissionHierarchyLevel5(ctx *TestContext) TestResult {
	return testPermissionHierarchyWithLevels(ctx, 5)
}

// testPermissionHierarchyWithLevels tests permission hierarchy with specified level difference
func testPermissionHierarchyWithLevels(ctx *TestContext, levelDiff int) TestResult {
	// Clear permission data to avoid interference from previous tests
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

	permissionService := services.NewPermissionService(ctx.DB, aiGatewayConfig, ctx.Gateway)

	// Create hierarchical department structure based on level difference
	var parentDept, childDept string
	var employeeNumber string

	switch levelDiff {
	case 1:
		parentDept = "R&D_Center"
		childDept = "Frontend_Dev_Dept"
		employeeNumber = "000010"
	case 2:
		parentDept = "R&D_Center"
		childDept = "Frontend_Dev_Dept_Team1"
		employeeNumber = "000011"
	case 3:
		parentDept = "R&D_Center"
		childDept = "Frontend_Dev_Dept_Team1_SubTeam"
		employeeNumber = "000012"
	case 5:
		parentDept = "Tech_Group"
		childDept = "Frontend_Dev_Dept_Team1_SubTeam_Alpha"
		employeeNumber = "000013"
	default:
		return TestResult{Passed: false, Message: fmt.Sprintf("Unsupported level difference: %d", levelDiff)}
	}

	// Create employee in child department
	employee := &models.EmployeeDepartment{
		EmployeeNumber:     employeeNumber,
		Username:           fmt.Sprintf("test_employee_%d", levelDiff),
		DeptFullLevelNames: "Tech_Group,R&D_Center,Frontend_Dev_Dept,Frontend_Dev_Dept_Team1,Frontend_Dev_Dept_Team1_SubTeam,Frontend_Dev_Dept_Team1_SubTeam_Alpha",
	}
	if err := ctx.DB.DB.Create(employee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
	}

	// Clear previous permission calls
	mockStore.ClearPermissionCalls()

	// Set parent department whitelist
	parentModels := []string{"gpt-3.5-turbo", "deepseek-v3"}
	if err := permissionService.SetDepartmentWhitelist(parentDept, parentModels); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set parent department whitelist: %v", err)}
	}

	// Set child department whitelist (should override parent)
	childModels := []string{"gpt-4", "claude-3-opus"}
	if err := permissionService.SetDepartmentWhitelist(childDept, childModels); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set child department whitelist: %v", err)}
	}

	// 1. Verify aigateway calls (should be 2 times: set parent dept once, set child dept once)
	permissionCalls := mockStore.GetPermissionCalls()
	if len(permissionCalls) != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 permission calls to aigateway (parent + child dept), got %d", len(permissionCalls))}
	}

	// 2. Verify specific content of last aigateway call (should be child department permissions)
	call := permissionCalls[len(permissionCalls)-1] // Get last call
	if call.Operation != "set" {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected operation 'set', got %s", call.Operation)}
	}

	if call.EmployeeNumber != employeeNumber {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected employee number %s, got %s in aigateway call", employeeNumber, call.EmployeeNumber)}
	}

	if len(call.Models) != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 models in aigateway call, got %d", len(call.Models))}
	}

	// Verify aigateway call models are child department permissions (high priority)
	expectedChildModels := map[string]bool{"gpt-4": true, "claude-3-opus": true}
	for _, model := range call.Models {
		if !expectedChildModels[model] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Unexpected model %s in aigateway call, should be child department models", model)}
		}
	}

	// 3. Verify parent department whitelist record in database
	var parentWhitelist models.ModelWhitelist
	if err := ctx.DB.DB.Where("target_type = ? AND target_identifier = ?", "department", parentDept).First(&parentWhitelist).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to find parent department whitelist in database: %v", err)}
	}

	parentModelsInDB := parentWhitelist.GetAllowedModelsAsSlice()
	if len(parentModelsInDB) != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 models in parent department whitelist database, got %d", len(parentModelsInDB))}
	}

	expectedParentModels := map[string]bool{"gpt-3.5-turbo": true, "deepseek-v3": true}
	for _, model := range parentModelsInDB {
		if !expectedParentModels[model] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Unexpected parent department model %s in database", model)}
		}
	}

	// 4. Verify child department whitelist record in database
	var childWhitelist models.ModelWhitelist
	if err := ctx.DB.DB.Where("target_type = ? AND target_identifier = ?", "department", childDept).First(&childWhitelist).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to find child department whitelist in database: %v", err)}
	}

	childModelsInDB := childWhitelist.GetAllowedModelsAsSlice()
	if len(childModelsInDB) != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 models in child department whitelist database, got %d", len(childModelsInDB))}
	}

	for _, model := range childModelsInDB {
		if !expectedChildModels[model] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Unexpected child department model %s in database", model)}
		}
	}

	// 5. Verify employee's effective permission record in database
	var effectivePermission models.EffectivePermission
	if err := ctx.DB.DB.Where("employee_number = ?", employeeNumber).First(&effectivePermission).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to find effective permissions for employee %s in database: %v", employeeNumber, err)}
	}

	effectiveModelsInDB := effectivePermission.GetEffectiveModelsAsSlice()
	if len(effectiveModelsInDB) != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 effective models for employee %s in database, got %d", employeeNumber, len(effectiveModelsInDB))}
	}

	// Verify effective permissions are child department permissions (permission inheritance and priority verification)
	for _, model := range effectiveModelsInDB {
		if !expectedChildModels[model] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Expected child department model in effective permissions, got %s", model)}
		}
	}

	// 6. Verify consistency between aigateway and database data
	if len(call.Models) != len(effectiveModelsInDB) {
		return TestResult{Passed: false, Message: fmt.Sprintf("Model count mismatch: aigateway=%d, database=%d", len(call.Models), len(effectiveModelsInDB))}
	}

	dbModelSet := make(map[string]bool)
	for _, model := range effectiveModelsInDB {
		dbModelSet[model] = true
	}

	for _, model := range call.Models {
		if !dbModelSet[model] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Model %s found in aigateway call but not in database effective permissions", model)}
		}
	}

	// 7. Verify permission priority logic (most critical verification)
	// Ensure employee gets child department permissions not parent department permissions
	effectiveModels, err := permissionService.GetUserEffectivePermissions(employeeNumber)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective permissions: %v", err)}
	}

	if len(effectiveModels) != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 effective models from service, got %d", len(effectiveModels))}
	}

	// Verify service returns child department models (not parent department models)
	for _, model := range effectiveModels {
		if !expectedChildModels[model] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Service returned wrong model %s, should be child department model", model)}
		}
		// Ensure it's not parent department models
		if expectedParentModels[model] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Service returned parent department model %s, child department should override", model)}
		}
	}

	// 8. Verify priority data integrity (verify system indeed has two different whitelist records)
	if parentWhitelist.ID == childWhitelist.ID {
		return TestResult{Passed: false, Message: "Parent and child department whitelist should be different records"}
	}

	if parentWhitelist.TargetIdentifier == childWhitelist.TargetIdentifier {
		return TestResult{Passed: false, Message: "Parent and child department should have different target identifiers"}
	}

	return TestResult{Passed: true, Message: fmt.Sprintf("Permission hierarchy level %d test succeeded with comprehensive verification", levelDiff)}
}

// testUserOverridesDepartment tests user whitelist overriding department whitelist
func testUserOverridesDepartment(ctx *TestContext) TestResult {
	// Create services
	aiGatewayConfig := &config.AiGatewayConfig{
		Host:       "localhost",
		Port:       8080,
		AdminPath:  "/model-permission",
		AuthHeader: "x-admin-key",
		AuthValue:  "test-key",
	}

	permissionService := services.NewPermissionService(ctx.DB, aiGatewayConfig, ctx.Gateway)

	// Create test employee
	employee := &models.EmployeeDepartment{
		EmployeeNumber:     "000020",
		Username:           "dept_test_employee",
		DeptFullLevelNames: "Tech_Group,R&D_Center,Mobile_Dev_Dept,Mobile_Dev_Dept_Team1",
	}
	if err := ctx.DB.DB.Create(employee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
	}

	// Set department whitelist
	deptModels := []string{"gpt-3.5-turbo", "deepseek-v3"}
	if err := permissionService.SetDepartmentWhitelist("Mobile_Dev_Dept", deptModels); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set department whitelist: %v", err)}
	}

	// Set user whitelist (should override department)
	userModels := []string{"gpt-4", "claude-3-opus", "gemini-pro"}
	if err := permissionService.SetUserWhitelist("000020", userModels); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set user whitelist: %v", err)}
	}

	// Verify employee has user permissions (higher priority)
	effectiveModels, err := permissionService.GetUserEffectivePermissions("000020")
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective permissions: %v", err)}
	}

	if len(effectiveModels) != 3 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 3 effective models, got %d", len(effectiveModels))}
	}

	// Verify models are from user whitelist
	expectedModels := map[string]bool{"gpt-4": true, "claude-3-opus": true, "gemini-pro": true}
	for _, model := range effectiveModels {
		if !expectedModels[model] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Unexpected model %s in effective permissions", model)}
		}
	}

	return TestResult{Passed: true, Message: "User overrides department test succeeded"}
}
