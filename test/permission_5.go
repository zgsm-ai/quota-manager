package main

import (
	"fmt"
	"quota-manager/internal/handlers"
	"quota-manager/internal/validation"
	"strings"
)

// testPermissionValidation tests permission API parameter validation
func testPermissionValidation(ctx *TestContext) TestResult {
	// Test Set User Whitelist validation
	if !testSetUserWhitelistValidation() {
		return TestResult{Passed: false, Message: "Set User Whitelist validation test failed"}
	}

	// Test Set Department Whitelist validation
	if !testSetDepartmentWhitelistValidation() {
		return TestResult{Passed: false, Message: "Set Department Whitelist validation test failed"}
	}

	// Test Get Effective Permissions validation
	if !testGetEffectivePermissionsValidation() {
		return TestResult{Passed: false, Message: "Get Effective Permissions validation test failed"}
	}

	return TestResult{Passed: true, Message: "Permission validation test succeeded"}
}

// testSetUserWhitelistValidation tests SetUserWhitelistRequest validation
func testSetUserWhitelistValidation() bool {
	// Test valid request
	validReq := handlers.SetUserWhitelistRequest{
		EmployeeNumber: "EMP12345",
		Models:         []string{"gpt-4", "claude-3-opus"},
	}
	if err := validation.ValidateStruct(&validReq); err != nil {
		fmt.Printf("Valid SetUserWhitelistRequest should pass validation: %v\n", err)
		return false
	}

	// Test valid request with empty models (delete permissions)
	emptyModelsReq := handlers.SetUserWhitelistRequest{
		EmployeeNumber: "EMP123",
		Models:         []string{},
	}
	if err := validation.ValidateStruct(&emptyModelsReq); err != nil {
		fmt.Printf("SetUserWhitelistRequest with empty models should pass validation: %v\n", err)
		return false
	}

	// Test invalid employee number - too short
	shortEmpReq := handlers.SetUserWhitelistRequest{
		EmployeeNumber: "E",
		Models:         []string{"gpt-4"},
	}
	if err := validation.ValidateStruct(&shortEmpReq); err == nil {
		fmt.Printf("SetUserWhitelistRequest with short employee number should fail validation\n")
		return false
	}

	// Test invalid employee number - too long
	longEmpReq := handlers.SetUserWhitelistRequest{
		EmployeeNumber: "EMP123456789012345678901", // 23 characters
		Models:         []string{"gpt-4"},
	}
	if err := validation.ValidateStruct(&longEmpReq); err == nil {
		fmt.Printf("SetUserWhitelistRequest with long employee number should fail validation\n")
		return false
	}

	// Test invalid employee number - special characters
	specialCharReq := handlers.SetUserWhitelistRequest{
		EmployeeNumber: "EMP-123@",
		Models:         []string{"gpt-4"},
	}
	if err := validation.ValidateStruct(&specialCharReq); err == nil {
		fmt.Printf("SetUserWhitelistRequest with special characters in employee number should fail validation\n")
		return false
	}

	// Test too many models
	tooManyModelsReq := handlers.SetUserWhitelistRequest{
		EmployeeNumber: "EMP123",
		Models: []string{
			"model1", "model2", "model3", "model4", "model5",
			"model6", "model7", "model8", "model9", "model10", "model11", // 11 models
		},
	}
	if err := validation.ValidateStruct(&tooManyModelsReq); err == nil {
		fmt.Printf("SetUserWhitelistRequest with too many models should fail validation\n")
		return false
	}

	// Test missing required field - employee_number
	missingEmpReq := handlers.SetUserWhitelistRequest{
		Models: []string{"gpt-4"},
	}
	if err := validation.ValidateStruct(&missingEmpReq); err == nil {
		fmt.Printf("SetUserWhitelistRequest with missing employee_number should fail validation\n")
		return false
	}

	// Test missing required field - models
	missingModelsReq := handlers.SetUserWhitelistRequest{
		EmployeeNumber: "EMP123",
		// Models field is not set, should fail validation
	}
	if err := validation.ValidateStruct(&missingModelsReq); err == nil {
		fmt.Printf("SetUserWhitelistRequest with missing models should fail validation\n")
		return false
	}

	return true
}

