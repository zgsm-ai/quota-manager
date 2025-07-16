package handlers

import (
	"net/http"
	"quota-manager/internal/services"
	"quota-manager/internal/validation"

	"github.com/gin-gonic/gin"
)

// StarCheckPermissionHandler handles star check permission-related HTTP requests
type StarCheckPermissionHandler struct {
	starCheckPermissionService *services.StarCheckPermissionService
}

// NewStarCheckPermissionHandler creates a new star check permission handler
func NewStarCheckPermissionHandler(starCheckPermissionService *services.StarCheckPermissionService) *StarCheckPermissionHandler {
	return &StarCheckPermissionHandler{
		starCheckPermissionService: starCheckPermissionService,
	}
}

// SetUserStarCheckRequest represents user star check request
type SetUserStarCheckRequest struct {
	EmployeeNumber string `json:"employee_number" validate:"required,employee_number"`
	Enabled        *bool  `json:"enabled" validate:"required"`
}

// SetDepartmentStarCheckRequest represents department star check request
type SetDepartmentStarCheckRequest struct {
	DepartmentName string `json:"department_name" validate:"required,department_name"`
	Enabled        *bool  `json:"enabled" validate:"required"`
}

// SetUserStarCheckSetting sets star check setting for a user
func (h *StarCheckPermissionHandler) SetUserStarCheckSetting(c *gin.Context) {
	var req SetUserStarCheckRequest

	if err := validation.ValidateJSON(c, &req); err != nil {
		return
	}

	if err := h.starCheckPermissionService.SetUserStarCheckSetting(req.EmployeeNumber, *req.Enabled); err != nil {
		if err.Error() == "star check setting already exists with same value" {
			c.JSON(http.StatusOK, gin.H{
				"code":    "star_check_permission.setting_exists",
				"message": "Star check setting already exists, no update needed",
				"success": true,
			})
			return
		}

		// Check if it's a ServiceError
		if serviceErr, ok := err.(*services.ServiceError); ok {
			switch serviceErr.Code {
			case services.ErrorUserNotFound:
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    "star_check_permission.user_not_found",
					"message": serviceErr.Message,
					"success": false,
				})
				return
			case services.ErrorDeptNotFound:
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    "star_check_permission.department_not_found",
					"message": serviceErr.Message,
					"success": false,
				})
				return
			case services.ErrorDatabaseError:
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":    "star_check_permission.database_error",
					"message": serviceErr.Message,
					"success": false,
				})
				return
			}
		}

		// Default case for other errors
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "star_check_permission.set_user_setting_failed",
			"message": "Failed to set user star check setting: " + err.Error(),
			"success": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    "star_check_permission.success",
		"message": "User star check setting set successfully",
		"success": true,
		"data": gin.H{
			"employee_number": req.EmployeeNumber,
			"enabled":         *req.Enabled,
		},
	})
}

// SetDepartmentStarCheckSetting sets star check setting for a department
func (h *StarCheckPermissionHandler) SetDepartmentStarCheckSetting(c *gin.Context) {
	var req SetDepartmentStarCheckRequest

	if err := validation.ValidateJSON(c, &req); err != nil {
		return
	}

	if err := h.starCheckPermissionService.SetDepartmentStarCheckSetting(req.DepartmentName, *req.Enabled); err != nil {
		if err.Error() == "star check setting already exists with same value" {
			c.JSON(http.StatusOK, gin.H{
				"code":    "star_check_permission.setting_exists",
				"message": "Star check setting already exists, no update needed",
				"success": true,
			})
			return
		}

		// Check if it's a ServiceError
		if serviceErr, ok := err.(*services.ServiceError); ok {
			switch serviceErr.Code {
			case services.ErrorUserNotFound:
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    "star_check_permission.user_not_found",
					"message": serviceErr.Message,
					"success": false,
				})
				return
			case services.ErrorDeptNotFound:
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    "star_check_permission.department_not_found",
					"message": serviceErr.Message,
					"success": false,
				})
				return
			case services.ErrorDatabaseError:
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":    "star_check_permission.database_error",
					"message": serviceErr.Message,
					"success": false,
				})
				return
			}
		}

		// Default case for other errors
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "star_check_permission.set_department_setting_failed",
			"message": "Failed to set department star check setting: " + err.Error(),
			"success": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    "star_check_permission.success",
		"message": "Department star check setting set successfully",
		"success": true,
		"data": gin.H{
			"department_name": req.DepartmentName,
			"enabled":         *req.Enabled,
		},
	})
}
