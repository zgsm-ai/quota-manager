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
				"name":   "empty-title-strategy",
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
				"name":   "invalid-type-strategy",
				"title":  "Invalid Type Strategy",
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
				"name":   "zero-amount-strategy",
				"title":  "Zero Amount Strategy",
				"type":   "single",
				"amount": 0,
				"status": true,
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "negative amount",
			strategy: map[string]interface{}{
				"name":   "negative-amount-strategy",
				"title":  "Negative Amount Strategy",
				"type":   "single",
				"amount": -10,
				"status": true,
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "periodic without periodic_expr",
			strategy: map[string]interface{}{
				"name":   "periodic-no-expr-strategy",
				"title":  "Periodic No Expr Strategy",
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
				"name":          "periodic-invalid-cron-strategy",
				"title":         "Periodic Invalid Cron Strategy",
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
			expectedError:  "receiver_i_d: ReceiverID is required",
		},
		{
			name: "receiver_id with whitespace",
			transferData: map[string]interface{}{
				"receiver_id": " 123e4567-e89b-12d3-a456-426614174000",
				"quota_list": []map[string]interface{}{
					{
						"amount":      10,
						"expiry_date": "2025-06-30T23:59:59Z",
					},
				},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Transfer validation failed: quota not found for expiry date 2025-06-30 23:59:59 +0000 UTC",
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
			expectedError:  "receiver_i_d: ReceiverID must be a valid UUID format",
		},
		{
			name: "empty quota list",
			transferData: map[string]interface{}{
				"receiver_id": "123e4567-e89b-12d3-a456-426614174000",
				"quota_list":  []map[string]interface{}{},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "quota_list: QuotaList must be at least 1 characters",
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
			expectedError:  "amount: Amount is required",
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
			expectedError:  "amount: Amount is invalid",
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

			if message, ok := response["message"].(string); !ok || message == "" || message != tc.expectedError {
				return TestResult{Passed: false, Message: fmt.Sprintf("Test case '%s': error message '%s' is not expected", tc.name, message)}
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

// testValidatePageParams tests the validation of page and pageSize parameters
func testValidatePageParams(ctx *TestContext) TestResult {

	testCases := []struct {
		name     string
		page     int
		pageSize int
		expPage  int
		expSize  int
		expErr   bool
	}{
		{"both valid", 2, 20, 2, 20, false},
		{"page zero", 0, 20, 1, 20, false},
		{"page negative", -5, 20, 1, 20, false},
		{"pageSize zero", 2, 0, 2, 10, false},
		{"pageSize negative", 2, -10, 2, 10, false},
		{"both zero", 0, 0, 1, 10, false},
		{"both negative", -1, -1, 1, 10, false},
		{"pageSize too large", 1, 200, 1, 200, false},
	}

	for _, tc := range testCases {
		page, size, err := validation.ValidatePageParams(tc.page, tc.pageSize)
		if tc.expErr {
			if err == nil {
				return TestResult{Passed: false, Message: fmt.Sprintf("Test '%s': expected error but got nil", tc.name)}
			}
			continue
		}
		if err != nil {
			return TestResult{Passed: false, Message: fmt.Sprintf("Test '%s': unexpected error: %v", tc.name, err)}
		}
		if page != tc.expPage || size != tc.expSize {
			return TestResult{Passed: false, Message: fmt.Sprintf("Test '%s': expected (page,pageSize)=(%d,%d), got (%d,%d)", tc.name, tc.expPage, tc.expSize, page, size)}
		}
	}

	return TestResult{Passed: true, Message: "ValidatePageParams Test Succeeded"}
}
