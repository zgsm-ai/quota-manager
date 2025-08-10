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
	UserId  string `json:"user_id" validate:"required,uuid"`
	Enabled *bool  `json:"enabled" validate:"required"`
}

// SetDepartmentStarCheckRequest represents department star check request
type SetDepartmentStarCheckRequest struct {
	DepartmentName string `json:"department_name" validate:"required,department_name"`
	Enabled        *bool  `json:"enabled" validate:"required"`
}

// GetUserStarCheckQuery represents query parameters for getting user star check setting
type GetUserStarCheckQuery struct {
	UserId string `form:"user_id" validate:"required,uuid"`
}

// GetDepartmentStarCheckQuery represents query parameters for getting department star check setting
type GetDepartmentStarCheckQuery struct {
	DepartmentName string `form:"department_name" validate:"required,department_name"`
}

// SetUserStarCheckSetting sets star check setting for a user
func (h *StarCheckPermissionHandler) SetUserStarCheckSetting(c *gin.Context) {
	var req SetUserStarCheckRequest

	if err := validation.ValidateJSON(c, &req); err != nil {
		return
	}

	if err := h.starCheckPermissionService.SetUserStarCheckSetting(req.UserId, *req.Enabled); err != nil {
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
			"user_id": req.UserId,
			"enabled": *req.Enabled,
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

// GetUserStarCheckSetting gets star check setting for a user
func (h *StarCheckPermissionHandler) GetUserStarCheckSetting(c *gin.Context) {
	var q GetUserStarCheckQuery
	if err := validation.ValidateQuery(c, &q); err != nil {
		return
	}

	enabled, err := h.starCheckPermissionService.GetUserStarCheckSetting(q.UserId)
	if err != nil {
		if serviceErr, ok := err.(*services.ServiceError); ok {
			switch serviceErr.Code {
			case services.ErrorUserNotFound:
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    "star_check_permission.user_not_found",
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "star_check_permission.get_user_setting_failed",
			"message": "Failed to get user star check setting: " + err.Error(),
			"success": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    "star_check_permission.success",
		"message": "User star check setting fetched successfully",
		"success": true,
		"data": gin.H{
			"user_id": q.UserId,
			"enabled": enabled,
		},
	})
}

// GetDepartmentStarCheckSetting gets star check setting for a department
func (h *StarCheckPermissionHandler) GetDepartmentStarCheckSetting(c *gin.Context) {
	var q GetDepartmentStarCheckQuery
	if err := validation.ValidateQuery(c, &q); err != nil {
		return
	}

	enabled, err := h.starCheckPermissionService.GetDepartmentStarCheckSetting(q.DepartmentName)
	if err != nil {
		if serviceErr, ok := err.(*services.ServiceError); ok {
			switch serviceErr.Code {
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "star_check_permission.get_department_setting_failed",
			"message": "Failed to get department star check setting: " + err.Error(),
			"success": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    "star_check_permission.success",
		"message": "Department star check setting fetched successfully",
		"success": true,
		"data": gin.H{
			"department_name": q.DepartmentName,
			"enabled":         enabled,
		},
	})
}
