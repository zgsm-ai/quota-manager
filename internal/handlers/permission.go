package handlers

import (
	"net/http"
	modelsLib "quota-manager/internal/models"
	"quota-manager/internal/services"

	"quota-manager/internal/validation"

	"github.com/gin-gonic/gin"
)

// PermissionHandler handles permission-related HTTP requests
type PermissionHandler struct {
	permissionService   *services.PermissionService
	employeeSyncService *services.EmployeeSyncService
}

// NewPermissionHandler creates a new permission handler
func NewPermissionHandler(permissionService *services.PermissionService, employeeSyncService *services.EmployeeSyncService) *PermissionHandler {
	return &PermissionHandler{
		permissionService:   permissionService,
		employeeSyncService: employeeSyncService,
	}
}

// SetUserWhitelistRequest represents user whitelist request
type SetUserWhitelistRequest struct {
	EmployeeNumber string   `json:"employee_number" validate:"required,employee_number"`
	Models         []string `json:"models" validate:"required,max=10"`
}

// SetDepartmentWhitelistRequest represents department whitelist request
type SetDepartmentWhitelistRequest struct {
	DepartmentName string   `json:"department_name" validate:"required,department_name"`
	Models         []string `json:"models" validate:"required,max=10"`
}

// GetPermissionsRequest represents get permissions request
type GetPermissionsRequest struct {
	TargetType       string `form:"target_type" validate:"required,oneof=user department"`
	TargetIdentifier string `form:"target_identifier" validate:"required,min=2,max=100"`
}

// SetUserWhitelist sets whitelist for a user
func (h *PermissionHandler) SetUserWhitelist(c *gin.Context) {
	var req SetUserWhitelistRequest

	// Use the new validation helper instead of ShouldBindJSON
	if err := validation.ValidateJSON(c, &req); err != nil {
		// Error response is already sent by ValidateJSON
		return
	}

	if err := h.permissionService.SetUserWhitelist(req.EmployeeNumber, req.Models); err != nil {
		if err.Error() == "whitelist already exists with same models" {
			c.JSON(http.StatusOK, gin.H{
				"code":    "permission.whitelist_exists",
				"message": "Whitelist already exists, no update needed",
				"success": true,
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "permission.set_user_whitelist_failed",
			"message": "Failed to set user whitelist: " + err.Error(),
			"success": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    "permission.success",
		"message": "User whitelist set successfully",
		"success": true,
		"data": gin.H{
			"employee_number": req.EmployeeNumber,
			"models":          req.Models,
		},
	})
}

// SetDepartmentWhitelist sets whitelist for a department
func (h *PermissionHandler) SetDepartmentWhitelist(c *gin.Context) {
	var req SetDepartmentWhitelistRequest

	// Use the new validation helper instead of ShouldBindJSON
	if err := validation.ValidateJSON(c, &req); err != nil {
		// Error response is already sent by ValidateJSON
		return
	}

	if err := h.permissionService.SetDepartmentWhitelist(req.DepartmentName, req.Models); err != nil {
		if err.Error() == "whitelist already exists with same models" {
			c.JSON(http.StatusOK, gin.H{
				"code":    "permission.whitelist_exists",
				"message": "Whitelist already exists, no update needed",
				"success": true,
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "permission.set_department_whitelist_failed",
			"message": "Failed to set department whitelist: " + err.Error(),
			"success": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    "permission.success",
		"message": "Department whitelist set successfully",
		"success": true,
		"data": gin.H{
			"department_name": req.DepartmentName,
			"models":          req.Models,
		},
	})
}

// GetEffectivePermissions gets effective permissions for a user or department
func (h *PermissionHandler) GetEffectivePermissions(c *gin.Context) {
	var req GetPermissionsRequest

	// Use the new validation helper instead of ShouldBindQuery
	if err := validation.ValidateQuery(c, &req); err != nil {
		// Error response is already sent by ValidateQuery
		return
	}

	var modelsList []string
	var err error

	if req.TargetType == modelsLib.TargetTypeUser {
		modelsList, err = h.permissionService.GetUserEffectivePermissions(req.TargetIdentifier)
	} else {
		modelsList, err = h.permissionService.GetDepartmentEffectivePermissions(req.TargetIdentifier)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "permission.get_permissions_failed",
			"message": "Failed to get permissions: " + err.Error(),
			"success": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    "permission.success",
		"message": "Permissions retrieved successfully",
		"success": true,
		"data": gin.H{
			"target_type":       req.TargetType,
			"target_identifier": req.TargetIdentifier,
			"models":            modelsList,
		},
	})
}

// TriggerEmployeeSync triggers employee synchronization
func (h *PermissionHandler) TriggerEmployeeSync(c *gin.Context) {
	if err := h.employeeSyncService.SyncEmployees(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "permission.sync_failed",
			"message": "Failed to sync employees: " + err.Error(),
			"success": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    "permission.success",
		"message": "Employee sync triggered successfully",
		"success": true,
	})
}
