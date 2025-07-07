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

// StrategySchemaHandler demonstrates the new schema-based validation approach
type StrategySchemaHandler struct {
	service *services.StrategyService
}

func NewStrategySchemaHandler(service *services.StrategyService) *StrategySchemaHandler {
	return &StrategySchemaHandler{service: service}
}

// CreateStrategySchema creates a new strategy using schema validation
func (h *StrategySchemaHandler) CreateStrategySchema(c *gin.Context) {
	var strategy models.QuotaStrategy

	// Use the new validation helper
	if err := validation.ValidateJSON(c, &strategy); err != nil {
		// Error response is already sent by ValidateJSON
		return
	}

	// Additional business logic validation for periodic strategies
	if strategy.Type == "periodic" && strategy.PeriodicExpr == "" {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "periodic expression is required for periodic strategies"))
		return
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

// UpdateStrategySchema updates a strategy using schema validation
func (h *StrategySchemaHandler) UpdateStrategySchema(c *gin.Context) {
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

	// For partial updates, we need to validate individual fields
	// Create a temporary strategy object for validation
	tempStrategy := models.QuotaStrategy{}

	// Apply updates to temp strategy for validation
	if name, exists := updates["name"]; exists {
		if nameStr, ok := name.(string); ok {
			tempStrategy.Name = nameStr
		}
	}
	if title, exists := updates["title"]; exists {
		if titleStr, ok := title.(string); ok {
			tempStrategy.Title = titleStr
		}
	}
	if strategyType, exists := updates["type"]; exists {
		if typeStr, ok := strategyType.(string); ok {
			tempStrategy.Type = typeStr
		}
	}
	if amount, exists := updates["amount"]; exists {
		if amountInt, ok := amount.(float64); ok {
			tempStrategy.Amount = int(amountInt)
		}
	}
	if model, exists := updates["model"]; exists {
		if modelStr, ok := model.(string); ok {
			tempStrategy.Model = modelStr
		}
	}
	if periodicExpr, exists := updates["periodic_expr"]; exists {
		if exprStr, ok := periodicExpr.(string); ok {
			tempStrategy.PeriodicExpr = exprStr
		}
	}
	if conditionValue, exists := updates["condition"]; exists {
		if conditionStr, ok := conditionValue.(string); ok {
			tempStrategy.Condition = conditionStr
		}
	}

	// Validate the temp strategy (this will validate only the fields that are being updated)
	// For production use, you might want to implement field-specific validation
	if err := validation.ValidateStruct(&tempStrategy); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, err.Error()))
		return
	}

	// Additional condition validation
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

// Example of how to add schema validation to quota handlers

// TransferOutSchema demonstrates schema validation for transfer out
func TransferOutSchemaExample(c *gin.Context, service *services.QuotaService) {
	giver, err := getUserFromToken(c) // assuming this helper exists
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.NewErrorResponse(response.TokenInvalidCode,
			"Failed to extract user from token: "+err.Error()))
		return
	}

	var req services.TransferOutRequest

	// Use schema validation
	if err := validation.ValidateJSON(c, &req); err != nil {
		// Error response is already sent by ValidateJSON
		return
	}

	// Business logic continues...
	resp, err := service.TransferOut(giver, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.NewErrorResponse(response.QuotaTransferFailedCode,
			"Failed to transfer out quota: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.NewSuccessResponse(resp, "Quota transferred out successfully"))
}

// Helper function placeholder
func getUserFromToken(c *gin.Context) (*models.AuthUser, error) {
	// This would be implemented to extract user from JWT token
	return nil, nil
}
