package handlers

import (
	"net/http"
	"quota-manager/internal/response"
	"quota-manager/internal/services"

	"github.com/gin-gonic/gin"
)

// AiGatewayAdminHandler provides passthrough admin APIs to Higress ai-quota
type AiGatewayAdminHandler struct {
	svc *services.AiGatewayAdminService
}

func NewAiGatewayAdminHandler(svc *services.AiGatewayAdminService) *AiGatewayAdminHandler {
	return &AiGatewayAdminHandler{svc: svc}
}

// -------- Quota total --------
type quotaBody struct {
	UserID string  `json:"user_id"`
	Quota  float64 `json:"quota"`
}

type quotaDeltaBody struct {
	UserID string  `json:"user_id"`
	Value  float64 `json:"value"`
}

func (h *AiGatewayAdminHandler) QueryQuota(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "user_id is required"))
		return
	}
	val, err := h.svc.QueryQuota(userID)
	if err != nil {
		h.writeGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.NewSuccessResponse(gin.H{"user_id": userID, "quota": val}, "ok"))
}

func (h *AiGatewayAdminHandler) RefreshQuota(c *gin.Context) {
	var req quotaBody
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "invalid body: "+err.Error()))
		return
	}
	if req.UserID == "" {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "user_id is required"))
		return
	}
	if err := h.svc.RefreshQuota(req.UserID, req.Quota); err != nil {
		h.writeGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.NewSuccessResponse(nil, "ok"))
}

func (h *AiGatewayAdminHandler) DeltaQuota(c *gin.Context) {
	var req quotaDeltaBody
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "invalid body: "+err.Error()))
		return
	}
	if req.UserID == "" {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "user_id is required"))
		return
	}
	if err := h.svc.DeltaQuota(req.UserID, req.Value); err != nil {
		h.writeGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.NewSuccessResponse(nil, "ok"))
}

// -------- Quota used --------
func (h *AiGatewayAdminHandler) QueryUsedQuota(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "user_id is required"))
		return
	}
	val, err := h.svc.QueryUsedQuota(userID)
	if err != nil {
		h.writeGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.NewSuccessResponse(gin.H{"user_id": userID, "quota": val}, "ok"))
}

func (h *AiGatewayAdminHandler) RefreshUsedQuota(c *gin.Context) {
	var req quotaBody
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "invalid body: "+err.Error()))
		return
	}
	if req.UserID == "" {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "user_id is required"))
		return
	}
	if err := h.svc.RefreshUsedQuota(req.UserID, req.Quota); err != nil {
		h.writeGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.NewSuccessResponse(nil, "ok"))
}

func (h *AiGatewayAdminHandler) DeltaUsedQuota(c *gin.Context) {
	var req quotaDeltaBody
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "invalid body: "+err.Error()))
		return
	}
	if req.UserID == "" {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "user_id is required"))
		return
	}
	if err := h.svc.DeltaUsedQuota(req.UserID, req.Value); err != nil {
		h.writeGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.NewSuccessResponse(nil, "ok"))
}

// -------- Star projects --------
type starProjectsBody struct {
	EmployeeNumber  string      `json:"employee_number"`
	StarredProjects interface{} `json:"starred_projects"` // string or []string
}

func (h *AiGatewayAdminHandler) QueryStarProjects(c *gin.Context) {
	emp := c.Query("employee_number")
	if emp == "" {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "employee_number is required"))
		return
	}
	csv, err := h.svc.QueryStarProjects(emp)
	if err != nil {
		h.writeGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.NewSuccessResponse(gin.H{"employee_number": emp, "starred_projects": csv}, "ok"))
}

func (h *AiGatewayAdminHandler) SetStarProjects(c *gin.Context) {
	var req starProjectsBody
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "invalid body: "+err.Error()))
		return
	}
	if req.EmployeeNumber == "" {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "employee_number is required"))
		return
	}
	var csv string
	switch v := req.StarredProjects.(type) {
	case string:
		csv = v
	case []interface{}:
		// join with commas
		for i, it := range v {
			if i > 0 {
				csv += ","
			}
			if s, ok := it.(string); ok {
				csv += s
			}
		}
	case []string:
		for i, s := range v {
			if i > 0 {
				csv += ","
			}
			csv += s
		}
	default:
		csv = ""
	}
	if err := h.svc.SetStarProjects(req.EmployeeNumber, csv); err != nil {
		h.writeGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.NewSuccessResponse(nil, "ok"))
}