// testSetDepartmentWhitelistValidation tests SetDepartmentWhitelistRequest validation
func testSetDepartmentWhitelistValidation() bool {
	// Test valid department name - English
	validEngReq := handlers.SetDepartmentWhitelistRequest{
		DepartmentName: "RD_Center",
		Models:         []string{"gpt-4", "deepseek-v3"},
	}
	if err := validation.ValidateStruct(&validEngReq); err != nil {
		fmt.Printf("Valid English department name should pass validation: %v\n", err)
		return false
	}

	// Test valid department name - Chinese
	validChineseReq := handlers.SetDepartmentWhitelistRequest{
		DepartmentName: "研发中心",
		Models:         []string{"claude-3-opus"},
	}
	if err := validation.ValidateStruct(&validChineseReq); err != nil {
		fmt.Printf("Valid Chinese department name should pass validation: %v\n", err)
		return false
	}

	// Test valid department name - Mixed
	validMixedReq := handlers.SetDepartmentWhitelistRequest{
		DepartmentName: "RD部门_Tech-Division",
		Models:         []string{"qwen-turbo"},
	}
	if err := validation.ValidateStruct(&validMixedReq); err != nil {
		fmt.Printf("Valid mixed department name should pass validation: %v\n", err)
		return false
	}

	// Test invalid department name - too short
	shortDeptReq := handlers.SetDepartmentWhitelistRequest{
		DepartmentName: "R",
		Models:         []string{"gpt-4"},
	}
	if err := validation.ValidateStruct(&shortDeptReq); err == nil {
		fmt.Printf("SetDepartmentWhitelistRequest with short department name should fail validation\n")
		return false
	}

	// Test invalid department name - special characters
	specialCharDeptReq := handlers.SetDepartmentWhitelistRequest{
		DepartmentName: "R&D@Center!",
		Models:         []string{"gpt-4"},
	}
	if err := validation.ValidateStruct(&specialCharDeptReq); err == nil {
		fmt.Printf("SetDepartmentWhitelistRequest with invalid special characters should fail validation\n")
		return false
	}

	// Test missing required field - department_name
	missingDeptReq := handlers.SetDepartmentWhitelistRequest{
		Models: []string{"gpt-4"},
	}
	if err := validation.ValidateStruct(&missingDeptReq); err == nil {
		fmt.Printf("SetDepartmentWhitelistRequest with missing department_name should fail validation\n")
		return false
	}

	// Test missing required field - models
	missingModelsReq := handlers.SetDepartmentWhitelistRequest{
		DepartmentName: "研发中心",
		// Models field is not set, should fail validation
	}
	if err := validation.ValidateStruct(&missingModelsReq); err == nil {
		fmt.Printf("SetDepartmentWhitelistRequest with missing models should fail validation\n")
		return false
	}

	return true
}

// testGetEffectivePermissionsValidation tests GetPermissionsRequest validation
func testGetEffectivePermissionsValidation() bool {
	// Test valid user query
	validUserReq := handlers.GetPermissionsRequest{
		TargetType:       "user",
		TargetIdentifier: "EMP12345",
	}
	if err := validation.ValidateStruct(&validUserReq); err != nil {
		fmt.Printf("Valid user query should pass validation: %v\n", err)
		return false
	}

	// Test valid department query
	validDeptReq := handlers.GetPermissionsRequest{
		TargetType:       "department",
		TargetIdentifier: "研发中心",
	}
	if err := validation.ValidateStruct(&validDeptReq); err != nil {
		fmt.Printf("Valid department query should pass validation: %v\n", err)
		return false
	}

	// Test invalid target_type
	invalidTypeReq := handlers.GetPermissionsRequest{
		TargetType:       "group",
		TargetIdentifier: "EMP123",
	}
	if err := validation.ValidateStruct(&invalidTypeReq); err == nil {
		fmt.Printf("GetPermissionsRequest with invalid target_type should fail validation\n")
		return false
	}

	// Test invalid target_identifier - too short
	shortIdentifierReq := handlers.GetPermissionsRequest{
		TargetType:       "user",
		TargetIdentifier: "E",
	}
	if err := validation.ValidateStruct(&shortIdentifierReq); err == nil {
		fmt.Printf("GetPermissionsRequest with short target_identifier should fail validation\n")
		return false
	}

	// Test missing required field - target_type
	missingTypeReq := handlers.GetPermissionsRequest{
		TargetIdentifier: "EMP123",
	}
	if err := validation.ValidateStruct(&missingTypeReq); err == nil {
		fmt.Printf("GetPermissionsRequest with missing target_type should fail validation\n")
		return false
	}

	// Test missing required field - target_identifier
	missingIdentifierReq := handlers.GetPermissionsRequest{
		TargetType: "user",
	}
	if err := validation.ValidateStruct(&missingIdentifierReq); err == nil {
		fmt.Printf("GetPermissionsRequest with missing target_identifier should fail validation\n")
		return false
	}

	return true
}

// testPermissionCustomValidators tests custom validators for permission management
func testPermissionCustomValidators(ctx *TestContext) TestResult {
	// Test employee number validator
	if !testEmployeeNumberValidator() {
		return TestResult{Passed: false, Message: "Employee number validator test failed"}
	}

	// Test department name validator
	if !testDepartmentNameValidator() {
		return TestResult{Passed: false, Message: "Department name validator test failed"}
	}

	return TestResult{Passed: true, Message: "Permission custom validators test succeeded"}
}

