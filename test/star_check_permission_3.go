package main

import (
	"fmt"
	"quota-manager/internal/config"
	"quota-manager/internal/models"
	"quota-manager/internal/services"

	"time"

	"github.com/google/uuid"
)

// testUserStarCheckSettingChange tests user star check setting changes
func testUserStarCheckSettingChange(ctx *TestContext) TestResult {
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
		EmployeeNumber:     "260040",
		Username:           "user_change_test_star_employee",
		DeptFullLevelNames: "Tech_Group,Operations_Center,DevOps_Dept",
	}
	if err := ctx.DB.DB.Create(employee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
	}

	// Create auth mapping and use UUID for user-level operations
	uid, errUID := createAuthUserForEmployee(ctx, employee.EmployeeNumber, employee.Username)
	if errUID != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create auth user for %s: %v", employee.EmployeeNumber, errUID)}
	}
	// Initially set user star check setting to disabled
	if err := starCheckPermissionService.SetUserStarCheckSetting(uid, false); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set initial user star check setting: %v", err)}
	}

	// Clear star check calls
	mockStore.ClearStarCheckCalls()

	// Change user star check setting to enabled
	if err := starCheckPermissionService.SetUserStarCheckSetting(uid, true); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to update user star check setting: %v", err)}
	}

	// Verify Higress call was made
	starCheckCalls := mockStore.GetStarCheckCalls()
	if len(starCheckCalls) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 star check call for user change, got %d", len(starCheckCalls))}
	}

	call := starCheckCalls[0]
	if call.EmployeeNumber != "260040" {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected employee number 260040, got %s", call.EmployeeNumber)}
	}
	if !call.Enabled {
		return TestResult{Passed: false, Message: "Expected star check to be enabled in Higress call"}
	}

	// Verify effective setting
	enabled, err := starCheckPermissionService.GetUserEffectiveStarCheckSetting(uid)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective star check setting: %v", err)}
	}
	if !enabled {
		return TestResult{Passed: false, Message: "Expected star check to be enabled"}
	}

	return TestResult{Passed: true, Message: "User star check setting change test succeeded"}
}

