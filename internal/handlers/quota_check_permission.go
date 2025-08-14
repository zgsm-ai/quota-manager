package handlers

import (
	"net/http"
	"quota-manager/internal/response"
	"quota-manager/internal/services"
	"quota-manager/internal/validation"

	"github.com/gin-gonic/gin"
)

// QuotaCheckPermissionHandler handles quota check permission-related HTTP requests
type QuotaCheckPermissionHandler struct {
	quotaCheckPermissionService *services.QuotaCheckPermissionService
}

// NewQuotaCheckPermissionHandler creates a new quota check permission handler
func NewQuotaCheckPermissionHandler(quotaCheckPermissionService *services.QuotaCheckPermissionService) *QuotaCheckPermissionHandler {
	return &QuotaCheckPermissionHandler{
		quotaCheckPermissionService: quotaCheckPermissionService,
	}
}

// SetUserQuotaCheckRequest represents user quota check request
type SetUserQuotaCheckRequest struct {
	UserId  string `json:"user_id" validate:"required,uuid"`
	Enabled *bool  `json:"enabled" validate:"required"`
}

// SetDepartmentQuotaCheckRequest represents department quota check request
type SetDepartmentQuotaCheckRequest struct {
	DepartmentName string `json:"department_name" validate:"required,department_name"`
	Enabled        *bool  `json:"enabled" validate:"required"`
}

// GetUserQuotaCheckQuery represents query parameters for getting user quota check setting
type GetUserQuotaCheckQuery struct {
	UserId string `form:"user_id" validate:"required,uuid"`
}

// GetDepartmentQuotaCheckQuery represents query parameters for getting department quota check setting
type GetDepartmentQuotaCheckQuery struct {
	DepartmentName string `form:"department_name" validate:"required,department_name"`
}

// SetUserQuotaCheckSetting sets quota check setting for a user
func (h *QuotaCheckPermissionHandler) SetUserQuotaCheckSetting(c *gin.Context) {
	var req SetUserQuotaCheckRequest

	if err := validation.ValidateJSON(c, &req); err != nil {
		return
	}

	if err := h.quotaCheckPermissionService.SetUserQuotaCheckSetting(req.UserId, *req.Enabled); err != nil {
		// Check if it's a ServiceError
		if serviceErr, ok := err.(*services.ServiceError); ok {
			switch serviceErr.Code {
			case services.ErrorUserNotFound:
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    response.QuotaCheckPermissionUserNotFoundCode,
					"message": serviceErr.Message,
					"success": false,
				})
				return
			case services.ErrorDeptNotFound:
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    response.QuotaCheckPermissionDepartmentNotFoundCode,
					"message": serviceErr.Message,
					"success": false,
				})
				return
			case services.ErrorDatabaseError:
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":    response.QuotaCheckPermissionDatabaseErrorCode,
					"message": serviceErr.Message,
					"success": false,
				})
				return
			}
		}

		// Default case for other errors
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    response.QuotaCheckPermissionSetUserSettingFailedCode,
			"message": "Failed to set user quota check setting: " + err.Error(),
			"success": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    response.SuccessCode,
		"message": "User quota check setting set successfully",
		"success": true,
		"data": gin.H{
			"user_id": req.UserId,
			"enabled": *req.Enabled,
		},
	})
}

// SetDepartmentQuotaCheckSetting sets quota check setting for a department
func (h *QuotaCheckPermissionHandler) SetDepartmentQuotaCheckSetting(c *gin.Context) {
	var req SetDepartmentQuotaCheckRequest

	if err := validation.ValidateJSON(c, &req); err != nil {
		return
	}

	if err := h.quotaCheckPermissionService.SetDepartmentQuotaCheckSetting(req.DepartmentName, *req.Enabled); err != nil {
		// Check if it's a ServiceError
		if serviceErr, ok := err.(*services.ServiceError); ok {
			switch serviceErr.Code {
			case services.ErrorUserNotFound:
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    response.QuotaCheckPermissionUserNotFoundCode,
					"message": serviceErr.Message,
					"success": false,
				})
				return
			case services.ErrorDeptNotFound:
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    response.QuotaCheckPermissionDepartmentNotFoundCode,
					"message": serviceErr.Message,
					"success": false,
				})
				return
			case services.ErrorDatabaseError:
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":    response.QuotaCheckPermissionDatabaseErrorCode,
					"message": serviceErr.Message,
					"success": false,
				})
				return
			}
		}

		// Default case for other errors
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    response.QuotaCheckPermissionSetDepartmentSettingFailedCode,
			"message": "Failed to set department quota check setting: " + err.Error(),
			"success": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    response.SuccessCode,
		"message": "Department quota check setting set successfully",
		"success": true,
		"data": gin.H{
			"department_name": req.DepartmentName,
			"enabled":         *req.Enabled,
		},
	})
}

// GetUserQuotaCheckSetting gets quota check setting for a user
func (h *QuotaCheckPermissionHandler) GetUserQuotaCheckSetting(c *gin.Context) {
	var q GetUserQuotaCheckQuery
	if err := validation.ValidateQuery(c, &q); err != nil {
		return
	}

	enabled, err := h.quotaCheckPermissionService.GetUserQuotaCheckSetting(q.UserId)
	if err != nil {
		if serviceErr, ok := err.(*services.ServiceError); ok {
			switch serviceErr.Code {
			case services.ErrorUserNotFound:
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    response.QuotaCheckPermissionUserNotFoundCode,
					"message": serviceErr.Message,
					"success": false,
				})
				return
			case services.ErrorDatabaseError:
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":    response.QuotaCheckPermissionDatabaseErrorCode,
					"message": serviceErr.Message,
					"success": false,
				})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    response.QuotaCheckPermissionGetUserSettingFailedCode,
			"message": "Failed to get user quota check setting: " + err.Error(),
			"success": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    response.SuccessCode,
		"message": "User quota check setting fetched successfully",
		"success": true,
		"data": gin.H{
			"user_id": q.UserId,
			"enabled": enabled,
		},
	})
}

// GetDepartmentQuotaCheckSetting gets quota check setting for a department
func (h *QuotaCheckPermissionHandler) GetDepartmentQuotaCheckSetting(c *gin.Context) {
	var q GetDepartmentQuotaCheckQuery
	if err := validation.ValidateQuery(c, &q); err != nil {
		return
	}

	enabled, err := h.quotaCheckPermissionService.GetDepartmentQuotaCheckSetting(q.DepartmentName)
	if err != nil {
		if serviceErr, ok := err.(*services.ServiceError); ok {
			switch serviceErr.Code {
			case services.ErrorDeptNotFound:
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    response.QuotaCheckPermissionDepartmentNotFoundCode,
					"message": serviceErr.Message,
					"success": false,
				})
				return
			case services.ErrorDatabaseError:
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":    response.QuotaCheckPermissionDatabaseErrorCode,
					"message": serviceErr.Message,
					"success": false,
				})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    response.QuotaCheckPermissionGetDepartmentSettingFailedCode,
			"message": "Failed to get department quota check setting: " + err.Error(),
			"success": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    response.SuccessCode,
		"message": "Department quota check setting fetched successfully",
		"success": true,
		"data": gin.H{
			"department_name": q.DepartmentName,
			"enabled":         enabled,
		},
	})
}
