package main

import (
	"fmt"
	"quota-manager/internal/models"
	"quota-manager/internal/services"
	"quota-manager/internal/validation"
	"strings"
	"time"
)

// testSchemaValidation tests the new schema validation functionality
func testSchemaValidation(ctx *TestContext) TestResult {
	// Test QuotaStrategy validation

	// Test valid strategy
	validStrategy := models.QuotaStrategy{
		Name:      "test-strategy",
		Title:     "Test Strategy",
		Type:      "single",
		Amount:    100,
		Model:     "gpt-3.5-turbo",
		Condition: "true()",
	}

	if err := validation.ValidateStruct(&validStrategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Valid strategy should pass validation: %v", err)}
	}

	// Test invalid strategy - empty name
	invalidStrategy1 := models.QuotaStrategy{
		Name:   "", // Invalid: empty name
		Title:  "Test Strategy",
		Type:   "single",
		Amount: 100,
	}

	if err := validation.ValidateStruct(&invalidStrategy1); err == nil {
		return TestResult{Passed: false, Message: "Strategy with empty name should fail validation"}
	}

	// Test invalid strategy - invalid type
	invalidStrategy2 := models.QuotaStrategy{
		Name:   "test-strategy",
		Title:  "Test Strategy",
		Type:   "invalid-type", // Invalid: not 'single' or 'periodic'
		Amount: 100,
	}

	if err := validation.ValidateStruct(&invalidStrategy2); err == nil {
		return TestResult{Passed: false, Message: "Strategy with invalid type should fail validation"}
	}

	// Test valid strategy - negative amount (allowed by business logic)
	validStrategy3 := models.QuotaStrategy{
		Name:   "test-strategy-negative",
		Title:  "Test Strategy Negative",
		Type:   "single",
		Amount: -10, // Valid: negative amount is allowed
	}

	if err := validation.ValidateStruct(&validStrategy3); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Strategy with negative amount should pass validation: %v", err)}
	}

	// Test periodic strategy with valid cron expression
	validPeriodicStrategy := models.QuotaStrategy{
		Name:         "periodic-test",
		Title:        "Periodic Test Strategy",
		Type:         "periodic",
		Amount:       50,
		PeriodicExpr: "0 0 8 * * *", // Valid: daily at 8 AM
	}

	if err := validation.ValidateStruct(&validPeriodicStrategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Valid periodic strategy should pass validation: %v", err)}
	}

	// Test periodic strategy with invalid cron expression
	invalidPeriodicStrategy := models.QuotaStrategy{
		Name:         "periodic-test",
		Title:        "Periodic Test Strategy",
		Type:         "periodic",
		Amount:       50,
		PeriodicExpr: "invalid-cron", // Invalid: bad cron expression
	}

	if err := validation.ValidateStruct(&invalidPeriodicStrategy); err == nil {
		return TestResult{Passed: false, Message: "Periodic strategy with invalid cron should fail validation"}
	}

	return TestResult{Passed: true, Message: "Schema validation test succeeded"}
}

// testTransferRequestValidation tests validation of transfer request structures
func testTransferRequestValidation(ctx *TestContext) TestResult {
	// Test valid transfer out request
	validTransferOut := services.TransferOutRequest{
		ReceiverID: "123e4567-e89b-12d3-a456-426614174000", // Valid UUID
		QuotaList: []services.TransferQuotaItem{
			{
				Amount:     10,
				ExpiryDate: time.Now().Add(24 * time.Hour),
			},
		},
	}

	if err := validation.ValidateStruct(&validTransferOut); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Valid transfer out request should pass validation: %v", err)}
	}

	// Test invalid transfer out request - invalid UUID
	invalidTransferOut1 := services.TransferOutRequest{
		ReceiverID: "invalid-uuid", // Invalid: not a UUID
		QuotaList: []services.TransferQuotaItem{
			{
				Amount:     10,
				ExpiryDate: time.Now().Add(24 * time.Hour),
			},
		},
	}

	if err := validation.ValidateStruct(&invalidTransferOut1); err == nil {
		return TestResult{Passed: false, Message: "Transfer out request with invalid UUID should fail validation"}
	}

	// Test invalid transfer out request - empty quota list
	invalidTransferOut2 := services.TransferOutRequest{
		ReceiverID: "123e4567-e89b-12d3-a456-426614174000",
		QuotaList:  []services.TransferQuotaItem{}, // Invalid: empty list
	}

	if err := validation.ValidateStruct(&invalidTransferOut2); err == nil {
		return TestResult{Passed: false, Message: "Transfer out request with empty quota list should fail validation"}
	}

	// Test invalid transfer out request - negative amount
	invalidTransferOut3 := services.TransferOutRequest{
		ReceiverID: "123e4567-e89b-12d3-a456-426614174000",
		QuotaList: []services.TransferQuotaItem{
			{
				Amount:     -10, // Invalid: negative amount
				ExpiryDate: time.Now().Add(24 * time.Hour),
			},
		},
	}

	if err := validation.ValidateStruct(&invalidTransferOut3); err == nil {
		return TestResult{Passed: false, Message: "Transfer out request with negative amount should fail validation"}
	}

	// Test valid transfer in request
	validTransferIn := services.TransferInRequest{
		VoucherCode: "valid-voucher-code-12345", // Valid: meets length requirements
	}

	if err := validation.ValidateStruct(&validTransferIn); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Valid transfer in request should pass validation: %v", err)}
	}

	// Test invalid transfer in request - too short voucher code
	invalidTransferIn1 := services.TransferInRequest{
		VoucherCode: "short", // Invalid: too short
	}

	if err := validation.ValidateStruct(&invalidTransferIn1); err == nil {
		return TestResult{Passed: false, Message: "Transfer in request with short voucher code should fail validation"}
	}

	// Test invalid transfer in request - empty voucher code
	invalidTransferIn2 := services.TransferInRequest{
		VoucherCode: "", // Invalid: empty
	}

	if err := validation.ValidateStruct(&invalidTransferIn2); err == nil {
		return TestResult{Passed: false, Message: "Transfer in request with empty voucher code should fail validation"}
	}

	return TestResult{Passed: true, Message: "Transfer request validation test succeeded"}
}

