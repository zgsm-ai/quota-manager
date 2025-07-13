package main

import (
	"fmt"
	"quota-manager/internal/config"
	"quota-manager/internal/models"
	"quota-manager/internal/services"
)

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

	// Default employee sync config for compatibility

	defaultEmployeeSyncConfig := &config.EmployeeSyncConfig{

		Enabled: false, // Default to disabled for existing tests

		HrURL: "http://localhost:8099/api/hr/employees",

		HrKey: "test-hr-key",

		DeptURL: "http://localhost:8099/api/hr/departments",

		DeptKey: "test-dept-key",
	}

	permissionService := services.NewPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

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

	// Default employee sync config for compatibility

	defaultEmployeeSyncConfig := &config.EmployeeSyncConfig{

		Enabled: false, // Default to disabled for existing tests

		HrURL: "http://localhost:8099/api/hr/employees",

		HrKey: "test-hr-key",

		DeptURL: "http://localhost:8099/api/hr/departments",

		DeptKey: "test-dept-key",
	}

	permissionService := services.NewPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

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
