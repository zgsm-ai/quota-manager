package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"quota-manager/internal/config"
	"quota-manager/internal/handlers"
	"quota-manager/internal/models"
	"quota-manager/internal/response"

	"github.com/gin-gonic/gin"
)

// APITestContext holds the HTTP test context
type APITestContext struct {
	*TestContext
	Router          *gin.Engine
	StrategyHandler *handlers.StrategyHandler
	QuotaHandler    *handlers.QuotaHandler
}

// setupAPITestContext creates an API test context with HTTP handlers
func setupAPITestContext(ctx *TestContext) *APITestContext {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create handlers
	strategyHandler := handlers.NewStrategyHandler(ctx.StrategyService)
	serverConfig := &config.ServerConfig{TokenHeader: "authorization"}
	quotaHandler := handlers.NewQuotaHandler(ctx.QuotaService, serverConfig)

	// Create router
	router := gin.New()

	// Setup routes
	quotaManager := router.Group("/quota-manager")
	{
		// Health check
		quotaManager.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, response.NewSuccessResponse(gin.H{"status": "ok"}, "Service is running"))
		})

		// API routes
		v1 := quotaManager.Group("/api/v1")
		{
			// Strategy management API
			strategies := v1.Group("/strategies")
			{
				strategies.POST("", strategyHandler.CreateStrategy)
				strategies.GET("", strategyHandler.GetStrategies)
				strategies.GET("/:id", strategyHandler.GetStrategy)
				strategies.PUT("/:id", strategyHandler.UpdateStrategy)
				strategies.DELETE("/:id", strategyHandler.DeleteStrategy)
				strategies.POST("/:id/enable", strategyHandler.EnableStrategy)
				strategies.POST("/:id/disable", strategyHandler.DisableStrategy)
				strategies.POST("/scan", strategyHandler.TriggerScan)
			}

			// Quota management API
			handlers.RegisterQuotaRoutes(v1, quotaHandler)
		}
	}

	return &APITestContext{
		TestContext:     ctx,
		Router:          router,
		StrategyHandler: strategyHandler,
		QuotaHandler:    quotaHandler,
	}
}

// testAPIHealthCheck tests the health check endpoint
func testAPIHealthCheck(ctx *TestContext) TestResult {
	apiCtx := setupAPITestContext(ctx)

	// Create request
	req, _ := http.NewRequest("GET", "/quota-manager/health", nil)
	w := httptest.NewRecorder()

	// Perform request
	apiCtx.Router.ServeHTTP(w, req)

	// Check status code
	if w.Code != http.StatusOK {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected status 200, got %d", w.Code)}
	}

	// Parse response
	var resp response.ResponseData
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to parse response: %v", err)}
	}

	// Verify response format
	if resp.Code != response.SuccessCode {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected code %s, got %s", response.SuccessCode, resp.Code)}
	}

	if !resp.Success {
		return TestResult{Passed: false, Message: "Expected success to be true"}
	}

	if resp.Message != "Service is running" {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected message 'Service is running', got '%s'", resp.Message)}
	}

	// Verify data field
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		return TestResult{Passed: false, Message: "Data field is not an object"}
	}

	if status, exists := data["status"]; !exists || status != "ok" {
		return TestResult{Passed: false, Message: "Expected data.status to be 'ok'"}
	}

	return TestResult{Passed: true, Message: "API Health Check Test Succeeded"}
}

// testAPICreateStrategy tests strategy creation endpoint
func testAPICreateStrategy(ctx *TestContext) TestResult {
	apiCtx := setupAPITestContext(ctx)

	// Create strategy request
	strategy := map[string]interface{}{
		"name":      "api-test-strategy",
		"title":     "API Test Strategy",
		"type":      "single",
		"amount":    100,
		"model":     "gpt-3.5-turbo",
		"condition": "",
		"status":    true,
	}

	body, _ := json.Marshal(strategy)
	req, _ := http.NewRequest("POST", "/quota-manager/api/v1/strategies", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Perform request
	apiCtx.Router.ServeHTTP(w, req)

	// Check status code
	if w.Code != http.StatusCreated {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected status 201, got %d", w.Code)}
	}

	// Parse response
	var resp response.ResponseData
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to parse response: %v", err)}
	}

	// Verify response format
	if resp.Code != response.SuccessCode {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected code %s, got %s", response.SuccessCode, resp.Code)}
	}

	if !resp.Success {
		return TestResult{Passed: false, Message: "Expected success to be true"}
	}

	if resp.Message != "Strategy created successfully" {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected message 'Strategy created successfully', got '%s'", resp.Message)}
	}

	// Verify data field contains strategy with ID
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		return TestResult{Passed: false, Message: "Data field is not an object"}
	}

	if _, exists := data["id"]; !exists {
		return TestResult{Passed: false, Message: "Expected data to contain id field"}
	}

	if data["name"] != "api-test-strategy" {
		return TestResult{Passed: false, Message: "Strategy name mismatch in response data"}
	}

	return TestResult{Passed: true, Message: "API Create Strategy Test Succeeded"}
}

