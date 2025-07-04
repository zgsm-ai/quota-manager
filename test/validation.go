package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"quota-manager/internal/validation"
)

// testValidationUtils tests the validation utility functions
func testValidationUtils(ctx *TestContext) TestResult {
	// Test UUID validation
	validUUIDs := []string{
		"123e4567-e89b-12d3-a456-426614174000",
		"550e8400-e29b-41d4-a716-446655440000",
		"6ba7b810-9dad-11d1-80b4-00c04fd430c8",
	}

	invalidUUIDs := []string{
		"",
		"invalid-uuid",
		"123e4567-e89b-12d3-a456",
		"123e4567-e89b-12d3-a456-42661417400g", // invalid character
		"123e4567e89b12d3a456426614174000",     // no hyphens
	}

	for _, uuid := range validUUIDs {
		if !validation.IsValidUUID(uuid) {
			return TestResult{Passed: false, Message: fmt.Sprintf("Valid UUID %s failed validation", uuid)}
		}
	}

	for _, uuid := range invalidUUIDs {
		if validation.IsValidUUID(uuid) {
			return TestResult{Passed: false, Message: fmt.Sprintf("Invalid UUID %s passed validation", uuid)}
		}
	}

	// Test cron expression validation
	validCronExprs := []string{
		"0 * * * * *",   // every minute
		"*/5 * * * * *", // every 5 seconds
		"0 0 * * * *",   // every hour
		"0 0 0 * * *",   // every day at midnight
		"0 0 8 * * *",   // every day at 8 AM
		"0 0 8 * * 1",   // every Monday at 8 AM
		"0 0 0 1 * *",   // first day of every month
	}

	invalidCronExprs := []string{
		"",
		"invalid-cron",
		"* * * *",      // too few fields
		"60 * * * * *", // invalid second (>59)
		"* 60 * * * *", // invalid minute (>59)
		"* * 25 * * *", // invalid hour (>23)
		"* * * 32 * *", // invalid day of month (>31)
		"* * * * 13 *", // invalid month (>12)
		"* * * * * 8",  // invalid day of week (>7)
	}

	for _, expr := range validCronExprs {
		if err := validation.IsValidCronExpr(expr); err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Valid cron expression %s failed validation: %v", expr, err)}
		}
	}

	for _, expr := range invalidCronExprs {
		if err := validation.IsValidCronExpr(expr); err == nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Invalid cron expression %s passed validation", expr)}
		}
	}

	// Test positive integer validation
	validIntegers := []interface{}{1, 10, 100, int32(50), int64(200), float64(25), "42"}
	invalidIntegers := []interface{}{0, -1, -10, float64(-5), float64(3.14), "0", "-1", "abc", ""}

	for _, val := range validIntegers {
		if !validation.IsPositiveInteger(val) {
			return TestResult{Passed: false, Message: fmt.Sprintf("Valid positive integer %v failed validation", val)}
		}
	}

	for _, val := range invalidIntegers {
		if validation.IsPositiveInteger(val) {
			return TestResult{Passed: false, Message: fmt.Sprintf("Invalid positive integer %v passed validation", val)}
		}
	}

	// Test strategy type validation
	if !validation.IsValidStrategyType("single") {
		return TestResult{Passed: false, Message: "Valid strategy type 'single' failed validation"}
	}
	if !validation.IsValidStrategyType("periodic") {
		return TestResult{Passed: false, Message: "Valid strategy type 'periodic' failed validation"}
	}
	if validation.IsValidStrategyType("invalid") {
		return TestResult{Passed: false, Message: "Invalid strategy type 'invalid' passed validation"}
	}
	if validation.IsValidStrategyType("") {
		return TestResult{Passed: false, Message: "Empty strategy type passed validation"}
	}

	return TestResult{Passed: true, Message: "Validation Utils Test Succeeded"}
}

