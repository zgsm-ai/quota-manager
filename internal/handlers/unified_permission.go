package handlers

import (
	"net/http"
	"quota-manager/internal/response"
	"quota-manager/internal/services"
	"quota-manager/internal/validation"

	"github.com/gin-gonic/gin"
)

// UnifiedPermissionHandler handles unified permission queries and sync
type UnifiedPermissionHandler struct {
	unifiedPermissionService *services.UnifiedPermissionService
}

// NewUnifiedPermissionHandler creates a new unified permission handler
func NewUnifiedPermissionHandler(unifiedPermissionService *services.UnifiedPermissionService) *UnifiedPermissionHandler {
	return &UnifiedPermissionHandler{
		unifiedPermissionService: unifiedPermissionService,
	}
}

// GetEffectivePermissionsRequest represents unified permission query request
type GetEffectivePermissionsRequest struct {
	Type             string `form:"type" validate:"required,oneof=model star-check quota-check"`
	TargetType       string `form:"target_type" validate:"required,oneof=user department"`
	TargetIdentifier string `form:"target_identifier" validate:"required,min=2,max=100"`
}

// GetEffectivePermissions gets effective permissions (unified endpoint)
func (h *UnifiedPermissionHandler) GetEffectivePermissions(c *gin.Context) {
	var req GetEffectivePermissionsRequest

	if err := validation.ValidateQuery(c, &req); err != nil {
		return
	}

	switch req.Type {
	case "model":
		h.handleModelPermissions(c, req)
	case "star-check":
		h.handleStarCheckPermissions(c, req)
	case "quota-check":
		h.handleQuotaCheckPermissions(c, req)
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    response.UnifiedPermissionInvalidTypeCode,
			"message": "Invalid permission type",
			"success": false,
		})
	}
}

// handleModelPermissions handles model permission queries
func (h *UnifiedPermissionHandler) handleModelPermissions(c *gin.Context, req GetEffectivePermissionsRequest) {
	modelsList, err := h.unifiedPermissionService.GetModelEffectivePermissions(req.TargetType, req.TargetIdentifier)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    response.ModelPermissionGetPermissionsFailedCode,
			"message": "Failed to get model permissions: " + err.Error(),
			"success": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    response.SuccessCode,
		"message": "Model permissions retrieved successfully",
		"success": true,
		"data": gin.H{
			"type":              "model",
			"target_type":       req.TargetType,
			"target_identifier": req.TargetIdentifier,
			"models":            modelsList,
		},
	})
}

// handleStarCheckPermissions handles star check permission queries
func (h *UnifiedPermissionHandler) handleStarCheckPermissions(c *gin.Context, req GetEffectivePermissionsRequest) {
	enabled, err := h.unifiedPermissionService.GetStarCheckEffectivePermissions(req.TargetType, req.TargetIdentifier)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    response.StarCheckPermissionGetPermissionsFailedCode,
			"message": "Failed to get star check permissions: " + err.Error(),
			"success": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    response.SuccessCode,
		"message": "Star check permissions retrieved successfully",
		"success": true,
		"data": gin.H{
			"type":              "star-check",
			"target_type":       req.TargetType,
			"target_identifier": req.TargetIdentifier,
			"enabled":           enabled,
		},
	})
}

// handleQuotaCheckPermissions handles quota check permission queries
func (h *UnifiedPermissionHandler) handleQuotaCheckPermissions(c *gin.Context, req GetEffectivePermissionsRequest) {
	enabled, err := h.unifiedPermissionService.GetQuotaCheckEffectivePermissions(req.TargetType, req.TargetIdentifier)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    response.QuotaCheckPermissionGetPermissionsFailedCode,
			"message": "Failed to get quota check permissions: " + err.Error(),
			"success": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    response.SuccessCode,
		"message": "Quota check permissions retrieved successfully",
		"success": true,
		"data": gin.H{
			"type":              "quota-check",
			"target_type":       req.TargetType,
			"target_identifier": req.TargetIdentifier,
			"enabled":           enabled,
		},
	})
}

// TriggerEmployeeSync triggers employee synchronization
func (h *UnifiedPermissionHandler) TriggerEmployeeSync(c *gin.Context) {
	if err := h.unifiedPermissionService.TriggerEmployeeSync(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    response.EmployeeSyncFailedCode,
			"message": "Failed to sync employees: " + err.Error(),
			"success": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    response.SuccessCode,
		"message": "Employee sync triggered successfully",
		"success": true,
	})
}
