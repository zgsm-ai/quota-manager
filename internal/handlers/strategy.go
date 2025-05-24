package handlers

import (
	"net/http"
	"strconv"
	"quota-manager/internal/models"
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// status is bool type, no additional validation needed, JSON parsing will handle it automatically

	if err := h.service.CreateStrategy(&strategy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, strategy)
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"strategies": strategies,
		"total":      len(strategies),
	})
}

// GetStrategy gets a single strategy
func (h *StrategyHandler) GetStrategy(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid strategy id"})
		return
	}

	strategy, err := h.service.GetStrategy(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, strategy)
}

// UpdateStrategy updates a strategy
func (h *StrategyHandler) UpdateStrategy(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid strategy id"})
		return
	}

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// status is bool type, JSON parsing will handle it automatically, no additional validation needed

	if err := h.service.UpdateStrategy(id, updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "strategy updated successfully"})
}

// EnableStrategy enables a strategy
func (h *StrategyHandler) EnableStrategy(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid strategy id"})
		return
	}

	if err := h.service.EnableStrategy(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "strategy enabled successfully"})
}

// DisableStrategy disables a strategy
func (h *StrategyHandler) DisableStrategy(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid strategy id"})
		return
	}

	if err := h.service.DisableStrategy(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "strategy disabled successfully"})
}

// DeleteStrategy deletes a strategy
func (h *StrategyHandler) DeleteStrategy(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid strategy id"})
		return
	}

	if err := h.service.DeleteStrategy(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "strategy deleted successfully"})
}

// TriggerScan manually triggers strategy scan
func (h *StrategyHandler) TriggerScan(c *gin.Context) {
	go h.service.TraverseStrategy()
	c.JSON(http.StatusOK, gin.H{"message": "strategy scan triggered"})
}