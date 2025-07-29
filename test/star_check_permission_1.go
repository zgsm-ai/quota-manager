package main

import (
	"fmt"
	"quota-manager/internal/config"
	"quota-manager/internal/models"
	"quota-manager/internal/services"
)

// testUserStarCheckSettingManagement tests user star check setting management
func testUserStarCheckSettingManagement(ctx *TestContext) TestResult {
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
	starCheckPermissionService := services.NewStarCheckPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

	// Create test employees for this test
	targetEmployee := &models.EmployeeDepartment{
		EmployeeNumber:     "200001",
		Username:           "star_check_test_employee",
		DeptFullLevelNames: "Tech_Group,R&D_Center,Testing_Dept",
	}
	if err := ctx.DB.DB.Create(targetEmployee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create target employee: %v", err)}
	}

	// Create another employee to verify isolation
	otherEmployee := &models.EmployeeDepartment{
		EmployeeNumber:     "200002",
		Username:           "other_star_test_employee",
		DeptFullLevelNames: "Tech_Group,R&D_Center,Development_Dept",
	}
	if err := ctx.DB.DB.Create(otherEmployee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create other employee: %v", err)}
	}

	// Test: Set user star check to enabled
	if err := starCheckPermissionService.SetUserStarCheckSetting("200001", true); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set user star check setting: %v", err)}
	}

	// Verify the target employee has correct setting
	enabled, err := starCheckPermissionService.GetUserEffectiveStarCheckSetting("200001")
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get target employee star check setting: %v", err)}
	}

	if !enabled {
		return TestResult{Passed: false, Message: "Expected star check to be enabled for target employee"}
	}

	// Verify the other employee is NOT affected (should have default disabled)
	otherEnabled, err := starCheckPermissionService.GetUserEffectiveStarCheckSetting("200002")
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get other employee star check setting: %v", err)}
	}

	if otherEnabled {
		return TestResult{Passed: false, Message: "Expected star check to remain disabled for other employee"}
	}

	return TestResult{Passed: true, Message: "User star check setting management test succeeded"}
}

// testDepartmentStarCheckSettingManagement tests department star check setting management
func testDepartmentStarCheckSettingManagement(ctx *TestContext) TestResult {
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
		Enabled: false,
		HrURL:   "http://localhost:8099/api/hr/employees",
		HrKey:   "test-hr-key",
		DeptURL: "http://localhost:8099/api/hr/departments",
		DeptKey: "test-dept-key",
	}

	// Create services
	starCheckPermissionService := services.NewStarCheckPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

	// Create test employees - one in target department, one in different department
	targetEmployee := &models.EmployeeDepartment{
		EmployeeNumber:     "201001",
		Username:           "dept_star_test_employee",
		DeptFullLevelNames: "Tech_Group,R&D_Center,Backend_Dev_Dept",
	}
	if err := ctx.DB.DB.Create(targetEmployee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create target employee: %v", err)}
	}

	// Create employee in a different department to verify isolation
	otherDeptEmployee := &models.EmployeeDepartment{
		EmployeeNumber:     "201002",
		Username:           "other_dept_star_employee",
		DeptFullLevelNames: "Tech_Group,Product_Center,Product_Design_Dept",
	}
	if err := ctx.DB.DB.Create(otherDeptEmployee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create other department employee: %v", err)}
	}

	// Test: Set department star check for "R&D_Center" to disabled
	if err := starCheckPermissionService.SetDepartmentStarCheckSetting("R&D_Center", false); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set department star check setting: %v", err)}
	}

	// Set department star check for "Product_Center" to enabled for comparison
	if err := starCheckPermissionService.SetDepartmentStarCheckSetting("Product_Center", true); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set Product_Center department star check setting: %v", err)}
	}

	// Verify the target employee (in R&D_Center) has disabled star check
	enabled, err := starCheckPermissionService.GetUserEffectiveStarCheckSetting("201001")
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get target employee star check setting: %v", err)}
	}

	if enabled {
		return TestResult{Passed: false, Message: "Expected star check to be disabled for employee in R&D_Center"}
	}

	// Verify the other department employee is NOT affected
	otherEnabled, err := starCheckPermissionService.GetUserEffectiveStarCheckSetting("201002")
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get other department employee star check setting: %v", err)}
	}

	if !otherEnabled {
		return TestResult{Passed: false, Message: "Expected star check to remain enabled for employee in Product_Center"}
	}

	return TestResult{Passed: true, Message: "Department star check setting management test succeeded"}
}

