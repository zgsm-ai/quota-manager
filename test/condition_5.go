package main

import (
	"fmt"
	"quota-manager/internal/condition"
	"quota-manager/internal/config"
	"quota-manager/internal/models"
	"time"
)

// testBelongToWithEmployeeSync tests belong-to condition with employee sync enabled
func testBelongToWithEmployeeSync(ctx *TestContext) TestResult {
	// Create mock employee sync configuration
	employeeSyncConfig := &config.EmployeeSyncConfig{
		Enabled: true,
		HrURL:   "http://localhost:8099/api/hr/employees",
		HrKey:   "test-hr-key",
		DeptURL: "http://localhost:8099/api/hr/departments",
		DeptKey: "test-dept-key",
	}

	// Create test employee department data
	employees := []models.EmployeeDepartment{
		{
			EmployeeNumber:     "EMP001",
			Username:           "zhang_san",
			DeptFullLevelNames: "公司,技术部,研发中心,AI团队",
		},
		{
			EmployeeNumber:     "EMP002",
			Username:           "li_si",
			DeptFullLevelNames: "Company,Tech_Group,R&D_Center,AI_Team",
		},
		{
			EmployeeNumber:     "EMP003",
			Username:           "wang_wu",
			DeptFullLevelNames: "公司,销售部,华南区",
		},
	}

	// Insert test employee department data
	for _, emp := range employees {
		if err := ctx.DB.DB.Create(&emp).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create employee department: %v", err)}
		}
	}

	// Create test users with employee numbers
	userZhangSan := createTestUser("zhang_san_id", "Zhang San", 0)
	userZhangSan.EmployeeNumber = "EMP001"

	userLiSi := createTestUser("li_si_id", "Li Si", 0)
	userLiSi.EmployeeNumber = "EMP002"

	userWangWu := createTestUser("wang_wu_id", "Wang Wu", 0)
	userWangWu.EmployeeNumber = "EMP003"

	userNoEmpNum := createTestUser("no_emp_num_id", "No Employee Number", 0)
	userNoEmpNum.EmployeeNumber = ""

	users := []*models.UserInfo{userZhangSan, userLiSi, userWangWu, userNoEmpNum}

	for _, user := range users {
		if err := ctx.DB.AuthDB.Create(user).Error; err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create user: %v", err)}
		}
	}

	// Test Chinese department name matching
	strategyNameChinese := fmt.Sprintf("belong-to-chinese-test-%d", time.Now().UnixNano())
	strategyChinese := &models.QuotaStrategy{
		Name:      strategyNameChinese,
		Title:     "Belong To Chinese Department Test",
		Type:      "single",
		Amount:    100,
		Model:     "test-model",
		Condition: `belong-to("技术部")`, // Chinese department name
		Status:    true,
	}

	// Create strategy service with employee sync enabled
	strategyService := ctx.createStrategyServiceWithEmployeeSync(employeeSyncConfig)

	if err := strategyService.CreateStrategy(strategyChinese); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create Chinese strategy: %v", err)}
	}

	// Execute strategy
	userList := []models.UserInfo{*userZhangSan, *userLiSi, *userWangWu, *userNoEmpNum}
	strategyService.ExecStrategy(strategyChinese, userList)

	// Check Zhang San should be executed (matches Chinese department)
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategyChinese.ID, userZhangSan.ID).Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Zhang San (Chinese dept) expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check Li Si should not be executed (different department structure)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategyChinese.ID, userLiSi.ID).Count(&executeCount)
	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Li Si (English dept) expected execution 0 times, actually executed %d times", executeCount)}
	}

	// Test English department name matching
	strategyNameEnglish := fmt.Sprintf("belong-to-english-test-%d", time.Now().UnixNano())
	strategyEnglish := &models.QuotaStrategy{
		Name:      strategyNameEnglish,
		Title:     "Belong To English Department Test",
		Type:      "single",
		Amount:    100,
		Model:     "test-model",
		Condition: `belong-to("Tech_Group")`, // English department name
		Status:    true,
	}

	if err := strategyService.CreateStrategy(strategyEnglish); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create English strategy: %v", err)}
	}

	// Execute strategy
	strategyService.ExecStrategy(strategyEnglish, userList)

	// Check Li Si should be executed (matches English department)
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategyEnglish.ID, userLiSi.ID).Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Li Si (English dept) expected execution 1 time, actually executed %d times", executeCount)}
	}

	// Check Zhang San should not be executed (Chinese dept structure doesn't contain "Tech_Group")
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategyEnglish.ID, userZhangSan.ID).Count(&executeCount)
	if executeCount != 0 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Zhang San (Chinese dept) expected execution 0 times, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "belong-to with employee sync test succeeded"}
}