// testAPICreateStrategyInvalidData tests strategy creation with invalid data
func testAPICreateStrategyInvalidData(ctx *TestContext) TestResult {
	apiCtx := setupAPITestContext(ctx)

	// Create invalid strategy request (missing required fields)
	strategy := map[string]interface{}{
		"name": "", // Invalid: empty name
		"type": "invalid-type",
	}

	body, _ := json.Marshal(strategy)
	req, _ := http.NewRequest("POST", "/quota-manager/api/v1/strategies", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Perform request
	apiCtx.Router.ServeHTTP(w, req)

	// Check status code (should be 400 for invalid request parameters)
	if w.Code != http.StatusBadRequest {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected status 400, got %d", w.Code)}
	}

	// Parse response
	var resp response.ResponseData
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to parse response: %v", err)}
	}

	// Verify error response format
	if resp.Code != response.BadRequestCode {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected code %s, got %s", response.BadRequestCode, resp.Code)}
	}

	if resp.Success {
		return TestResult{Passed: false, Message: "Expected success to be false"}
	}

	if resp.Data != nil {
		return TestResult{Passed: false, Message: "Expected data to be nil for error response"}
	}

	return TestResult{Passed: true, Message: "API Create Strategy Invalid Data Test Succeeded"}
}

// testAPIGetStrategyNotFound tests getting a non-existent strategy
func testAPIGetStrategyNotFound(ctx *TestContext) TestResult {
	apiCtx := setupAPITestContext(ctx)

	// Request non-existent strategy
	req, _ := http.NewRequest("GET", "/quota-manager/api/v1/strategies/99999", nil)
	w := httptest.NewRecorder()

	// Perform request
	apiCtx.Router.ServeHTTP(w, req)

	// Check status code
	if w.Code != http.StatusNotFound {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected status 404, got %d", w.Code)}
	}

	// Parse response
	var resp response.ResponseData
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to parse response: %v", err)}
	}

	// Verify error response format
	if resp.Code != response.StrategyNotFoundCode {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected code %s, got %s", response.StrategyNotFoundCode, resp.Code)}
	}

	if resp.Success {
		return TestResult{Passed: false, Message: "Expected success to be false"}
	}

	return TestResult{Passed: true, Message: "API Get Strategy Not Found Test Succeeded"}
}

// testAPIInvalidStrategyID tests endpoints with invalid strategy ID
func testAPIInvalidStrategyID(ctx *TestContext) TestResult {
	apiCtx := setupAPITestContext(ctx)

	// Test with invalid ID format
	req, _ := http.NewRequest("GET", "/quota-manager/api/v1/strategies/invalid-id", nil)
	w := httptest.NewRecorder()

	// Perform request
	apiCtx.Router.ServeHTTP(w, req)

	// Check status code
	if w.Code != http.StatusBadRequest {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected status 400, got %d", w.Code)}
	}

	// Parse response
	var resp response.ResponseData
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to parse response: %v", err)}
	}

	// Verify error response format
	if resp.Code != response.InvalidStrategyIDCode {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected code %s, got %s", response.InvalidStrategyIDCode, resp.Code)}
	}

	if resp.Success {
		return TestResult{Passed: false, Message: "Expected success to be false"}
	}

	if resp.Message != "Invalid strategy ID format" {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected message 'Invalid strategy ID format', got '%s'", resp.Message)}
	}

	return TestResult{Passed: true, Message: "API Invalid Strategy ID Test Succeeded"}
}