// testStarCheckSettingPriorityAndInheritance tests star check setting priority and inheritance
func testStarCheckSettingPriorityAndInheritance(ctx *TestContext) TestResult {
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
		Enabled: false,
		HrURL:   "http://localhost:8099/api/hr/employees",
		HrKey:   "test-hr-key",
		DeptURL: "http://localhost:8099/api/hr/departments",
		DeptKey: "test-dept-key",
	}

	// Create services
	starCheckPermissionService := services.NewStarCheckPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

	// Create test employee for this test
	employee := &models.EmployeeDepartment{
		EmployeeNumber:     "202003",
		Username:           "star_priority_test_employee",
		DeptFullLevelNames: "Tech_Group,Product_Center,Product_R&D_Dept",
	}
	if err := ctx.DB.DB.Create(employee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
	}

	// Set department setting (parent department) to disabled
	if err := starCheckPermissionService.SetDepartmentStarCheckSetting("Product_Center", false); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set department star check setting: %v", err)}
	}

	// Set user setting to enabled (should override department)
	if err := starCheckPermissionService.SetUserStarCheckSetting("202003", true); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set user star check setting: %v", err)}
	}

	// Get effective setting - should be user setting (higher priority)
	enabled, err := starCheckPermissionService.GetUserEffectiveStarCheckSetting("202003")
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective star check setting: %v", err)}
	}

	if !enabled {
		return TestResult{Passed: false, Message: "Expected user setting (enabled) to override department setting (disabled)"}
	}

	return TestResult{Passed: true, Message: "Star check setting priority and inheritance test succeeded"}
}

// testStarCheckPermissionDistribution tests star check permission distribution to Higress
func testStarCheckPermissionDistribution(ctx *TestContext) TestResult {
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

	starCheckPermissionService := services.NewStarCheckPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

	// Create test employee
	employee := &models.EmployeeDepartment{
		EmployeeNumber:     "203001",
		Username:           "star_distribution_test",
		DeptFullLevelNames: "Tech_Group,R&D_Center,Frontend_Dev_Dept,Frontend_Dev_Dept_Team1",
	}
	if err := ctx.DB.DB.Create(employee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
	}

	// Clear previous star check calls
	mockStore.ClearStarCheckCalls()

	// Set user star check setting
	if err := starCheckPermissionService.SetUserStarCheckSetting("203001", false); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set user star check setting: %v", err)}
	}

	// Verify star check setting was distributed to Higress
	starCheckCalls := mockStore.GetStarCheckCalls()
	if len(starCheckCalls) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 star check call, got %d", len(starCheckCalls))}
	}

	call := starCheckCalls[0]
	if call.EmployeeNumber != "203001" {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected employee number 203001, got %s", call.EmployeeNumber)}
	}

	if call.Enabled {
		return TestResult{Passed: false, Message: "Expected star check to be disabled in Higress call"}
	}

	return TestResult{Passed: true, Message: "Star check permission distribution test succeeded"}
}

// testUnifiedPermissionQueries tests unified permission query interface
func testUnifiedPermissionQueries(ctx *TestContext) TestResult {
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

	permissionService := services.NewPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)
	starCheckPermissionService := services.NewStarCheckPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)
	quotaCheckPermissionService := services.NewQuotaCheckPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)
	unifiedPermissionService := services.NewUnifiedPermissionService(permissionService, starCheckPermissionService, quotaCheckPermissionService, nil)

	// Create test employee
	employee := &models.EmployeeDepartment{
		EmployeeNumber:     "204001",
		Username:           "unified_query_test",
		DeptFullLevelNames: "Tech_Group,R&D_Center,QA_Dept",
	}
	if err := ctx.DB.DB.Create(employee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
	}

	// Set model permissions
	if err := permissionService.SetUserWhitelist("204001", []string{"gpt-4", "claude-3-opus"}); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set model permissions: %v", err)}
	}

	// Set star check setting
	if err := starCheckPermissionService.SetUserStarCheckSetting("204001", false); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set star check setting: %v", err)}
	}

	// Test unified model permission query
	models, err := unifiedPermissionService.GetModelEffectivePermissions("user", "204001")
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get model permissions via unified service: %v", err)}
	}

	if len(models) != 2 || models[0] != "gpt-4" || models[1] != "claude-3-opus" {
		return TestResult{Passed: false, Message: "Model permissions query via unified service returned unexpected results"}
	}

	// Test unified star check permission query
	enabled, err := unifiedPermissionService.GetStarCheckEffectivePermissions("user", "204001")
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get star check permissions via unified service: %v", err)}
	}

	if enabled {
		return TestResult{Passed: false, Message: "Star check permission query via unified service returned unexpected result"}
	}

	return TestResult{Passed: true, Message: "Unified permission queries test succeeded"}
}