// testBelongToFallbackToOriginal tests belong-to condition falls back to original logic
func testBelongToFallbackToOriginal(ctx *TestContext) TestResult {
	// Create mock employee sync configuration with disabled sync
	employeeSyncConfig := &config.EmployeeSyncConfig{
		Enabled: false, // Disabled sync should fall back to original logic
	}

	// Create test user with Company field set
	userWithCompany := createTestUser("company_user_id", "Company User", 0)
	userWithCompany.Company = "TechCorp"
	userWithCompany.EmployeeNumber = "EMP999" // This should be ignored when sync is disabled

	if err := ctx.DB.AuthDB.Create(userWithCompany).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create user: %v", err)}
	}

	// Create strategy using original Company-based logic
	strategyName := fmt.Sprintf("belong-to-fallback-test-%d", time.Now().UnixNano())
	strategy := &models.QuotaStrategy{
		Name:      strategyName,
		Title:     "Belong To Fallback Test",
		Type:      "single",
		Amount:    100,
		Model:     "test-model",
		Condition: `belong-to("TechCorp")`,
		Status:    true,
	}

	// Create strategy service with employee sync disabled
	strategyService := ctx.createStrategyServiceWithEmployeeSync(employeeSyncConfig)

	if err := strategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create strategy: %v", err)}
	}

	// Execute strategy
	userList := []models.UserInfo{*userWithCompany}
	strategyService.ExecStrategy(strategy, userList)

	// Check user should be executed using Company field
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userWithCompany.ID).Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("Company user expected execution 1 time, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "belong-to fallback to original logic test succeeded"}
}

// testBelongToWithNoEmployeeNumber tests belong-to condition with empty employee number
func testBelongToWithNoEmployeeNumber(ctx *TestContext) TestResult {
	// Create mock employee sync configuration
	employeeSyncConfig := &config.EmployeeSyncConfig{
		Enabled: true,
	}

	// Create test user without employee number but with Company
	userNoEmpNum := createTestUser("no_emp_user_id", "No Employee Number", 0)
	userNoEmpNum.Company = "FallbackCorp"
	userNoEmpNum.EmployeeNumber = "" // Empty employee number

	if err := ctx.DB.AuthDB.Create(userNoEmpNum).Error; err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create user: %v", err)}
	}

	// Create strategy
	strategyName := fmt.Sprintf("belong-to-no-empnum-test-%d", time.Now().UnixNano())
	strategy := &models.QuotaStrategy{
		Name:      strategyName,
		Title:     "Belong To No Employee Number Test",
		Type:      "single",
		Amount:    100,
		Model:     "test-model",
		Condition: `belong-to("FallbackCorp")`,
		Status:    true,
	}

	// Create strategy service with employee sync enabled
	strategyService := ctx.createStrategyServiceWithEmployeeSync(employeeSyncConfig)

	if err := strategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create strategy: %v", err)}
	}

	// Execute strategy
	userList := []models.UserInfo{*userNoEmpNum}
	strategyService.ExecStrategy(strategy, userList)

	// Should fall back to Company field when employee number is empty
	var executeCount int64
	ctx.DB.Model(&models.QuotaExecute{}).Where("strategy_id = ? AND user_id = ? AND status = 'completed'", strategy.ID, userNoEmpNum.ID).Count(&executeCount)
	if executeCount != 1 {
		return TestResult{Passed: false, Message: fmt.Sprintf("User without employee number expected execution 1 time, actually executed %d times", executeCount)}
	}

	return TestResult{Passed: true, Message: "belong-to with no employee number test succeeded"}
}