// testAPIValidationCreateStrategy tests strategy creation parameter validation
func testAPIValidationCreateStrategy(ctx *TestContext) TestResult {
	apiCtx := setupAPITestContext(ctx)

	// Test cases for strategy creation validation
	testCases := []struct {
		name           string
		strategy       map[string]interface{}
		expectedStatus int
		expectedError  string
	}{
		{
			name: "valid strategy",
			strategy: map[string]interface{}{
				"name":      "valid-strategy",
				"title":     "Valid Strategy",
				"type":      "single",
				"amount":    100,
				"model":     "gpt-3.5-turbo",
				"condition": "github-star(\"test\")",
				"status":    true,
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "empty name",
			strategy: map[string]interface{}{
				"name":   "",
				"title":  "Valid Strategy",
				"type":   "single",
				"amount": 100,
				"status": true,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "strategy name is required",
		},
		{
			name: "empty title",
			strategy: map[string]interface{}{
				"name":   "valid-strategy",
				"title":  "",
				"type":   "single",
				"amount": 100,
				"status": true,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "strategy title is required",
		},
		{
			name: "invalid strategy type",
			strategy: map[string]interface{}{
				"name":   "valid-strategy",
				"title":  "Valid Strategy",
				"type":   "invalid",
				"amount": 100,
				"status": true,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Strategy type must be 'single' or 'periodic'",
		},
		{
			name: "zero amount",
			strategy: map[string]interface{}{
				"name":   "valid-strategy",
				"title":  "Valid Strategy",
				"type":   "single",
				"amount": 0,
				"status": true,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Strategy amount must be a positive integer",
		},
		{
			name: "negative amount",
			strategy: map[string]interface{}{
				"name":   "valid-strategy",
				"title":  "Valid Strategy",
				"type":   "single",
				"amount": -10,
				"status": true,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Strategy amount must be a positive integer",
		},
		{
			name: "periodic without periodic_expr",
			strategy: map[string]interface{}{
				"name":   "valid-strategy",
				"title":  "Valid Strategy",
				"type":   "periodic",
				"amount": 100,
				"status": true,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "periodic expression is required",
		},
		{
			name: "periodic with invalid cron expression",
			strategy: map[string]interface{}{
				"name":          "valid-strategy",
				"title":         "Valid Strategy",
				"type":          "periodic",
				"amount":        100,
				"periodic_expr": "invalid-cron",
				"status":        true,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid periodic expression",
		},
		{
			name: "valid periodic strategy",
			strategy: map[string]interface{}{
				"name":          "valid-periodic-strategy",
				"title":         "Valid Periodic Strategy",
				"type":          "periodic",
				"amount":        50,
				"periodic_expr": "0 0 8 * * *",
				"condition":     "github-star(\"test\")",
				"status":        true,
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "invalid condition expression",
			strategy: map[string]interface{}{
				"name":      "invalid-condition-strategy",
				"title":     "Invalid Condition Strategy",
				"type":      "single",
				"amount":    100,
				"condition": "invalid-function(\"test\")",
				"status":    true,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid condition expression",
		},
	}

	for _, tc := range testCases {
		body, _ := json.Marshal(tc.strategy)
		req, _ := http.NewRequest("POST", "/quota-manager/api/v1/strategies", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		apiCtx.Router.ServeHTTP(w, req)

		if w.Code != tc.expectedStatus {
			return TestResult{Passed: false, Message: fmt.Sprintf("Test case '%s': expected status %d, got %d", tc.name, tc.expectedStatus, w.Code)}
		}

		if tc.expectedError != "" {
			var response map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				return TestResult{Passed: false, Message: fmt.Sprintf("Test case '%s': failed to parse response: %v", tc.name, err)}
			}

			if message, ok := response["message"].(string); !ok || message == "" {
				return TestResult{Passed: false, Message: fmt.Sprintf("Test case '%s': no error message in response", tc.name)}
			}
		}
	}

	return TestResult{Passed: true, Message: "API Validation Create Strategy Test Succeeded"}
}

// testAPIValidationTransferOut tests quota transfer out parameter validation
func testAPIValidationTransferOut(ctx *TestContext) TestResult {
	apiCtx := setupAPITestContext(ctx)

	// Test JWT token with valid user ID (complete JWT format: header.payload.signature)
	testToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1bml2ZXJzYWxfaWQiOiJ1c2VyMDAxIiwibmFtZSI6IkpvaG4gRG9lIiwic3RhZmZJRCI6ImVtcDAwMSIsImdpdGh1YiI6ImpvaG5kb2UiLCJwaG9uZSI6IjEzODAwMTM4MDAxIn0.signature"

	testCases := []struct {
		name           string
		transferData   map[string]interface{}
		expectedStatus int
		expectedError  string
	}{
		{
			name: "valid transfer request",
			transferData: map[string]interface{}{
				"receiver_id": "123e4567-e89b-12d3-a456-426614174000",
				"quota_list": []map[string]interface{}{
					{
						"amount":      10,
						"expiry_date": "2025-06-30T23:59:59Z",
					},
				},
			},
			expectedStatus: http.StatusBadRequest, // Will fail due to insufficient quota, but validation passes
		},
		{
			name: "empty receiver_id",
			transferData: map[string]interface{}{
				"receiver_id": "",
				"quota_list": []map[string]interface{}{
					{
						"amount":      10,
						"expiry_date": "2025-06-30T23:59:59Z",
					},
				},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "receiver_id is required",
		},
		{
			name: "invalid receiver_id UUID format",
			transferData: map[string]interface{}{
				"receiver_id": "invalid-uuid",
				"quota_list": []map[string]interface{}{
					{
						"amount":      10,
						"expiry_date": "2025-06-30T23:59:59Z",
					},
				},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "receiver_id must be a valid UUID format",
		},
		{
			name: "empty quota list",
			transferData: map[string]interface{}{
				"receiver_id": "123e4567-e89b-12d3-a456-426614174000",
				"quota_list":  []map[string]interface{}{},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Quota list cannot be empty",
		},
		{
			name: "zero amount in quota list",
			transferData: map[string]interface{}{
				"receiver_id": "123e4567-e89b-12d3-a456-426614174000",
				"quota_list": []map[string]interface{}{
					{
						"amount":      0,
						"expiry_date": "2025-06-30T23:59:59Z",
					},
				},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "amount must be a positive integer",
		},
		{
			name: "negative amount in quota list",
			transferData: map[string]interface{}{
				"receiver_id": "123e4567-e89b-12d3-a456-426614174000",
				"quota_list": []map[string]interface{}{
					{
						"amount":      -5,
						"expiry_date": "2025-06-30T23:59:59Z",
					},
				},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "amount must be a positive integer",
		},
	}

	for _, tc := range testCases {
		body, _ := json.Marshal(tc.transferData)
		req, _ := http.NewRequest("POST", "/quota-manager/api/v1/quota/transfer-out", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+testToken)
		w := httptest.NewRecorder()

		apiCtx.Router.ServeHTTP(w, req)

		if w.Code != tc.expectedStatus {
			return TestResult{Passed: false, Message: fmt.Sprintf("Test case '%s': expected status %d, got %d", tc.name, tc.expectedStatus, w.Code)}
		}

		if tc.expectedError != "" {
			var response map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				return TestResult{Passed: false, Message: fmt.Sprintf("Test case '%s': failed to parse response: %v", tc.name, err)}
			}

			if message, ok := response["message"].(string); !ok || message == "" {
				return TestResult{Passed: false, Message: fmt.Sprintf("Test case '%s': no error message in response", tc.name)}
			}
		}
	}

	return TestResult{Passed: true, Message: "API Validation Transfer Out Test Succeeded"}
}

// testAPIValidationUserID tests user ID validation in admin endpoints
func testAPIValidationUserID(ctx *TestContext) TestResult {
	apiCtx := setupAPITestContext(ctx)

	testCases := []struct {
		name           string
		userID         string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "valid UUID",
			userID:         "123e4567-e89b-12d3-a456-426614174000",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid UUID format",
			userID:         "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "user_id must be a valid UUID format",
		},
		{
			name:           "empty user ID",
			userID:         "",
			expectedStatus: http.StatusBadRequest, // Now properly handles empty user_id with 400 error
			expectedError:  "user_id is required and cannot be empty",
		},
	}

	for _, tc := range testCases {
		var url string
		if tc.userID == "" {
			url = "/quota-manager/api/v1/quota/audit/"
		} else {
			url = fmt.Sprintf("/quota-manager/api/v1/quota/audit/%s", tc.userID)
		}

		req, _ := http.NewRequest("GET", url, nil)
		w := httptest.NewRecorder()

		apiCtx.Router.ServeHTTP(w, req)

		if w.Code != tc.expectedStatus {
			return TestResult{Passed: false, Message: fmt.Sprintf("Test case '%s': expected status %d, got %d", tc.name, tc.expectedStatus, w.Code)}
		}

		if tc.expectedError != "" {
			var response map[string]interface{}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				return TestResult{Passed: false, Message: fmt.Sprintf("Test case '%s': failed to parse response: %v", tc.name, err)}
			}

			if message, ok := response["message"].(string); !ok || message == "" {
				return TestResult{Passed: false, Message: fmt.Sprintf("Test case '%s': no error message in response", tc.name)}
			}
		}
	}

	return TestResult{Passed: true, Message: "API Validation User ID Test Succeeded"}
}
