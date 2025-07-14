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

	// Default employee sync config for compatibility
	defaultEmployeeSyncConfig := &config.EmployeeSyncConfig{
		Enabled: false, // Default to disabled for existing tests
		HrURL:   "http://localhost:8099/api/hr/employees",
		HrKey:   "test-hr-key",
		DeptURL: "http://localhost:8099/api/hr/departments",
		DeptKey: "test-dept-key",
	}

	// Create services
	permissionService := services.NewPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

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

	// Default employee sync config for compatibility
	defaultEmployeeSyncConfig := &config.EmployeeSyncConfig{
		Enabled: false, // Default to disabled for existing tests
		HrURL:   "http://localhost:8099/api/hr/employees",
		HrKey:   "test-hr-key",
		DeptURL: "http://localhost:8099/api/hr/departments",
		DeptKey: "test-dept-key",
	}

	// Create services
	permissionService := services.NewPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

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

	// Default employee sync config for compatibility
	defaultEmployeeSyncConfig := &config.EmployeeSyncConfig{
		Enabled: false, // Default to disabled for existing tests
		HrURL:   "http://localhost:8099/api/hr/employees",
		HrKey:   "test-hr-key",
		DeptURL: "http://localhost:8099/api/hr/departments",
		DeptKey: "test-dept-key",
	}

	// Create services
	permissionService := services.NewPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

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

	// Default employee sync config for compatibility
	defaultEmployeeSyncConfig := &config.EmployeeSyncConfig{
		Enabled: false, // Default to disabled for existing tests
		HrURL:   "http://localhost:8099/api/hr/employees",
		HrKey:   "test-hr-key",
		DeptURL: "http://localhost:8099/api/hr/departments",
		DeptKey: "test-dept-key",
	}

	// Create services
	permissionService := services.NewPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

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
	// Default employee sync config for compatibility

	defaultEmployeeSyncConfig := &config.EmployeeSyncConfig{
		Enabled: false, // Default to disabled for existing tests
		HrURL:   "http://localhost:8099/api/hr/employees",
		HrKey:   "test-hr-key",
		DeptURL: "http://localhost:8099/api/hr/departments",
		DeptKey: "test-dept-key",
	}

	permissionService := services.NewPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)
	starCheckPermissionService := services.NewStarCheckPermissionService(ctx.DB, aiGatewayConfig, employeeSyncConfig, ctx.Gateway)
	employeeSyncService := services.NewEmployeeSyncService(ctx.DB, employeeSyncConfig, permissionService, starCheckPermissionService)

	// Clear any previous permission calls from earlier tests
	// Clear permission data for test isolation
	if err := clearPermissionData(ctx); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to clear permission data: %v", err)}
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
	// Default employee sync config for compatibility
	defaultEmployeeSyncConfig := &config.EmployeeSyncConfig{
		Enabled: false, // Default to disabled for existing tests
		HrURL:   "http://localhost:8099/api/hr/employees",
		HrKey:   "test-hr-key",
		DeptURL: "http://localhost:8099/api/hr/departments",
		DeptKey: "test-dept-key",
	}

	permissionService := services.NewPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

	// Clear permission data for test isolation
	if err := clearPermissionData(ctx); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to clear permission data: %v", err)}
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

	// Default employee sync config for compatibility

	defaultEmployeeSyncConfig := &config.EmployeeSyncConfig{

		Enabled: false, // Default to disabled for existing tests

		HrURL: "http://localhost:8099/api/hr/employees",

		HrKey: "test-hr-key",

		DeptURL: "http://localhost:8099/api/hr/departments",

		DeptKey: "test-dept-key",
	}

	permissionService := services.NewPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

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

