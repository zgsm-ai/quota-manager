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

// CreateStrategy 创建策略
func (h *StrategyHandler) CreateStrategy(c *gin.Context) {
	var strategy models.QuotaStrategy
	if err := c.ShouldBindJSON(&strategy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.CreateStrategy(&strategy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, strategy)
}

// GetStrategies 获取策略列表
func (h *StrategyHandler) GetStrategies(c *gin.Context) {
	strategies, err := h.service.GetStrategies()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, strategies)
}

// GetStrategy 获取单个策略
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

// UpdateStrategy 更新策略
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

	if err := h.service.UpdateStrategy(id, updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "strategy updated successfully"})
}

// DeleteStrategy 删除策略
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

// TriggerScan 手动触发策略扫描
func (h *StrategyHandler) TriggerScan(c *gin.Context) {
	go h.service.TraverseStrategy()
	c.JSON(http.StatusOK, gin.H{"message": "strategy scan triggered"})
}