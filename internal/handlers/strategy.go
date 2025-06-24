package handlers

import (
	"net/http"
	"strconv"
	"quota-manager/internal/models"
	"quota-manager/internal/response"
	"quota-manager/internal/services"

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

	// status is bool type, no additional validation needed, JSON parsing will handle it automatically

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

	// status is bool type, JSON parsing will handle it automatically, no additional validation needed

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
	go h.service.TraverseStrategy()
	c.JSON(http.StatusOK, response.NewSuccessResponse(nil, "Strategy scan triggered successfully"))
}