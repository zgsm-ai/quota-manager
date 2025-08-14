package handlers

import (
	"net/http"
	"quota-manager/internal/response"
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
	UserId string   `json:"user_id" validate:"required,uuid"`
	Models []string `json:"models" validate:"required,max=10"`
}

// SetDepartmentModelWhitelistRequest represents department model whitelist request
type SetDepartmentModelWhitelistRequest struct {
	DepartmentName string   `json:"department_name" validate:"required,department_name"`
	Models         []string `json:"models" validate:"required,max=10"`
}

// GetUserModelWhitelistQuery represents query parameters for getting user model whitelist
type GetUserModelWhitelistQuery struct {
	UserId string `form:"user_id" validate:"required,uuid"`
}

// GetDepartmentModelWhitelistQuery represents query parameters for getting department model whitelist
type GetDepartmentModelWhitelistQuery struct {
	DepartmentName string `form:"department_name" validate:"required,department_name"`
}

// SetUserWhitelist sets model whitelist for a user
func (h *ModelPermissionHandler) SetUserWhitelist(c *gin.Context) {
	var req SetUserModelWhitelistRequest

	if err := validation.ValidateJSON(c, &req); err != nil {
		return
	}

	if err := h.permissionService.SetUserWhitelist(req.UserId, req.Models); err != nil {
		if err.Error() == "whitelist already exists with same models" {
			c.JSON(http.StatusOK, gin.H{
				"code":    response.ModelPermissionWhitelistExistsCode,
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
					"code":    response.ModelPermissionUserNotFoundCode,
					"message": serviceErr.Message,
					"success": false,
				})
				return
			case services.ErrorDeptNotFound:
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    response.ModelPermissionDepartmentNotFoundCode,
					"message": serviceErr.Message,
					"success": false,
				})
				return
			case services.ErrorDatabaseError:
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":    response.ModelPermissionDatabaseErrorCode,
					"message": serviceErr.Message,
					"success": false,
				})
				return
			}
		}

		// Default case for other errors
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    response.ModelPermissionSetUserWhitelistFailedCode,
			"message": "Failed to set user model whitelist: " + err.Error(),
			"success": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    response.SuccessCode,
		"message": "User model whitelist set successfully",
		"success": true,
		"data": gin.H{
			"user_id": req.UserId,
			"models":  req.Models,
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
				"code":    response.ModelPermissionWhitelistExistsCode,
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
					"code":    response.ModelPermissionUserNotFoundCode,
					"message": serviceErr.Message,
					"success": false,
				})
				return
			case services.ErrorDeptNotFound:
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    response.ModelPermissionDepartmentNotFoundCode,
					"message": serviceErr.Message,
					"success": false,
				})
				return
			case services.ErrorDatabaseError:
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":    response.ModelPermissionDatabaseErrorCode,
					"message": serviceErr.Message,
					"success": false,
				})
				return
			}
		}

		// Default case for other errors
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    response.ModelPermissionSetDepartmentWhitelistFailedCode,
			"message": "Failed to set department model whitelist: " + err.Error(),
			"success": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    response.SuccessCode,
		"message": "Department model whitelist set successfully",
		"success": true,
		"data": gin.H{
			"department_name": req.DepartmentName,
			"models":          req.Models,
		},
	})
}

// GetUserWhitelist gets model whitelist for a user
func (h *ModelPermissionHandler) GetUserWhitelist(c *gin.Context) {
	var q GetUserModelWhitelistQuery
	if err := validation.ValidateQuery(c, &q); err != nil {
		return
	}

	modelsList, err := h.permissionService.GetUserWhitelist(q.UserId)
	if err != nil {
		if serviceErr, ok := err.(*services.ServiceError); ok {
			switch serviceErr.Code {
			case services.ErrorUserNotFound:
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    response.ModelPermissionUserNotFoundCode,
					"message": serviceErr.Message,
					"success": false,
				})
				return
			case services.ErrorDatabaseError:
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":    response.ModelPermissionDatabaseErrorCode,
					"message": serviceErr.Message,
					"success": false,
				})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    response.ModelPermissionGetUserWhitelistFailedCode,
			"message": "Failed to get user model whitelist: " + err.Error(),
			"success": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    response.SuccessCode,
		"message": "User model whitelist fetched successfully",
		"success": true,
		"data": gin.H{
			"user_id": q.UserId,
			"models":  modelsList,
		},
	})
}

// GetDepartmentWhitelist gets model whitelist for a department
func (h *ModelPermissionHandler) GetDepartmentWhitelist(c *gin.Context) {
	var q GetDepartmentModelWhitelistQuery
	if err := validation.ValidateQuery(c, &q); err != nil {
		return
	}

	modelsList, err := h.permissionService.GetDepartmentWhitelist(q.DepartmentName)
	if err != nil {
		if serviceErr, ok := err.(*services.ServiceError); ok {
			switch serviceErr.Code {
			case services.ErrorDeptNotFound:
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    response.ModelPermissionDepartmentNotFoundCode,
					"message": serviceErr.Message,
					"success": false,
				})
				return
			case services.ErrorDatabaseError:
				c.JSON(http.StatusInternalServerError, gin.H{
					"code":    response.ModelPermissionDatabaseErrorCode,
					"message": serviceErr.Message,
					"success": false,
				})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    response.ModelPermissionGetDepartmentWhitelistFailedCode,
			"message": "Failed to get department model whitelist: " + err.Error(),
			"success": false,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    response.SuccessCode,
		"message": "Department model whitelist fetched successfully",
		"success": true,
		"data": gin.H{
			"department_name": q.DepartmentName,
			"models":          modelsList,
		},
	})
}