// testStarCheckEmployeeSync tests star check permissions during employee sync
func testStarCheckEmployeeSync(ctx *TestContext) TestResult {
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

	permissionService := services.NewPermissionService(ctx.DB, aiGatewayConfig, employeeSyncConfig, ctx.Gateway)
	starCheckPermissionService := services.NewStarCheckPermissionService(ctx.DB, aiGatewayConfig, employeeSyncConfig, ctx.Gateway)
	quotaCheckPermissionService := services.NewQuotaCheckPermissionService(ctx.DB, aiGatewayConfig, employeeSyncConfig, ctx.Gateway)
	// Create a config manager for the employee sync service
	configManager := config.NewManager(&config.Config{EmployeeSync: *employeeSyncConfig})
	_ = services.NewEmployeeSyncService(ctx.DB, configManager, permissionService, starCheckPermissionService, quotaCheckPermissionService)

	// Create test employee with initial department
	employee := &models.EmployeeDepartment{
		EmployeeNumber:     "205001",
		Username:           "sync_test_employee",
		DeptFullLevelNames: "Tech_Group,R&D_Center,Backend_Dept",
	}
	if err := ctx.DB.DB.Create(employee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
	}

	// Set department-level star check setting
	if err := starCheckPermissionService.SetDepartmentStarCheckSetting("R&D_Center", false); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set department star check setting: %v", err)}
	}

	// Trigger initial permission calculation to establish baseline
	if err := starCheckPermissionService.UpdateEmployeeStarCheckPermissions("205001"); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to establish baseline star check permissions: %v", err)}
	}

	// Verify initial state (should be disabled due to R&D_Center setting)
	initialEnabled, err := starCheckPermissionService.GetUserEffectiveStarCheckSetting("205001")
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get initial star check setting: %v", err)}
	}

	if initialEnabled {
		return TestResult{Passed: false, Message: "Expected initial star check to be disabled due to R&D_Center setting"}
	}

	// Clear previous calls
	mockStore.ClearStarCheckCalls()

	// Create temporary employee in Operations_Center department to satisfy department existence check
	tempEmployee := &models.EmployeeDepartment{
		EmployeeNumber:     "temp_operations_205001",
		Username:           "temp_operations_employee",
		DeptFullLevelNames: "Tech_Group,Operations_Center,Mobile_Support_Dept",
	}
	if err := ctx.DB.DB.Create(tempEmployee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create temporary employee: %v", err)}
	}

	// Set the new department to enabled to create a change
	if err := starCheckPermissionService.SetDepartmentStarCheckSetting("Operations_Center", true); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set Operations_Center department star check setting: %v", err)}
	}

	// Clean up temporary employee
	if err := ctx.DB.DB.Delete(tempEmployee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to delete temporary employee: %v", err)}
	}

	// Simulate department change - use a unique department name to avoid test interference
	employee.DeptFullLevelNames = "Tech_Group,Operations_Center,Mobile_Support_Dept"
	if err := ctx.DB.DB.Save(employee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to update employee department: %v", err)}
	}

	// Trigger permission update (simulating sync)
	if err := starCheckPermissionService.UpdateEmployeeStarCheckPermissions("205001"); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to update star check permissions: %v", err)}
	}

	// Verify star check setting changed (should be enabled now due to Operations_Center setting)
	enabled, err := starCheckPermissionService.GetUserEffectiveStarCheckSetting("205001")
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get updated star check setting: %v", err)}
	}

	if !enabled {
		return TestResult{Passed: false, Message: "Expected star check to be enabled after department change to Operations_Center"}
	}

	// Verify Higress was notified
	starCheckCalls := mockStore.GetStarCheckCalls()
	if len(starCheckCalls) < 1 {
		return TestResult{Passed: false, Message: "Expected at least 1 star check call after department change"}
	}

	return TestResult{Passed: true, Message: "Star check employee sync test succeeded"}
}