// testUserDepartmentStarCheckChange tests user department change affecting star check settings
func testUserDepartmentStarCheckChange(ctx *TestContext) TestResult {
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

	starCheckPermissionService := services.NewStarCheckPermissionService(ctx.DB, aiGatewayConfig, employeeSyncConfig, ctx.Gateway)

	// Run multiple department change scenarios
	scenarios := []struct {
		name                 string
		employeeNumber       string
		originalDept         string
		targetDept           string
		originalSetting      *bool // nil means no setting
		targetSetting        *bool // nil means no setting
		expectedSetting      bool
		expectedNotification bool
		description          string
	}{
		{
			name:                 "No settings - default behavior",
			employeeNumber:       "dept_change_001",
			originalDept:         "Source_Dept",
			targetDept:           "Target_Dept",
			originalSetting:      nil,
			targetSetting:        nil,
			expectedSetting:      false, // default disabled
			expectedNotification: false,
			description:          "User moves between departments with no star check settings",
		},
		{
			name:                 "From disabled dept to no setting",
			employeeNumber:       "dept_change_002",
			originalDept:         "Disabled_Source_Dept",
			targetDept:           "No_Setting_Target_Dept",
			originalSetting:      new(bool), // false
			targetSetting:        nil,
			expectedSetting:      false, // default disabled
			expectedNotification: false, // no change (false -> false)
			description:          "User moves from department with disabled star check to department with no setting",
		},
		{
			name:                 "From no setting to disabled dept",
			employeeNumber:       "dept_change_003",
			originalDept:         "No_Setting_Source_Dept",
			targetDept:           "Disabled_Target_Dept",
			originalSetting:      nil,
			targetSetting:        new(bool), // false
			expectedSetting:      false,
			expectedNotification: false, // no change (false -> false)
			description:          "User moves from department with no setting to department with disabled star check",
		},
		{
			name:                 "From disabled to enabled dept",
			employeeNumber:       "dept_change_004",
			originalDept:         "Disabled_Source_Dept2",
			targetDept:           "Enabled_Target_Dept",
			originalSetting:      new(bool), // false
			targetSetting:        func() *bool { b := true; return &b }(),
			expectedSetting:      true,
			expectedNotification: true,
			description:          "User moves from disabled star check department to enabled star check department",
		},
	}

	for _, scenario := range scenarios {
		// Clear data for each scenario
		if err := clearPermissionData(ctx); err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Scenario %s: Failed to clear permission data: %v", scenario.name, err)}
		}

		// Setup: Create test employee in original department
		employee := &models.EmployeeDepartment{
			EmployeeNumber:     scenario.employeeNumber,
			Username:           fmt.Sprintf("test_employee_%s", scenario.employeeNumber),
			DeptFullLevelNames: fmt.Sprintf("Company_Root,Division_A,%s,%s_Team", scenario.originalDept, scenario.originalDept),
		}
		if err := ctx.DB.DB.Create(employee).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Scenario %s: Failed to create employee: %v", scenario.name, err)}
		}

		// Also create corresponding auth user in authdb for employee sync resolution
		authUser := &models.UserInfo{
			ID:             uuid.NewString(),
			CreatedAt:      time.Now().Add(-time.Hour),
			UpdatedAt:      time.Now(),
			AccessTime:     time.Now(),
			Name:           employee.Username,
			EmployeeNumber: scenario.employeeNumber,
			GithubID:       fmt.Sprintf("test_%s_%d", scenario.employeeNumber, time.Now().UnixNano()),
			GithubName:     employee.Username,
			Devices:        "{}",
		}
		if err := ctx.DB.AuthDB.Create(authUser).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Scenario %s: Failed to create auth user: %v", scenario.name, err)}
		}

		// Setup: Configure original department setting if specified
		if scenario.originalSetting != nil {
			if err := starCheckPermissionService.SetDepartmentStarCheckSetting(scenario.originalDept, *scenario.originalSetting); err != nil {
				return TestResult{Passed: false, Message: fmt.Sprintf("Scenario %s: Failed to set original department setting: %v", scenario.name, err)}
			}
		}

		// Setup: Configure target department setting if specified
		if scenario.targetSetting != nil {
			// Create temporary employee in target department if needed to satisfy department existence check
			var tempEmployee *models.EmployeeDepartment
			var targetEmployeeCount int64
			ctx.DB.DB.Model(&models.EmployeeDepartment{}).Where("dept_full_level_names LIKE ?", "%"+scenario.targetDept+"%").Count(&targetEmployeeCount)

			if targetEmployeeCount == 0 {
				tempEmployee = &models.EmployeeDepartment{
					EmployeeNumber:     fmt.Sprintf("temp_%s", scenario.employeeNumber),
					Username:           "temp_star_check_employee",
					DeptFullLevelNames: fmt.Sprintf("Company_Root,Division_B,%s,%s_Team", scenario.targetDept, scenario.targetDept),
				}
				if err := ctx.DB.DB.Create(tempEmployee).Error; err != nil {
					return TestResult{Passed: false, Message: fmt.Sprintf("Scenario %s: Failed to create temporary employee: %v", scenario.name, err)}
				}
			}

			if err := starCheckPermissionService.SetDepartmentStarCheckSetting(scenario.targetDept, *scenario.targetSetting); err != nil {
				// Clean up temporary employee before returning error
				if tempEmployee != nil {
					ctx.DB.DB.Delete(tempEmployee)
				}
				return TestResult{Passed: false, Message: fmt.Sprintf("Scenario %s: Failed to set target department setting: %v", scenario.name, err)}
			}

			// Clean up temporary employee
			if tempEmployee != nil {
				if err := ctx.DB.DB.Delete(tempEmployee).Error; err != nil {
					return TestResult{Passed: false, Message: fmt.Sprintf("Scenario %s: Failed to delete temporary employee: %v", scenario.name, err)}
				}
			}
		}

		// Trigger initial star check permission calculation
		if err := starCheckPermissionService.UpdateEmployeeStarCheckPermissions(scenario.employeeNumber); err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Scenario %s: Failed to establish baseline: %v", scenario.name, err)}
		}

		// Clear star check calls to focus on department change
		mockStore.ClearStarCheckCalls()

		// Action: Change employee's department
		employee.DeptFullLevelNames = fmt.Sprintf("Company_Root,Division_B,%s,%s_Team", scenario.targetDept, scenario.targetDept)
		if err := ctx.DB.DB.Save(employee).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Scenario %s: Failed to update employee department: %v", scenario.name, err)}
		}

		// Trigger star check permission update (simulating employee sync)
		if err := starCheckPermissionService.UpdateEmployeeStarCheckPermissions(scenario.employeeNumber); err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Scenario %s: Failed to update star check permissions: %v", scenario.name, err)}
		}

		// Verify: Check effective star check setting
		actualSetting, err := starCheckPermissionService.GetUserEffectiveStarCheckSetting(authUser.ID)
		if err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Scenario %s: Failed to get effective star check setting: %v", scenario.name, err)}
		}

		if actualSetting != scenario.expectedSetting {
			return TestResult{Passed: false, Message: fmt.Sprintf("Scenario %s: Expected star check setting %v, got %v", scenario.name, scenario.expectedSetting, actualSetting)}
		}

		// Verify: Check Higress notification
		starCheckCalls := mockStore.GetStarCheckCalls()
		actualNotification := len(starCheckCalls) > 0

		if actualNotification != scenario.expectedNotification {
			return TestResult{Passed: false, Message: fmt.Sprintf("Scenario %s: Expected notification %v, got %v", scenario.name, scenario.expectedNotification, actualNotification)}
		}

		if scenario.expectedNotification && len(starCheckCalls) > 0 {
			call := starCheckCalls[0]
			if call.EmployeeNumber != scenario.employeeNumber {
				return TestResult{Passed: false, Message: fmt.Sprintf("Scenario %s: Wrong employee number in Higress call", scenario.name)}
			}
			if call.Enabled != scenario.expectedSetting {
				return TestResult{Passed: false, Message: fmt.Sprintf("Scenario %s: Wrong setting in Higress call", scenario.name)}
			}
		}
	}

	return TestResult{Passed: true, Message: "User department star check change test succeeded"}
}

