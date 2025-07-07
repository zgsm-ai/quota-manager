package handlers

import (
	"fmt"
	"net/http"
	"quota-manager/internal/config"
	"quota-manager/internal/models"
	"quota-manager/internal/response"
	"quota-manager/internal/services"
	"quota-manager/internal/validation"

	"strings"

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
		c.JSON(http.StatusUnauthorized, response.NewErrorResponse(response.TokenInvalidCode,
			"Failed to extract user from token: "+err.Error()))
		return
	}

	quotaInfo, err := h.quotaService.GetUserQuota(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.NewErrorResponse(response.InternalErrorCode,
			"Failed to retrieve user quota: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.NewSuccessResponse(quotaInfo, "User quota retrieved successfully"))
}

// PaginationQuery defines pagination parameters for query binding
type PaginationQuery struct {
	Page     int `form:"page"`
	PageSize int `form:"page_size"`
}

// UserIDUri is used for binding and validating user_id from URI
type UserIDUri struct {
	UserID string `uri:"user_id" binding:"required" validate:"required,uuid"`
}

// GetQuotaAuditRecords handles GET /quota-manager/api/v1/quota/audit
func (h *QuotaHandler) GetQuotaAuditRecords(c *gin.Context) {
	userID, err := h.getUserIDFromToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.NewErrorResponse(response.TokenInvalidCode,
			"Failed to extract user from token: "+err.Error()))
		return
	}

	var req PaginationQuery
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode,
			"Invalid query parameters: "+err.Error()))
		return
	}

	// Validate and normalize pagination parameters
	page, pageSize, err := validation.ValidatePageParams(req.Page, req.PageSize)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, err.Error()))
		return
	}

	records, total, err := h.quotaService.GetQuotaAuditRecords(userID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.NewErrorResponse(response.DatabaseErrorCode,
			"Failed to retrieve quota audit records: "+err.Error()))
		return
	}

	data := gin.H{
		"total":   total,
		"records": records,
	}

	c.JSON(http.StatusOK, response.NewSuccessResponse(data, "Quota audit records retrieved successfully"))
}

// TransferOut handles POST /quota-manager/api/v1/quota/transfer-out
func (h *QuotaHandler) TransferOut(c *gin.Context) {
	giver, err := h.getUserFromToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.NewErrorResponse(response.TokenInvalidCode,
			"Failed to extract user from token: "+err.Error()))
		return
	}

	// Create a temporary struct without validation tags for binding
	type TransferOutRequestRaw struct {
		ReceiverID string                       `json:"receiver_id"`
		QuotaList  []services.TransferQuotaItem `json:"quota_list"`
	}

	var rawReq TransferOutRequestRaw
	if err := c.ShouldBindJSON(&rawReq); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode,
			"Invalid request body: "+err.Error()))
		return
	}

	// Clean receiver_id to remove leading/trailing whitespace
	rawReq.ReceiverID = strings.TrimSpace(rawReq.ReceiverID)

	// Convert to the actual request struct with validation tags
	req := services.TransferOutRequest{
		ReceiverID: rawReq.ReceiverID,
		QuotaList:  rawReq.QuotaList,
	}

	if err := validation.ValidateStruct(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, err.Error()))
		return
	}

	resp, err := h.quotaService.TransferOut(giver, &req)
	if err != nil {
		// Business logic errors (insufficient quota, etc.) should return 400
		errMsg := err.Error()
		if strings.Contains(errMsg, "receiver_id cannot be empty") ||
			strings.Contains(errMsg, "insufficient") ||
			strings.Contains(errMsg, "quota not found") {
			c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.QuotaTransferFailedCode,
				"Transfer validation failed: "+err.Error()))
			return
		}
		// Otherwise it's a server-side error
		c.JSON(http.StatusInternalServerError, response.NewErrorResponse(response.QuotaTransferFailedCode,
			"Failed to transfer out quota: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, response.NewSuccessResponse(resp, "Quota transferred out successfully"))
}

// TransferIn handles POST /quota-manager/api/v1/quota/transfer-in
func (h *QuotaHandler) TransferIn(c *gin.Context) {
	receiver, err := h.getUserFromToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.NewErrorResponse(response.TokenInvalidCode,
			"Failed to extract user from token: "+err.Error()))
		return
	}

	var req services.TransferInRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode,
			"Invalid request body: "+err.Error()))
		return
	}
	if err := validation.ValidateStruct(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, err.Error()))
		return
	}

	resp, err := h.quotaService.TransferIn(receiver, &req)
	if err != nil {
		// For TransferIn, service layer returns TransferInResponse even for business logic errors
		// Only database/system errors return actual errors
		c.JSON(http.StatusInternalServerError, response.NewErrorResponse(response.QuotaTransferFailedCode,
			"Failed to transfer in quota: "+err.Error()))
		return
	}

	// Check if the transfer had business logic issues (voucher validation, etc.)
	if resp.Status == services.TransferStatusFailed {
		// These are business logic failures, should return 400
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.QuotaTransferFailedCode,
			resp.Message))
		return
	}

	c.JSON(http.StatusOK, response.NewSuccessResponse(resp, "Quota transferred in successfully"))
}

// GetUserQuotaAuditRecordsAdminEmptyID handles the case when user_id is empty
func (h *QuotaHandler) GetUserQuotaAuditRecordsAdminEmptyID(c *gin.Context) {
	c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode,
		"user_id is required and cannot be empty"))
}

// GetUserQuotaAuditRecordsAdmin gets quota audit records for a specific user (admin function)
func (h *QuotaHandler) GetUserQuotaAuditRecordsAdmin(c *gin.Context) {
	// Bind user_id from URI
	var uriReq UserIDUri
	if err := c.ShouldBindUri(&uriReq); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "Invalid user_id: "+err.Error()))
		return
	}

	// Validate user_id
	if err := validation.ValidateStruct(&uriReq); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, err.Error()))
		return
	}

	// Bind pagination parameters from query
	var queryReq PaginationQuery
	if err := c.ShouldBindQuery(&queryReq); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "Invalid query parameters: "+err.Error()))
		return
	}

	// Validate and normalize pagination parameters
	page, pageSize, err := validation.ValidatePageParams(queryReq.Page, queryReq.PageSize)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, err.Error()))
		return
	}

	records, total, err := h.quotaService.GetUserQuotaAuditRecords(uriReq.UserID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.NewErrorResponse(response.DatabaseErrorCode, "Failed to retrieve quota audit records: "+err.Error()))
		return
	}

	data := gin.H{
		"total":   total,
		"records": records,
	}

	c.JSON(http.StatusOK, response.NewSuccessResponse(data, "User quota audit records retrieved successfully"))
}

// RegisterQuotaRoutes registers quota-related routes
func RegisterQuotaRoutes(r *gin.RouterGroup, quotaHandler *QuotaHandler) {
	quota := r.Group("/quota")
	{
		quota.GET("", quotaHandler.GetUserQuota)
		quota.GET("/audit", quotaHandler.GetQuotaAuditRecords)
		quota.POST("/transfer-out", quotaHandler.TransferOut)
		quota.POST("/transfer-in", quotaHandler.TransferIn)
		// Handle empty user_id case (must be before parameterized route)
		quota.GET("/audit/", quotaHandler.GetUserQuotaAuditRecordsAdminEmptyID)
		quota.GET("/audit/:user_id", quotaHandler.GetUserQuotaAuditRecordsAdmin)
	}
}
