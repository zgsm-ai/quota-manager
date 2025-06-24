package handlers

import (
	"fmt"
	"net/http"
	"quota-manager/internal/config"
	"quota-manager/internal/models"
	"quota-manager/internal/services"

	"github.com/gin-gonic/gin"
)

// QuotaHandler handles quota-related HTTP requests
type QuotaHandler struct {
	quotaService *services.QuotaService
	serverConfig *config.ServerConfig
}

// NewQuotaHandler creates a new quota handler
func NewQuotaHandler(quotaService *services.QuotaService, serverConfig *config.ServerConfig) *QuotaHandler {
	return &QuotaHandler{
		quotaService: quotaService,
		serverConfig: serverConfig,
	}
}

// getUserFromToken extracts user info from token in request header
func (h *QuotaHandler) getUserFromToken(c *gin.Context) (*models.AuthUser, error) {
	tokenHeader := h.serverConfig.TokenHeader
	if tokenHeader == "" {
		tokenHeader = "authorization"
	}

	token := c.GetHeader(tokenHeader)
	if token == "" {
		return nil, fmt.Errorf("missing token in header: %s", tokenHeader)
	}

	return models.ParseUserInfoFromToken(token)
}

// getUserIDFromToken extracts user ID from token in request header
func (h *QuotaHandler) getUserIDFromToken(c *gin.Context) (string, error) {
	authUser, err := h.getUserFromToken(c)
	if err != nil {
		return "", err
	}
	return authUser.ID, nil
}

// GetUserQuota handles GET /quota-manager/api/v1/quota
func (h *QuotaHandler) GetUserQuota(c *gin.Context) {
	userID, err := h.getUserIDFromToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "failed to get user from token: " + err.Error(),
		})
		return
	}

	quotaInfo, err := h.quotaService.GetUserQuota(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get user quota: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, quotaInfo)
}

// GetQuotaAuditRecords handles GET /quota-manager/api/v1/quota/audit
func (h *QuotaHandler) GetQuotaAuditRecords(c *gin.Context) {
	userID, err := h.getUserIDFromToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "failed to get user from token: " + err.Error(),
		})
		return
	}

	var req struct {
		Page     int `form:"page"`
		PageSize int `form:"page_size"`
	}

	if err := c.ShouldBindQuery(&req); err != nil {
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

	records, total, err := h.quotaService.GetQuotaAuditRecords(userID, req.Page, req.PageSize)
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

// TransferOut handles POST /quota-manager/api/v1/quota/transfer-out
func (h *QuotaHandler) TransferOut(c *gin.Context) {
	giver, err := h.getUserFromToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "failed to get user from token: " + err.Error(),
		})
		return
	}

	var req services.TransferOutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request: " + err.Error(),
		})
		return
	}

	response, err := h.quotaService.TransferOut(giver, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to transfer out quota: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// TransferIn handles POST /quota-manager/api/v1/quota/transfer-in
func (h *QuotaHandler) TransferIn(c *gin.Context) {
	receiver, err := h.getUserFromToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "failed to get user from token: " + err.Error(),
		})
		return
	}

	var req services.TransferInRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid request: " + err.Error(),
		})
		return
	}

	response, err := h.quotaService.TransferIn(receiver, &req)
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