// testUserStarCheckAdditionAndRemoval tests user star check addition and removal scenarios
func testUserStarCheckAdditionAndRemoval(ctx *TestContext) TestResult {
	// Clear star check data for test isolation
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

	// Test Scenario 1: Add new user with star check setting
	employee1 := &models.EmployeeDepartment{
		EmployeeNumber:     "addition_001",
		Username:           "new_user_with_setting",
		DeptFullLevelNames: "Tech_Group,Innovation_Center,Research_Dept",
	}
	if err := ctx.DB.DB.Create(employee1).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee1: %v", err)}
	}

	// Clear calls before setting
	mockStore.ClearStarCheckCalls()

	// Create auth mapping and set using UUID
	addUID, errUID := createAuthUserForEmployee(ctx, employee1.EmployeeNumber, employee1.Username)
	if errUID != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create auth user for %s: %v", employee1.EmployeeNumber, errUID)}
	}
	// Set user star check setting to enabled (different from default false)
	if err := starCheckPermissionService.SetUserStarCheckSetting(addUID, true); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set star check setting for new user: %v", err)}
	}

	// Verify Higress was notified for new user with specific setting
	starCheckCalls := mockStore.GetStarCheckCalls()
	if len(starCheckCalls) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 star check call for new user with setting, got %d", len(starCheckCalls))}
	}

	if starCheckCalls[0].EmployeeNumber != "addition_001" || !starCheckCalls[0].Enabled {
		return TestResult{Passed: false, Message: "Incorrect Higress call for new user with enabled setting"}
	}

	// Test Scenario 2: Remove user star check setting (revert to default)
	mockStore.ClearStarCheckCalls()

	// First set a department setting so removing user setting has visible effect
	if err := starCheckPermissionService.SetDepartmentStarCheckSetting("Research_Dept", false); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set department setting: %v", err)}
	}

	// Clear the user setting by setting it to the same as department (simulating removal)
	if err := starCheckPermissionService.SetUserStarCheckSetting(addUID, false); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set identical star check setting: %v", err)}
	}

	// Verify notification for setting change (from true to false)
	starCheckCalls = mockStore.GetStarCheckCalls()
	if len(starCheckCalls) != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected 1 star check call for setting change, got %d", len(starCheckCalls))}
	}

	// Test Scenario 3: Complete user removal simulation via EmployeeSyncService
	// Create employee sync service to properly handle user deletion
	employeeSyncConfig := &config.EmployeeSyncConfig{
		Enabled: true,
		HrURL:   ctx.MockServer.URL + "/api/test/employees",
		HrKey:   "TEST_EMP_KEY_32_BYTES_1234567890",
		DeptURL: ctx.MockServer.URL + "/api/test/departments",
		DeptKey: "TEST_DEPT_KEY_32_BYTES_123456789",
	}

	// Create a minimal permission service for EmployeeSyncService
	permissionService := services.NewPermissionService(ctx.DB, aiGatewayConfig, employeeSyncConfig, ctx.Gateway)
	quotaCheckPermissionService := services.NewQuotaCheckPermissionService(ctx.DB, aiGatewayConfig, employeeSyncConfig, ctx.Gateway)
	employeeSyncService := services.NewEmployeeSyncService(ctx.DB, config.NewManager(&config.Config{EmployeeSync: *employeeSyncConfig}), permissionService, starCheckPermissionService, quotaCheckPermissionService)

	// Setup department hierarchy
	SetupDefaultDepartmentHierarchy()

	// Add user to mock HR data (deptID 8 corresponds to Testing_Dept_Team1)
	AddMockEmployee("removal_002", "user_to_remove", "test_removal_002@example.com", "13800000002", 8)

	// Sync the user into the system
	if err := employeeSyncService.SyncEmployees(); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to sync new employee: %v", err)}
	}

	// Set user star check setting using UUID
	remUID, errUID := createAuthUserForEmployee(ctx, "removal_002", "user_to_remove")
	if errUID != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create auth user for removal_002: %v", errUID)}
	}
	if err := starCheckPermissionService.SetUserStarCheckSetting(remUID, true); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set star check setting for user to remove: %v", err)}
	}

	// Verify user has effective setting
	enabled, err := starCheckPermissionService.GetUserEffectiveStarCheckSetting(remUID)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective setting for user to remove: %v", err)}
	}
	if !enabled {
		return TestResult{Passed: false, Message: "Expected user to have enabled star check setting"}
	}

	// Remove user from mock HR data (simulating HR system deletion)
	RemoveMockEmployeeByNumber("removal_002")

	// Trigger employee removal via employee sync service
	if err := employeeSyncService.SyncEmployees(); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Employee sync for removal failed: %v", err)}
	}

	// Verify getting effective setting for removed user should return error
	removedEnabled, err := starCheckPermissionService.GetUserEffectiveStarCheckSetting("removal_002")
	if err == nil {
		return TestResult{Passed: false, Message: "Expected error when getting effective setting for removed user"}
	}
	_ = removedEnabled

	return TestResult{Passed: true, Message: "User star check addition and removal test succeeded"}
}