// -------- Toggles --------
type toggleBody struct {
	EmployeeNumber string `json:"employee_number"`
	Enabled        bool   `json:"enabled"`
}

func (h *AiGatewayAdminHandler) QueryStarCheck(c *gin.Context) {
	emp := c.Query("employee_number")
	if emp == "" {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "employee_number is required"))
		return
	}
	enabled, err := h.svc.QueryStarCheck(emp)
	if err != nil {
		h.writeGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.NewSuccessResponse(gin.H{"employee_number": emp, "enabled": enabled}, "ok"))
}

func (h *AiGatewayAdminHandler) SetStarCheck(c *gin.Context) {
	var req toggleBody
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "invalid body: "+err.Error()))
		return
	}
	if req.EmployeeNumber == "" {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "employee_number is required"))
		return
	}
	if err := h.svc.SetStarCheck(req.EmployeeNumber, req.Enabled); err != nil {
		h.writeGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.NewSuccessResponse(nil, "ok"))
}

func (h *AiGatewayAdminHandler) QueryQuotaCheck(c *gin.Context) {
	emp := c.Query("employee_number")
	if emp == "" {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "employee_number is required"))
		return
	}
	enabled, err := h.svc.QueryQuotaCheck(emp)
	if err != nil {
		h.writeGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.NewSuccessResponse(gin.H{"employee_number": emp, "enabled": enabled}, "ok"))
}

func (h *AiGatewayAdminHandler) SetQuotaCheck(c *gin.Context) {
	var req toggleBody
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "invalid body: "+err.Error()))
		return
	}
	if req.EmployeeNumber == "" {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "employee_number is required"))
		return
	}
	if err := h.svc.SetQuotaCheck(req.EmployeeNumber, req.Enabled); err != nil {
		h.writeGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.NewSuccessResponse(nil, "ok"))
}

// -------- Models permission --------
type userModelsBody struct {
	EmployeeNumber string   `json:"employee_number"`
	Models         []string `json:"models"`
}

func (h *AiGatewayAdminHandler) SetUserModels(c *gin.Context) {
	var req userModelsBody
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "invalid body: "+err.Error()))
		return
	}
	if req.EmployeeNumber == "" {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "employee_number is required"))
		return
	}
	if err := h.svc.SetUserModels(req.EmployeeNumber, req.Models); err != nil {
		h.writeGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.NewSuccessResponse(nil, "ok"))
}

func (h *AiGatewayAdminHandler) GetUserModels(c *gin.Context) {
	emp := c.Query("employee_number")
	if emp == "" {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "employee_number is required"))
		return
	}
	models, err := h.svc.QueryUserModels(emp)
	if err != nil {
		h.writeGatewayError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.NewSuccessResponse(gin.H{"employee_number": emp, "models": models}, "ok"))
}

// -------- helpers --------
func (h *AiGatewayAdminHandler) writeGatewayError(c *gin.Context, err error) {
	// If it's HTTPError, use its status
	type httpErr interface {
		Status() int
		Error() string
	}
	if he, ok := err.(*services.ServiceError); ok {
		// not used here, reserved
		_ = he
	}
	if he, ok := err.(interface{ GetStatusCode() int }); ok {
		c.JSON(he.GetStatusCode(), response.NewErrorResponse(response.AiGatewayErrorCode, err.Error()))
		return
	}
	// try pkg utils.HTTPError
	if he2, ok := err.(interface{ StatusCode() int }); ok {
		c.JSON(he2.StatusCode(), response.NewErrorResponse(response.AiGatewayErrorCode, err.Error()))
		return
	}
	// fallback
	c.JSON(http.StatusInternalServerError, response.NewErrorResponse(response.InternalErrorCode, err.Error()))
}