// testCustomValidators tests the custom validation functions
func testCustomValidators(ctx *TestContext) TestResult {
	// Test positive validator
	type TestStruct struct {
		PositiveInt int `validate:"gt=0"`
	}

	validPositive := TestStruct{PositiveInt: 10}
	if err := validation.ValidateStruct(&validPositive); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Positive integer should pass validation: %v", err)}
	}

	invalidPositive := TestStruct{PositiveInt: -5}
	if err := validation.ValidateStruct(&invalidPositive); err == nil {
		return TestResult{Passed: false, Message: "Negative integer should fail positive validation"}
	}

	// Test UUID validator
	type UUIDStruct struct {
		ID string `validate:"uuid"`
	}

	validUUID := UUIDStruct{ID: "123e4567-e89b-12d3-a456-426614174000"}
	if err := validation.ValidateStruct(&validUUID); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Valid UUID should pass validation: %v", err)}
	}

	invalidUUID := UUIDStruct{ID: "invalid-uuid"}
	if err := validation.ValidateStruct(&invalidUUID); err == nil {
		return TestResult{Passed: false, Message: "Invalid UUID should fail validation"}
	}

	// Test cron validator
	type CronStruct struct {
		Expression string `validate:"cron"`
	}

	validCron := CronStruct{Expression: "0 0 8 * * *"}
	if err := validation.ValidateStruct(&validCron); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Valid cron expression should pass validation: %v", err)}
	}

	invalidCron := CronStruct{Expression: "invalid-cron"}
	if err := validation.ValidateStruct(&invalidCron); err == nil {
		return TestResult{Passed: false, Message: "Invalid cron expression should fail validation"}
	}

	// Test strategy_type validator
	type StrategyTypeStruct struct {
		Type string `validate:"oneof=single periodic"`
	}

	validType1 := StrategyTypeStruct{Type: "single"}
	if err := validation.ValidateStruct(&validType1); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("'single' strategy type should pass validation: %v", err)}
	}

	validType2 := StrategyTypeStruct{Type: "periodic"}
	if err := validation.ValidateStruct(&validType2); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("'periodic' strategy type should pass validation: %v", err)}
	}

	invalidType := StrategyTypeStruct{Type: "invalid"}
	if err := validation.ValidateStruct(&invalidType); err == nil {
		return TestResult{Passed: false, Message: "Invalid strategy type should fail validation"}
	}

	return TestResult{Passed: true, Message: "Custom validators test succeeded"}
}

// testErrorMessages tests that validation errors provide helpful messages
func testErrorMessages(ctx *TestContext) TestResult {
	invalidStrategy := models.QuotaStrategy{
		Name:   "",        // Invalid: empty name
		Type:   "invalid", // Invalid: bad type
		Amount: -10,       // Invalid: negative amount
	}

	err := validation.ValidateStruct(&invalidStrategy)
	if err == nil {
		return TestResult{Passed: false, Message: "Invalid strategy should produce validation errors"}
	}

	// Check that error message contains field information
	errorMsg := err.Error()
	if !containsAny(errorMsg, []string{"name", "type", "amount"}) {
		return TestResult{Passed: false, Message: fmt.Sprintf("Error message should contain field names: %s", errorMsg)}
	}

	return TestResult{Passed: true, Message: "Error messages test succeeded"}
}

// Helper function to check if string contains any of the given substrings
func containsAny(str string, substrings []string) bool {
	for _, substr := range substrings {
		if strings.Contains(str, substr) {
			return true
		}
	}
	return false
}