// testNonExistentUserAndDepartmentStarCheck tests star check behavior with non-existent users and departments
func testNonExistentUserAndDepartmentStarCheck(ctx *TestContext) TestResult {
	// Clear star check data for test isolation
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
		Enabled: false, // Disabled to allow non-existent user testing
		HrURL:   "http://localhost:8099/api/hr/employees",
		HrKey:   "test-hr-key",
		DeptURL: "http://localhost:8099/api/hr/departments",
		DeptKey: "test-dept-key",
	}

	starCheckPermissionService := services.NewStarCheckPermissionService(ctx.DB, aiGatewayConfig, defaultEmployeeSyncConfig, ctx.Gateway)

	// Test 1: Set star check setting for non-existent user (should fail now even when sync disabled)
	if err := starCheckPermissionService.SetUserStarCheckSetting("nonexistent_user_001", false); err == nil {
		return TestResult{Passed: false, Message: "Expected error when setting star check for non-existent user with sync disabled, but got no error"}
	}

	// Verify get effective also fails for non-existent user
	_, err := starCheckPermissionService.GetUserEffectiveStarCheckSetting("nonexistent_user_001")
	if err == nil {
		return TestResult{Passed: false, Message: "Expected error when getting star check for non-existent user with sync disabled"}
	}

	// Test 2: Set star check setting for non-existent department (should fail)
	err = starCheckPermissionService.SetDepartmentStarCheckSetting("NonExistent_Department", false)
	if err == nil {
		return TestResult{Passed: false, Message: "Expected error when setting star check for non-existent department"}
	}

	// Test 3: Get star check setting for non-existent department (should return error)
	_, err = starCheckPermissionService.GetDepartmentStarCheckSetting("NonExistent_Department")
	if err == nil {
		return TestResult{Passed: false, Message: "Expected error when getting star check setting for non-existent department"}
	}

	// Test 4: Update star check permissions for non-existent user
	if err := starCheckPermissionService.UpdateEmployeeStarCheckPermissions("nonexistent_user_002"); err != nil {
		// Update may still be non-erroring; continue to validate Get behavior
	}

	// Verify getting effective setting for non-existent user returns error
	_, err = starCheckPermissionService.GetUserEffectiveStarCheckSetting("nonexistent_user_002")
	if err == nil {
		return TestResult{Passed: false, Message: "Expected error when getting star check setting for non-existent user"}
	}

	// Test 5: Test with employee sync enabled (should enforce user existence)
	employeeSyncConfig := &config.EmployeeSyncConfig{
		Enabled: true, // Enabled to enforce user validation
		HrURL:   "http://localhost:8099/api/hr/employees",
		HrKey:   "test-hr-key",
		DeptURL: "http://localhost:8099/api/hr/departments",
		DeptKey: "test-dept-key",
	}

	strictStarCheckService := services.NewStarCheckPermissionService(ctx.DB, aiGatewayConfig, employeeSyncConfig, ctx.Gateway)

	// Should fail when trying to set star check for non-existent user with sync enabled
	err = strictStarCheckService.SetUserStarCheckSetting("nonexistent_user_strict", false)
	if err == nil {
		return TestResult{Passed: false, Message: "Expected error when setting star check for non-existent user with sync enabled"}
	}

	return TestResult{Passed: true, Message: "Non-existent user and department star check test succeeded"}
}