// testAPIGetStrategies tests getting strategy list
func testAPIGetStrategies(ctx *TestContext) TestResult {
	apiCtx := setupAPITestContext(ctx)

	// Create a strategy first
	strategy := &models.QuotaStrategy{
		Name:      "list-test-strategy",
		Title:     "List Test Strategy",
		Type:      "single",
		Amount:    50,
		Model:     "test-model",
		Condition: "true()",
		Status:    true,
	}
	if err := ctx.StrategyService.CreateStrategy(strategy); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to create test strategy: %v", err)}
	}

	// Request strategy list
	req, _ := http.NewRequest("GET", "/quota-manager/api/v1/strategies", nil)
	w := httptest.NewRecorder()

	// Perform request
	apiCtx.Router.ServeHTTP(w, req)

	// Check status code
	if w.Code != http.StatusOK {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected status 200, got %d", w.Code)}
	}

	// Parse response
	var resp response.ResponseData
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to parse response: %v", err)}
	}

	// Verify response format
	if resp.Code != response.SuccessCode {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected code %s, got %s", response.SuccessCode, resp.Code)}
	}

	if !resp.Success {
		return TestResult{Passed: false, Message: "Expected success to be true"}
	}

	if resp.Message != "Strategies retrieved successfully" {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected message 'Strategies retrieved successfully', got '%s'", resp.Message)}
	}

	// Verify data structure
	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		return TestResult{Passed: false, Message: "Data field is not an object"}
	}

	if _, exists := data["strategies"]; !exists {
		return TestResult{Passed: false, Message: "Expected data to contain strategies field"}
	}

	if _, exists := data["total"]; !exists {
		return TestResult{Passed: false, Message: "Expected data to contain total field"}
	}

	return TestResult{Passed: true, Message: "API Get Strategies Test Succeeded"}
}

// testAPIQuotaUnauthorized tests quota endpoint without token
func testAPIQuotaUnauthorized(ctx *TestContext) TestResult {
	apiCtx := setupAPITestContext(ctx)

	// Request without authorization header
	req, _ := http.NewRequest("GET", "/quota-manager/api/v1/quota", nil)
	w := httptest.NewRecorder()

	// Perform request
	apiCtx.Router.ServeHTTP(w, req)

	// Check status code
	if w.Code != http.StatusUnauthorized {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected status 401, got %d", w.Code)}
	}

	// Parse response
	var resp response.ResponseData
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to parse response: %v", err)}
	}

	// Verify error response format
	if resp.Code != response.TokenInvalidCode {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected code %s, got %s", response.TokenInvalidCode, resp.Code)}
	}

	if resp.Success {
		return TestResult{Passed: false, Message: "Expected success to be false"}
	}

	return TestResult{Passed: true, Message: "API Quota Unauthorized Test Succeeded"}
}

// Helper function to create a valid JWT token for testing
func createTestJWTToken() string {
	// Create a simple base64url encoded JSON without signature (for testing)
	userInfo := map[string]interface{}{
		"id":      "test-user-123",
		"name":    "Test User",
		"staffID": "emp001",
		"github":  "testuser",
		"phone":   "13800138000",
	}

	payload, _ := json.Marshal(userInfo)
	return "Bearer " + string(payload) // Simplified for testing
}

// testAPICreateStrategyInvalidCondition tests strategy creation with invalid condition expression
func testAPICreateStrategyInvalidCondition(ctx *TestContext) TestResult {
	apiCtx := setupAPITestContext(ctx)

	// Test with valid strategy data but invalid condition
	strategy := map[string]interface{}{
		"name":      "invalid-condition-test",
		"title":     "Invalid Condition Test Strategy",
		"type":      "single",
		"amount":    100,
		"model":     "gpt-3.5-turbo",
		"condition": "invalid-function(\"test\")", // Invalid function name
		"status":    true,
	}

	body, _ := json.Marshal(strategy)
	req, _ := http.NewRequest("POST", "/quota-manager/api/v1/strategies", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Perform request
	apiCtx.Router.ServeHTTP(w, req)

	// Check status code (should be 400 for invalid condition)
	if w.Code != http.StatusBadRequest {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected status 400, got %d", w.Code)}
	}

	// Parse response
	var resp response.ResponseData
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		return TestResult{Passed: false, Message: fmt.Sprintf("Failed to parse response: %v", err)}
	}

	// Verify error response format
	if resp.Code != response.BadRequestCode {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected code %s, got %s", response.BadRequestCode, resp.Code)}
	}

	if resp.Success {
		return TestResult{Passed: false, Message: "Expected success to be false"}
	}

	if resp.Data != nil {
		return TestResult{Passed: false, Message: "Expected data to be nil for error response"}
	}

	// Verify the error message contains condition validation error
	if !strings.Contains(resp.Message, "Invalid condition expression") {
		return TestResult{Passed: false, Message: fmt.Sprintf("Expected error message to contain 'Invalid condition expression', got '%s'", resp.Message)}
	}

	return TestResult{Passed: true, Message: "API Create Strategy Invalid Condition Test Succeeded"}
}