// testEmployeeNumberValidator tests the employee_number custom validator
func testEmployeeNumberValidator() bool {
	// Test valid employee numbers
	validEmployeeNumbers := []string{
		"EMP123",
		"emp456",
		"ABC123DEF",
		"12345",
		"ab",                   // minimum length
		"12345678901234567890", // maximum length (20 chars)
	}

	for _, empNum := range validEmployeeNumbers {
		req := handlers.SetUserWhitelistRequest{
			EmployeeNumber: empNum,
			Models:         []string{},
		}
		if err := validation.ValidateStruct(&req); err != nil {
			fmt.Printf("Valid employee number '%s' should pass validation: %v\n", empNum, err)
			return false
		}
	}

	// Test invalid employee numbers
	invalidEmployeeNumbers := []string{
		"E",                     // too short
		"123456789012345678901", // too long (21 chars)
		"EMP-123",               // contains hyphen
		"EMP@123",               // contains @
		"EMP 123",               // contains space
		"EMP_123",               // contains underscore
		"",                      // empty
	}

	for _, empNum := range invalidEmployeeNumbers {
		req := handlers.SetUserWhitelistRequest{
			EmployeeNumber: empNum,
			Models:         []string{},
		}
		if err := validation.ValidateStruct(&req); err == nil {
			fmt.Printf("Invalid employee number '%s' should fail validation\n", empNum)
			return false
		}
	}

	return true
}

// testDepartmentNameValidator tests the department_name custom validator
func testDepartmentNameValidator() bool {
	// Test valid department names
	validDepartmentNames := []string{
		"RD",             // minimum length
		"研发中心",           // Chinese
		"RD_Center",      // English with underscore
		"Tech-Division",  // English with hyphen
		"RD部门_Tech",      // Mixed Chinese and English
		"Development123", // With numbers
	}

	for _, deptName := range validDepartmentNames {
		req := handlers.SetDepartmentWhitelistRequest{
			DepartmentName: deptName,
			Models:         []string{},
		}
		if err := validation.ValidateStruct(&req); err != nil {
			fmt.Printf("Valid department name '%s' should pass validation: %v\n", deptName, err)
			return false
		}
	}

	// Test invalid department names
	invalidDepartmentNames := []string{
		"R",    // too short
		"R&D",  // contains &
		"R@D",  // contains @
		"R!D",  // contains !
		"R$D",  // contains $
		"R%D",  // contains %
		"R^D",  // contains ^
		"R*D",  // contains *
		"R+D",  // contains +
		"R=D",  // contains =
		"R|D",  // contains |
		"R\\D", // contains backslash
		"R/D",  // contains forward slash
		"R?D",  // contains question mark
		"R<D",  // contains <
		"R>D",  // contains >
		"R,D",  // contains comma
		"R.D",  // contains period
		"R;D",  // contains semicolon
		"R:D",  // contains colon
		"R\"D", // contains quote
		"R'D",  // contains apostrophe
		"",     // empty
	}

	for _, deptName := range invalidDepartmentNames {
		req := handlers.SetDepartmentWhitelistRequest{
			DepartmentName: deptName,
			Models:         []string{},
		}
		if err := validation.ValidateStruct(&req); err == nil {
			fmt.Printf("Invalid department name '%s' should fail validation\n", deptName)
			return false
		}
	}

	return true
}

// testPermissionValidationErrorMessages tests validation error messages
func testPermissionValidationErrorMessages(ctx *TestContext) TestResult {
	// Test employee number error message
	invalidEmpReq := handlers.SetUserWhitelistRequest{
		EmployeeNumber: "E", // too short
		Models:         []string{},
	}
	if err := validation.ValidateStruct(&invalidEmpReq); err != nil {
		errorMsg := err.Error()
		if !containsKeywords(errorMsg, []string{"employee_number", "2-20 characters", "alphanumeric"}) {
			return TestResult{Passed: false, Message: fmt.Sprintf("Employee number error message should contain field name and requirements: %s", errorMsg)}
		}
	} else {
		return TestResult{Passed: false, Message: "Invalid employee number should produce validation error"}
	}

	// Test department name error message
	invalidDeptReq := handlers.SetDepartmentWhitelistRequest{
		DepartmentName: "R@D", // invalid characters
		Models:         []string{},
	}
	if err := validation.ValidateStruct(&invalidDeptReq); err != nil {
		errorMsg := err.Error()
		if !containsKeywords(errorMsg, []string{"department_name", "2-100 characters"}) {
			return TestResult{Passed: false, Message: fmt.Sprintf("Department name error message should contain field name and requirements: %s", errorMsg)}
		}
	} else {
		return TestResult{Passed: false, Message: "Invalid department name should produce validation error"}
	}

	// Test too many models error message
	tooManyModelsReq := handlers.SetUserWhitelistRequest{
		EmployeeNumber: "EMP123",
		Models: []string{
			"model1", "model2", "model3", "model4", "model5",
			"model6", "model7", "model8", "model9", "model10", "model11", // 11 models
		},
	}
	if err := validation.ValidateStruct(&tooManyModelsReq); err != nil {
		errorMsg := err.Error()
		if !containsKeywords(errorMsg, []string{"models", "10"}) {
			return TestResult{Passed: false, Message: fmt.Sprintf("Too many models error message should mention field and limit: %s", errorMsg)}
		}
	} else {
		return TestResult{Passed: false, Message: "Too many models should produce validation error"}
	}

	return TestResult{Passed: true, Message: "Permission validation error messages test succeeded"}
}

// containsKeywords checks if a string contains all specified keywords
func containsKeywords(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if !strings.Contains(strings.ToLower(text), strings.ToLower(keyword)) {
			return false
		}
	}
	return true
}
