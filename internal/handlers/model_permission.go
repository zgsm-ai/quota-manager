package handlers

import (
	"net/http"
	"quota-manager/internal/services"
	"quota-manager/internal/validation"

	"github.com/gin-gonic/gin"
)

// ModelPermissionHandler handles model permission-related HTTP requests
type ModelPermissionHandler struct {
	permissionService *services.PermissionService
}

// NewModelPermissionHandler creates a new model permission handler
func NewModelPermissionHandler(permissionService *services.PermissionService) *ModelPermissionHandler {
	return &ModelPermissionHandler{
		permissionService: permissionService,
	}
}

// SetUserModelWhitelistRequest represents user model whitelist request
type SetUserModelWhitelistRequest struct {
	EmployeeNumber string   `json:"employee_number" validate:"required,employee_number"`
	Models         []string `json:"models" validate:"required,max=10"`
}

// SetDepartmentModelWhitelistRequest represents department model whitelist request
type SetDepartmentModelWhitelistRequest struct {
	DepartmentName string   `json:"department_name" validate:"required,department_name"`
	Models         []string `json:"models" validate:"required,max=10"`
}

// SetUserWhitelist sets model whitelist for a user
func (h *ModelPermissionHandler) SetUserWhitelist(c *gin.Context) {
	var req SetUserModelWhitelistRequest

	if err := validation.ValidateJSON(c, &req); err != nil {
		return
	}

	if err := h.permissionService.SetUserWhitelist(req.EmployeeNumber, req.Models); err != nil {
		if err.Error() == "whitelist already exists with same models" {
			c.JSON(http.StatusOK, gin.H{
				"code":    "model_permission.whitelist_exists",
				"message": "Model whitelist already exists, no update needed",
				"success": true,
			})
			return
		}

		// Check if it's a ServiceError
		if serviceErr, ok := err.(*services.ServiceError); ok {
			switch serviceErr.Code {
			case services.ErrorUserNotFound:
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    "model_permission.user_not_found",
					"message": serviceErr.Message,
					"success": false,
				})
				return
			case services.ErrorDeptNotFound:
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    "model_permission.department_not_found",
					"message": serviceErr.Message,
					"success": false,
				})
				return
			case services.ErrorDatabaseError:
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":    "model_permission.database_error",
					"message": serviceErr.Message,
					"success": false,
				})
				return
			}
		}

		// Default case for other errors
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "model_permission.set_user_whitelist_failed",
			"message": "Failed to set user model whitelist: " + err.Error(),
			"success": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    "model_permission.success",
		"message": "User model whitelist set successfully",
		"success": true,
		"data": gin.H{
			"employee_number": req.EmployeeNumber,
			"models":          req.Models,
		},
	})
}

// SetDepartmentWhitelist sets model whitelist for a department
func (h *ModelPermissionHandler) SetDepartmentWhitelist(c *gin.Context) {
	var req SetDepartmentModelWhitelistRequest

	if err := validation.ValidateJSON(c, &req); err != nil {
		return
	}

	if err := h.permissionService.SetDepartmentWhitelist(req.DepartmentName, req.Models); err != nil {
		if err.Error() == "whitelist already exists with same models" {
			c.JSON(http.StatusOK, gin.H{
				"code":    "model_permission.whitelist_exists",
				"message": "Model whitelist already exists, no update needed",
				"success": true,
			})
			return
		}

		// Check if it's a ServiceError
		if serviceErr, ok := err.(*services.ServiceError); ok {
			switch serviceErr.Code {
			case services.ErrorUserNotFound:
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    "model_permission.user_not_found",
					"message": serviceErr.Message,
					"success": false,
				})
				return
			case services.ErrorDeptNotFound:
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    "model_permission.department_not_found",
					"message": serviceErr.Message,
					"success": false,
				})
				return
			case services.ErrorDatabaseError:
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":    "model_permission.database_error",
					"message": serviceErr.Message,
					"success": false,
				})
				return
			}
		}

		// Default case for other errors
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "model_permission.set_department_whitelist_failed",
			"message": "Failed to set department model whitelist: " + err.Error(),
			"success": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    "model_permission.success",
		"message": "Department model whitelist set successfully",
		"success": true,
		"data": gin.H{
			"department_name": req.DepartmentName,
			"models":          req.Models,
		},
	})
}
