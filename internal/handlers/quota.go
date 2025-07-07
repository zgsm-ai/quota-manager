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

// GetQuotaAuditRecords handles GET /quota-manager/api/v1/quota/audit
func (h *QuotaHandler) GetQuotaAuditRecords(c *gin.Context) {
	userID, err := h.getUserIDFromToken(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, response.NewErrorResponse(response.TokenInvalidCode,
			"Failed to extract user from token: "+err.Error()))
		return
	}

	var req struct {
		Page     int `form:"page"`
		PageSize int `form:"page_size"`
	}

	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode,
			"Invalid query parameters: "+err.Error()))
		return
	}

	// Validate and set default values using validation package
	page, pageSize, err := validation.ValidatePageParams(req.Page, req.PageSize)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, err.Error()))
		return
	}
	req.Page = page
	req.PageSize = pageSize

	records, total, err := h.quotaService.GetQuotaAuditRecords(userID, req.Page, req.PageSize)
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

	var req services.TransferOutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode,
			"Invalid request body: "+err.Error()))
		return
	}

	// Clean receiver_id to remove leading/trailing whitespace before generating voucher
	cleanReceiverID := strings.TrimSpace(req.ReceiverID)

	// Validate receiver_id is not empty
	if err := validation.ValidateRequiredString(cleanReceiverID, "receiver_id"); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, err.Error()))
		return
	}

	// Validate receiver_id is valid UUID format
	if !validation.IsValidUUID(cleanReceiverID) {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode,
			"receiver_id must be a valid UUID format"))
		return
	}

	// Validate quota list is not empty
	if len(req.QuotaList) == 0 {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode,
			"Quota list cannot be empty"))
		return
	}

	// Validate each quota item in the list
	for i, quota := range req.QuotaList {
		if !validation.IsPositiveInteger(quota.Amount) {
			c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode,
				fmt.Sprintf("Quota item %d: amount must be a positive integer", i+1)))
			return
		}

		// Validate expiry date is not zero time
		if quota.ExpiryDate.IsZero() {
			c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode,
				fmt.Sprintf("Quota item %d: expiry_date is required", i+1)))
			return
		}
	}

	resp, err := h.quotaService.TransferOut(giver, &req)
	if err != nil {
		// Check if it's a business logic error (insufficient quota, etc.) - should be 400
		errMsg := err.Error()
		if strings.Contains(errMsg, "receiver_id cannot be empty") ||
			strings.Contains(errMsg, "insufficient") ||
			strings.Contains(errMsg, "quota not found") {
			c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.QuotaTransferFailedCode,
				"Transfer validation failed: "+err.Error()))
			return
		}

		// Otherwise it's a server-side error (database, AiGateway, etc.)
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

	// Validate voucher code is not empty
	if err := validation.ValidateRequiredString(req.VoucherCode, "voucher_code"); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, err.Error()))
		return
	}

	// Validate voucher code length (should be a reasonable JWT-like string)
	if err := validation.ValidateStringLength(req.VoucherCode, "voucher_code", 10, 2000); err != nil {
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
	userID := c.Param("user_id")

	// Validate user_id is not empty
	if err := validation.ValidateRequiredString(userID, "user_id"); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, err.Error()))
		return
	}

	// Validate user_id is valid UUID format
	if !validation.IsValidUUID(userID) {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode,
			"user_id must be a valid UUID format"))
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

	// Validate and set default values using validation package
	page, pageSize, err := validation.ValidatePageParams(req.Page, req.PageSize)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, err.Error()))
		return
	}
	req.Page = page
	req.PageSize = pageSize

	records, total, err := h.quotaService.GetUserQuotaAuditRecords(userID, req.Page, req.PageSize)
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
