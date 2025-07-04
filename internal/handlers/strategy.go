package handlers

import (
	"net/http"
	"quota-manager/internal/condition"
	"quota-manager/internal/models"
	"quota-manager/internal/response"
	"quota-manager/internal/services"
	"quota-manager/internal/validation"
	"strconv"

	"github.com/gin-gonic/gin"
)

type StrategyHandler struct {
	service *services.StrategyService
}

func NewStrategyHandler(service *services.StrategyService) *StrategyHandler {
	return &StrategyHandler{service: service}
}

// CreateStrategy creates a new strategy
func (h *StrategyHandler) CreateStrategy(c *gin.Context) {
	var strategy models.QuotaStrategy
	if err := c.ShouldBindJSON(&strategy); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "Invalid request body: "+err.Error()))
		return
	}

	// Validate required fields - these are client-side validation errors, should return 400
	if err := validation.ValidateRequiredString(strategy.Name, "strategy name"); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, err.Error()))
		return
	}

	if err := validation.ValidateRequiredString(strategy.Title, "strategy title"); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, err.Error()))
		return
	}

	// Validate strategy name length (1-100 characters)
	if err := validation.ValidateStringLength(strategy.Name, "strategy name", 1, 100); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, err.Error()))
		return
	}

	// Validate strategy title length (1-200 characters)
	if err := validation.ValidateStringLength(strategy.Title, "strategy title", 1, 200); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, err.Error()))
		return
	}

	// Validate strategy type
	if !validation.IsValidStrategyType(strategy.Type) {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "Strategy type must be 'single' or 'periodic'"))
		return
	}

	// Validate amount is positive integer
	if !validation.IsPositiveInteger(strategy.Amount) {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "Strategy amount must be a positive integer"))
		return
	}

	// For periodic strategies, periodic_expr is required and must be valid cron expression
	if strategy.Type == "periodic" {
		if err := validation.ValidateRequiredString(strategy.PeriodicExpr, "periodic expression"); err != nil {
			c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, err.Error()))
			return
		}

		if err := validation.IsValidCronExpr(strategy.PeriodicExpr); err != nil {
			c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "Invalid periodic expression: "+err.Error()))
			return
		}
	}

	// Validate model field if provided
	if strategy.Model != "" {
		if err := validation.ValidateStringLength(strategy.Model, "model", 1, 100); err != nil {
			c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, err.Error()))
			return
		}
	}

	// Validate condition expression syntax (only if condition is not empty)
	if strategy.Condition != "" {
		parser := condition.NewParser(strategy.Condition)
		if _, err := parser.Parse(); err != nil {
			c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "Invalid condition expression: "+err.Error()))
			return
		}
	}

	// Server-side errors (database, service layer) should return 500
	if err := h.service.CreateStrategy(&strategy); err != nil {
		c.JSON(http.StatusInternalServerError, response.NewErrorResponse(response.StrategyCreateFailedCode, "Failed to create strategy: "+err.Error()))
		return
	}

	c.JSON(http.StatusCreated, response.NewSuccessResponse(strategy, "Strategy created successfully"))
}

// GetStrategies gets the strategy list
func (h *StrategyHandler) GetStrategies(c *gin.Context) {
	// Support filtering by status through query parameters
	status := c.Query("status")

	var strategies []models.QuotaStrategy
	var err error

	switch status {
	case "enabled", "true":
		strategies, err = h.service.GetEnabledStrategies()
	case "disabled", "false":
		strategies, err = h.service.GetDisabledStrategies()
	default:
		strategies, err = h.service.GetStrategies()
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, response.NewErrorResponse(response.DatabaseErrorCode, "Failed to retrieve strategies: "+err.Error()))
		return
	}

	data := gin.H{
		"strategies": strategies,
		"total":      len(strategies),
	}

	c.JSON(http.StatusOK, response.NewSuccessResponse(data, "Strategies retrieved successfully"))
}

