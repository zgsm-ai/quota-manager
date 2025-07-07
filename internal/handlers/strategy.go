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

	// Unified schema tag automatic validation
	if err := validation.ValidateStruct(&strategy); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, err.Error()))
		return
	}

	// For periodic type, periodic_expr is required and must be a valid cron expression.
	if strategy.Type == "periodic" {
		if strategy.PeriodicExpr == "" {
			c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "periodic_expr is required for periodic strategy"))
			return
		}
		if err := validation.IsValidCronExpr(strategy.PeriodicExpr); err != nil {
			c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "Invalid periodic expression: "+err.Error()))
			return
		}
	}

	// condition expression
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

	type UpdateStrategyRequest struct {
		Name         *string `json:"name" validate:"omitempty,min=1,max=100"`
		Title        *string `json:"title" validate:"omitempty,min=1,max=200"`
		Type         *string `json:"type" validate:"omitempty,oneof=single periodic"`
		Amount       *int    `json:"amount" validate:"omitempty"`
		PeriodicExpr *string `json:"periodic_expr" validate:"omitempty,cron"`
		Model        *string `json:"model" validate:"omitempty,min=1,max=100"`
		Condition    *string `json:"condition" validate:"omitempty"`
		Status       *bool   `json:"status"`
	}

	var req UpdateStrategyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "Invalid request body: "+err.Error()))
		return
	}
	if err := validation.ValidateStruct(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, err.Error()))
		return
	}

	// Special business logic: if type is periodic, periodic_expr must be valid cron
	if req.Type != nil && *req.Type == "periodic" && req.PeriodicExpr != nil {
		if err := validation.IsValidCronExpr(*req.PeriodicExpr); err != nil {
			c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "Invalid periodic expression: "+err.Error()))
			return
		}
	}

	// Special business logic: validate condition expression if present
	if req.Condition != nil && *req.Condition != "" {
		parser := condition.NewParser(*req.Condition)
		if _, err := parser.Parse(); err != nil {
			c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "Invalid condition expression: "+err.Error()))
			return
		}
	}

	// Prepare update map for service layer
	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Type != nil {
		updates["type"] = *req.Type
	}
	if req.Amount != nil {
		updates["amount"] = *req.Amount
	}
	if req.PeriodicExpr != nil {
		updates["periodic_expr"] = *req.PeriodicExpr
	}
	if req.Model != nil {
		updates["model"] = *req.Model
	}
	if req.Condition != nil {
		updates["condition"] = *req.Condition
	}
	if req.Status != nil {
		updates["status"] = *req.Status
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

	var req PaginationQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "Invalid query parameters: "+err.Error()))
		return
	}

	// Validate and normalize pagination parameters
	page, pageSize, err := validation.ValidatePageParams(req.Page, req.PageSize)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, err.Error()))
		return
	}

	records, total, err := h.service.GetStrategyExecuteRecords(id, page, pageSize)
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