func testBelongToMultipleOrgs(ctx *TestContext) TestResult {
	// Test multiple organization parameters
	conditionStr := `belong-to("org1", "org2", "org3")`
	parser := condition.NewParser(conditionStr)
	expr, err := parser.Parse()
	if err != nil {
		return TestResult{
			Passed:  false,
			Message: fmt.Sprintf("Failed to parse condition: %v", err),
		}
	}

	// Create evaluation context
	evalCtx := &condition.EvaluationContext{}

	// Test scenario 1: User belongs to one of the organizations
	user1 := &models.UserInfo{
		Company: "org2",
	}
	result1, err := expr.Evaluate(user1, evalCtx)
	if err != nil {
		return TestResult{
			Passed:  false,
			Message: fmt.Sprintf("Evaluation error: %v", err),
		}
	}
	if !result1 {
		return TestResult{
			Passed:  false,
			Message: "Expected true when user belongs to one of the organizations",
		}
	}

	// Test scenario 2: User does not belong to any organization
	user2 := &models.UserInfo{
		Company: "org4",
	}
	result2, err := expr.Evaluate(user2, evalCtx)
	if err != nil {
		return TestResult{
			Passed:  false,
			Message: fmt.Sprintf("Evaluation error: %v", err),
		}
	}
	if result2 {
		return TestResult{
			Passed:  false,
			Message: "Expected false when user does not belong to any organization",
		}
	}

	// Test with employee sync enabled
	evalCtx.ConfigQuerier = &testConfigQuerier{enabled: true}
	evalCtx.DatabaseQuerier = &testDatabaseQuerier{}

	// Scenario 3: Employee's department is in one of the organizations
	user3 := &models.UserInfo{
		EmployeeNumber: "emp123",
		Company:        "orgX", // Should not be used; department query should be used instead
	}
	result3, err := expr.Evaluate(user3, evalCtx)
	if err != nil {
		return TestResult{
			Passed:  false,
			Message: fmt.Sprintf("Evaluation error with employee sync: %v", err),
		}
	}
	if !result3 {
		return TestResult{
			Passed:  false,
			Message: "Expected true when employee department matches one organization",
		}
	}

	// Scenario 4: Employee's department is not in any organization
	user4 := &models.UserInfo{
		EmployeeNumber: "emp999",
		Company:        "orgX",
	}
	result4, err := expr.Evaluate(user4, evalCtx)
	if err != nil {
		return TestResult{
			Passed:  false,
			Message: fmt.Sprintf("Evaluation error with employee sync: %v", err),
		}
	}
	if result4 {
		return TestResult{
			Passed:  false,
			Message: "Expected false when employee department matches no organization",
		}
	}

	return TestResult{Passed: true}
}

// Mock ConfigQuerier for testing
type testConfigQuerier struct {
	enabled bool
}

func (t *testConfigQuerier) IsEmployeeSyncEnabled() bool {
	return t.enabled
}

// Mock DatabaseQuerier for testing
type testDatabaseQuerier struct{}

func (t *testDatabaseQuerier) QueryEmployeeDepartment(employeeNumber string) ([]string, error) {
	// Simulate database query
	if employeeNumber == "emp123" {
		return []string{"deptA", "org2", "deptC"}, nil // contains org2
	}
	return []string{"deptX", "deptY", "deptZ"}, nil
}