// testStarCheckEmployeeDataIntegrity tests star check employee data integrity scenarios
func testStarCheckEmployeeDataIntegrity(ctx *TestContext) TestResult {
	// Clear star check data for test isolation
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

	// Test 1: Data consistency between settings and effective settings
	employee := &models.EmployeeDepartment{
		EmployeeNumber:     "integrity_001",
		Username:           "integrity_test_user",
		DeptFullLevelNames: "Tech_Group,Quality_Center,Testing_Dept",
	}
	if err := ctx.DB.DB.Create(employee).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee: %v", err)}
	}

	// Create auth mapping and set using UUID
	uid, errUID := createAuthUserForEmployee(ctx, employee.EmployeeNumber, employee.Username)
	if errUID != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create auth user for %s: %v", employee.EmployeeNumber, errUID)}
	}
	// Set user star check setting
	if err := starCheckPermissionService.SetUserStarCheckSetting(uid, false); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set user star check setting: %v", err)}
	}

	// Verify effective setting matches
	effectiveEnabled, err := starCheckPermissionService.GetUserEffectiveStarCheckSetting(uid)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective star check setting: %v", err)}
	}
	if effectiveEnabled {
		return TestResult{Passed: false, Message: "Effective setting should match user setting (disabled)"}
	}

	// Check database consistency
	var userSetting models.StarCheckSetting
	err = ctx.DB.DB.Where("target_type = ? AND target_identifier = ?",
		models.TargetTypeUser, "integrity_001").First(&userSetting).Error
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to find user star check setting in database: %v", err)}
	}

	var effectiveSetting models.EffectiveStarCheckSetting
	err = ctx.DB.DB.Where("employee_number = ?", "integrity_001").First(&effectiveSetting).Error
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to find effective star check setting in database: %v", err)}
	}

	if userSetting.Enabled != effectiveSetting.Enabled {
		return TestResult{Passed: false, Message: "User setting and effective setting should match in database"}
	}

	if effectiveSetting.SettingID == nil || *effectiveSetting.SettingID != userSetting.ID {
		return TestResult{Passed: false, Message: "Effective setting should reference correct user setting ID"}
	}

	// Test 2: Department setting inheritance integrity
	if err := starCheckPermissionService.SetDepartmentStarCheckSetting("Testing_Dept", true); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to set department star check setting: %v", err)}
	}

	// User setting should still take precedence
	effectiveEnabled, err = starCheckPermissionService.GetUserEffectiveStarCheckSetting(uid)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get effective setting after department change: %v", err)}
	}
	if effectiveEnabled {
		return TestResult{Passed: false, Message: "User setting should still take precedence over department setting"}
	}

	// Test 3: Orphaned effective settings handling
	// Create another employee to test orphaned effective settings
	employee2 := &models.EmployeeDepartment{
		EmployeeNumber:     "integrity_002",
		Username:           "integrity_test_user2",
		DeptFullLevelNames: "Tech_Group,Quality_Center,Testing_Dept",
	}
	if err := ctx.DB.DB.Create(employee2).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee2: %v", err)}
	}

	// This employee should inherit department setting
	// First, update the employee's star check permissions to create the effective setting record
	if err := starCheckPermissionService.UpdateEmployeeStarCheckPermissions("integrity_002"); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to update employee2 star check permissions: %v", err)}
	}

	// Create auth mapping for employee2 and query by user_id under EmployeeSync=true
	uid2, errUID2 := createAuthUserForEmployee(ctx, employee2.EmployeeNumber, employee2.Username)
	if errUID2 != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create auth user for %s: %v", employee2.EmployeeNumber, errUID2)}
	}
	departmentEnabled, err := starCheckPermissionService.GetUserEffectiveStarCheckSetting(uid2)
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to get department inherited setting: %v", err)}
	}
	if !departmentEnabled {
		return TestResult{Passed: false, Message: "Employee should inherit enabled department setting"}
	}

	// Verify database references
	var effectiveSetting2 models.EffectiveStarCheckSetting
	err = ctx.DB.DB.Where("employee_number = ?", "integrity_002").First(&effectiveSetting2).Error
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to find effective setting for employee2: %v", err)}
	}

	var deptSetting models.StarCheckSetting
	err = ctx.DB.DB.Where("target_type = ? AND target_identifier = ?",
		models.TargetTypeDepartment, "Testing_Dept").First(&deptSetting).Error
	if err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to find department setting: %v", err)}
	}

	if effectiveSetting2.SettingID == nil || *effectiveSetting2.SettingID != deptSetting.ID {
		return TestResult{Passed: false, Message: "Effective setting should reference correct department setting ID"}
	}

	return TestResult{Passed: true, Message: "Star check employee data integrity test succeeded"}
}
