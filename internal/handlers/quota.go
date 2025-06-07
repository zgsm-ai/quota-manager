package handlers

import (
	"net/http"
	"quota-manager/internal/services"

	"github.com/gin-gonic/gin"
)

// QuotaHandler handles quota-related HTTP requests
type QuotaHandler struct {
	quotaService *services.QuotaService
}

// NewQuotaHandler creates a new quota handler
func NewQuotaHandler(quotaService *services.QuotaService) *QuotaHandler {
	return &QuotaHandler{
		quotaService: quotaService,
	}
}

// GetUserQuota handles GET /api/v1/quota
func (h *QuotaHandler) GetUserQuota(c *gin.Context) {
	var req struct {
		UserID string `json:"user_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request: " + err.Error(),
		})
		return
	}

	quotaInfo, err := h.quotaService.GetUserQuota(req.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get user quota: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, quotaInfo)
}

// GetQuotaAuditRecords handles GET /api/v1/quota/audit
func (h *QuotaHandler) GetQuotaAuditRecords(c *gin.Context) {
	var req struct {
		UserID   string `json:"user_id" binding:"required"`
		Page     int    `json:"page"`
		PageSize int    `json:"page_size"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request: " + err.Error(),
		})
		return
	}

	// Set default values
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}

	records, total, err := h.quotaService.GetQuotaAuditRecords(req.UserID, req.Page, req.PageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get quota audit records: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"total":   total,
		"records": records,
	})
}

// TransferOut handles POST /api/v1/quota/transfer-out
func (h *QuotaHandler) TransferOut(c *gin.Context) {
	var req services.TransferOutRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request: " + err.Error(),
		})
		return
	}

	response, err := h.quotaService.TransferOut(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to transfer out quota: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// TransferIn handles POST /api/v1/quota/transfer-in
func (h *QuotaHandler) TransferIn(c *gin.Context) {
	var req services.TransferInRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request: " + err.Error(),
		})
		return
	}

	response, err := h.quotaService.TransferIn(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to transfer in quota: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// RegisterQuotaRoutes registers quota-related routes
func RegisterQuotaRoutes(r *gin.RouterGroup, quotaHandler *QuotaHandler) {
	quota := r.Group("/quota")
	{
		quota.GET("", quotaHandler.GetUserQuota)
		quota.GET("/audit", quotaHandler.GetQuotaAuditRecords)
		quota.POST("/transfer-out", quotaHandler.TransferOut)
		quota.POST("/transfer-in", quotaHandler.TransferIn)
	}
}