// testEmptyWhitelistFallback tests that empty whitelists are treated as "not configured"
// and the system falls back to parent level permissions
func testEmptyWhitelistFallback(ctx *TestContext) TestResult {
	// Create services
	aiGatewayConfig := &config.AiGatewayConfig{
		Host:       "localhost",
		Port:       8080,
		AdminPath:  "/model-permission",
		AuthHeader: "x-admin-key",
		AuthValue:  "test-key",
	}

	employeeSyncConfig := &config.EmployeeSyncConfig{
		Enabled: false,
		HrURL:   "http://localhost:8099/api/hr/employees",
		HrKey:   "test-hr-key",
		DeptURL: "http://localhost:8099/api/hr/departments",
		DeptKey: "test-dept-key",
	}

	permissionService := services.NewPermissionService(ctx.DB, aiGatewayConfig, employeeSyncConfig, ctx.Gateway)

	// Test Case 1: Personal empty whitelist should fallback to department whitelist
	employee1 := &models.EmployeeDepartment{
		EmployeeNumber:     "E001",
		Username:           "test_user_1",
		DeptFullLevelNames: "Company,Tech_Group,Dev_Team",
	}
	if err := ctx.DB.DB.Create(employee1).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee1: %v", err)}
	}

	// Set department whitelist first
	if err := permissionService.SetDepartmentWhitelist("Dev_Team", []string{"gpt-4", "claude-3"}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set department whitelist: %v", err)}
	}

	// Set personal empty whitelist
	if err := permissionService.SetUserWhitelist("E001", []string{}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set empty user whitelist: %v", err)}
	}

	// Check effective permissions - should fallback to department whitelist
	effectiveModels, err := permissionService.GetUserEffectivePermissions("E001")
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective permissions: %v", err)}
	}

	if len(effectiveModels) != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 models from department fallback, got %d: %v", len(effectiveModels), effectiveModels)}
	}

	expectedModels := map[string]bool{"gpt-4": true, "claude-3": true}
	for _, model := range effectiveModels {
		if !expectedModels[model] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Unexpected model %s in fallback permissions", model)}
		}
	}

	// Test Case 2: Sub-department empty whitelist should fallback to parent department
	employee2 := &models.EmployeeDepartment{
		EmployeeNumber:     "E002",
		Username:           "test_user_2",
		DeptFullLevelNames: "Company,Sales_Group,Marketing_Team,Digital_Team",
	}
	if err := ctx.DB.DB.Create(employee2).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee2: %v", err)}
	}

	// Set parent department whitelist
	if err := permissionService.SetDepartmentWhitelist("Sales_Group", []string{"gpt-3.5", "deepseek-v3"}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set parent department whitelist: %v", err)}
	}

	// Set sub-department empty whitelist
	if err := permissionService.SetDepartmentWhitelist("Digital_Team", []string{}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set empty sub-department whitelist: %v", err)}
	}

	// Check effective permissions - should fallback to parent department
	effectiveModels2, err := permissionService.GetUserEffectivePermissions("E002")
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective permissions for E002: %v", err)}
	}

	if len(effectiveModels2) != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 models from parent department fallback, got %d: %v", len(effectiveModels2), effectiveModels2)}
	}

	expectedModels2 := map[string]bool{"gpt-3.5": true, "deepseek-v3": true}
	for _, model := range effectiveModels2 {
		if !expectedModels2[model] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Unexpected model %s in parent department fallback", model)}
		}
	}

	// Test Case 3: Multiple level fallback
	employee3 := &models.EmployeeDepartment{
		EmployeeNumber:     "E003",
		Username:           "test_user_3",
		DeptFullLevelNames: "RootCompany,Operations_Group,Support_Team,Level2_Team,Level3_Team",
	}
	if err := ctx.DB.DB.Create(employee3).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee3: %v", err)}
	}

	// Set root department whitelist
	if err := permissionService.SetDepartmentWhitelist("RootCompany", []string{"llama-3", "qwen-2"}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set root department whitelist: %v", err)}
	}

	// Set all intermediate departments to empty
	if err := permissionService.SetDepartmentWhitelist("Level3_Team", []string{}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set empty Level3_Team whitelist: %v", err)}
	}
	if err := permissionService.SetDepartmentWhitelist("Level2_Team", []string{}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set empty Level2_Team whitelist: %v", err)}
	}
	if err := permissionService.SetDepartmentWhitelist("Support_Team", []string{}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set empty Support_Team whitelist: %v", err)}
	}
	if err := permissionService.SetDepartmentWhitelist("Operations_Group", []string{}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set empty Operations_Group whitelist: %v", err)}
	}

	// Check effective permissions - should fallback to root company level
	effectiveModels3, err := permissionService.GetUserEffectivePermissions("E003")
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective permissions for E003: %v", err)}
	}

	if len(effectiveModels3) != 2 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 2 models from root department fallback, got %d: %v", len(effectiveModels3), effectiveModels3)}
	}

	expectedModels3 := map[string]bool{"llama-3": true, "qwen-2": true}
	for _, model := range effectiveModels3 {
		if !expectedModels3[model] {
			return TestResult{Passed: false, Message: fmt.Sprintf("Unexpected model %s in root department fallback", model)}
		}
	}

	// Test Case 4: Verify that non-empty whitelists still take priority
	employee4 := &models.EmployeeDepartment{
		EmployeeNumber:     "E004",
		Username:           "test_user_4",
		DeptFullLevelNames: "TestCompany,HR_Group,Recruitment_Team",
	}
	if err := ctx.DB.DB.Create(employee4).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee4: %v", err)}
	}

	// Set department whitelist first
	if err := permissionService.SetDepartmentWhitelist("Recruitment_Team", []string{"gpt-3", "gemini-pro"}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set department whitelist for E004: %v", err)}
	}

	// Set personal non-empty whitelist (should override department)
	if err := permissionService.SetUserWhitelist("E004", []string{"claude-3-opus"}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set user whitelist for E004: %v", err)}
	}

	// Check effective permissions - should be personal whitelist, not department fallback
	effectiveModels4, err := permissionService.GetUserEffectivePermissions("E004")
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective permissions for E004: %v", err)}
	}

	if len(effectiveModels4) != 1 || effectiveModels4[0] != "claude-3-opus" {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected personal whitelist [claude-3-opus], got %v", effectiveModels4)}
	}

	return TestResult{Passed: true, Message: "Empty whitelist fallback logic works correctly"}
}