// GetStrategy gets a single strategy
func (h *StrategyHandler) GetStrategy(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.InvalidStrategyIDCode, "Invalid strategy ID format"))
		return
	}

	strategy, err := h.service.GetStrategy(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response.NewErrorResponse(response.StrategyNotFoundCode, "Strategy not found: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.NewSuccessResponse(strategy, "Strategy retrieved successfully"))
}

// UpdateStrategy updates a strategy
func (h *StrategyHandler) UpdateStrategy(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.InvalidStrategyIDCode, "Invalid strategy ID format"))
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "Invalid request body: "+err.Error()))
		return
	}

	// Validate each field that is being updated
	if name, exists := updates["name"]; exists {
		if nameStr, ok := name.(string); ok {
			if err := validation.ValidateRequiredString(nameStr, "strategy name"); err != nil {
				c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, err.Error()))
				return
			}
			if err := validation.ValidateStringLength(nameStr, "strategy name", 1, 100); err != nil {
				c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, err.Error()))
				return
			}
		}
	}

	if title, exists := updates["title"]; exists {
		if titleStr, ok := title.(string); ok {
			if err := validation.ValidateRequiredString(titleStr, "strategy title"); err != nil {
				c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, err.Error()))
				return
			}
			if err := validation.ValidateStringLength(titleStr, "strategy title", 1, 200); err != nil {
				c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, err.Error()))
				return
			}
		}
	}

	if strategyType, exists := updates["type"]; exists {
		if typeStr, ok := strategyType.(string); ok {
			if !validation.IsValidStrategyType(typeStr) {
				c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "Strategy type must be 'single' or 'periodic'"))
				return
			}
		}
	}

	if amount, exists := updates["amount"]; exists {
		if !validation.IsPositiveInteger(amount) {
			c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "Strategy amount must be a positive integer"))
			return
		}
	}

	if periodicExpr, exists := updates["periodic_expr"]; exists {
		if exprStr, ok := periodicExpr.(string); ok {
			if err := validation.IsValidCronExpr(exprStr); err != nil {
				c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "Invalid periodic expression: "+err.Error()))
				return
			}
		}
	}

	if model, exists := updates["model"]; exists {
		if modelStr, ok := model.(string); ok && modelStr != "" {
			if err := validation.ValidateStringLength(modelStr, "model", 1, 100); err != nil {
				c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, err.Error()))
				return
			}
		}
	}

	if conditionValue, exists := updates["condition"]; exists {
		if conditionStr, ok := conditionValue.(string); ok && conditionStr != "" {
			parser := condition.NewParser(conditionStr)
			if _, err := parser.Parse(); err != nil {
				c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "Invalid condition expression: "+err.Error()))
				return
			}
		}
	}

	if err := h.service.UpdateStrategy(id, updates); err != nil {
		c.JSON(http.StatusInternalServerError, response.NewErrorResponse(response.StrategyUpdateFailedCode, "Failed to update strategy: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.NewSuccessResponse(nil, "Strategy updated successfully"))
}

// EnableStrategy enables a strategy
func (h *StrategyHandler) EnableStrategy(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.InvalidStrategyIDCode, "Invalid strategy ID format"))
		return
	}

	if err := h.service.EnableStrategy(id); err != nil {
		c.JSON(http.StatusInternalServerError, response.NewErrorResponse(response.StrategyUpdateFailedCode, "Failed to enable strategy: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.NewSuccessResponse(nil, "Strategy enabled successfully"))
}

// DisableStrategy disables a strategy
func (h *StrategyHandler) DisableStrategy(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.InvalidStrategyIDCode, "Invalid strategy ID format"))
		return
	}

	if err := h.service.DisableStrategy(id); err != nil {
		c.JSON(http.StatusInternalServerError, response.NewErrorResponse(response.StrategyUpdateFailedCode, "Failed to disable strategy: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.NewSuccessResponse(nil, "Strategy disabled successfully"))
}

// DeleteStrategy deletes a strategy
func (h *StrategyHandler) DeleteStrategy(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.InvalidStrategyIDCode, "Invalid strategy ID format"))
		return
	}

	if err := h.service.DeleteStrategy(id); err != nil {
		c.JSON(http.StatusInternalServerError, response.NewErrorResponse(response.StrategyDeleteFailedCode, "Failed to delete strategy: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.NewSuccessResponse(nil, "Strategy deleted successfully"))
}

// TriggerScan manually triggers strategy scan
func (h *StrategyHandler) TriggerScan(c *gin.Context) {
	go h.service.TraverseSingleStrategies()
	c.JSON(http.StatusOK, response.NewSuccessResponse(nil, "Strategy scan triggered successfully"))
}

// GetStrategyExecuteRecords gets execution records for a strategy
func (h *StrategyHandler) GetStrategyExecuteRecords(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.InvalidStrategyIDCode, "Invalid strategy ID format"))
		return
	}

	var req struct {
		Page     int `form:"page"`
		PageSize int `form:"page_size"`
	}

	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "Invalid query parameters: "+err.Error()))
		return
	}

	// Set default values
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	records, total, err := h.service.GetStrategyExecuteRecords(id, req.Page, req.PageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.NewErrorResponse(response.DatabaseErrorCode, "Failed to retrieve execution records: "+err.Error()))
		return
	}

	data := gin.H{
		"total":   total,
		"records": records,
	}

	c.JSON(http.StatusOK, response.NewSuccessResponse(data, "Strategy execution records retrieved successfully"))
}