// testEmptyStarCheckSettingFallback tests that empty star check settings are treated as "not configured"
// and the system falls back to parent level settings or default
func testEmptyStarCheckSettingFallback(ctx *TestContext) TestResult {
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
		Enabled: false,
		HrURL:   "http://localhost:8099/api/hr/employees",
		HrKey:   "test-hr-key",
		DeptURL: "http://localhost:8099/api/hr/departments",
		DeptKey: "test-dept-key",
	}

	starCheckPermissionService := services.NewStarCheckPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

	// Create test employee in hierarchical department structure
	employee := &models.EmployeeDepartment{
		EmployeeNumber:     "210001",
		Username:           "fallback_test_employee",
		DeptFullLevelNames: "Tech_Group,R&D_Center,Backend_Dev_Dept,Backend_Dev_Team1",
	}
	if err := ctx.DB.DB.Create(employee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
	}

	// Set parent department setting
	if err := starCheckPermissionService.SetDepartmentStarCheckSetting("R&D_Center", false); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set parent department star check setting: %v", err)}
	}

	// Get effective setting - should inherit from parent department
	enabled, err := starCheckPermissionService.GetUserEffectiveStarCheckSetting("210001")
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective star check setting: %v", err)}
	}

	if enabled {
		return TestResult{Passed: false, Message: "Expected star check to be disabled (inherited from R&D_Center)"}
	}

	// Create another employee with no department settings (should get default)
	defaultEmployee := &models.EmployeeDepartment{
		EmployeeNumber:     "210002",
		Username:           "default_test_employee",
		DeptFullLevelNames: "Admin_Group,HR_Center,Recruitment_Dept",
	}
	if err := ctx.DB.DB.Create(defaultEmployee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create default employee: %v", err)}
	}

	// Get effective setting - should get default (enabled)
	defaultEnabled, err := starCheckPermissionService.GetUserEffectiveStarCheckSetting("210002")
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get default star check setting: %v", err)}
	}

	if defaultEnabled {
		return TestResult{Passed: false, Message: "Expected default star check to be disabled"}
	}

	return TestResult{Passed: true, Message: "Empty star check setting fallback test succeeded"}
}

// testSyncWithoutStarCheckSetting tests sync behavior when no star check settings exist
func testSyncWithoutStarCheckSetting(ctx *TestContext) TestResult {
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
		Enabled: false,
		HrURL:   "http://localhost:8099/api/hr/employees",
		HrKey:   "test-hr-key",
		DeptURL: "http://localhost:8099/api/hr/departments",
		DeptKey: "test-dept-key",
	}

	starCheckPermissionService := services.NewStarCheckPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

	// Create test employee
	employee := &models.EmployeeDepartment{
		EmployeeNumber:     "211001",
		Username:           "no_setting_sync_employee",
		DeptFullLevelNames: "Operations_Group,Support_Center,Customer_Service_Dept",
	}
	if err := ctx.DB.DB.Create(employee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
	}

	// Clear previous star check calls
	mockStore.ClearStarCheckCalls()

	// Trigger permission update when no settings exist
	if err := starCheckPermissionService.UpdateEmployeeStarCheckPermissions("211001"); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to update star check permissions: %v", err)}
	}

	// Should NOT notify Higress for new user with default (disabled) setting
	starCheckCalls := mockStore.GetStarCheckCalls()
	if len(starCheckCalls) != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 0 star check calls for new user with default setting, got %d", len(starCheckCalls))}
	}

	// Verify effective setting is default (enabled)
	enabled, err := starCheckPermissionService.GetUserEffectiveStarCheckSetting("211001")
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective star check setting: %v", err)}
	}

	if enabled {
		return TestResult{Passed: false, Message: "Expected default star check to be disabled"}
	}

	return TestResult{Passed: true, Message: "Sync without star check setting test succeeded"}
}
