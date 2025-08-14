package handlers

import (
	"net/http"
	"quota-manager/internal/response"
	"quota-manager/internal/services"

	"github.com/gin-gonic/gin"
)

// ScanHandler handles unified scan operations
type ScanHandler struct {
	strategyService          *services.StrategyService
	unifiedPermissionService *services.UnifiedPermissionService
	schedulerService         *services.SchedulerService
	quotaService             *services.QuotaService
}

// NewScanHandler creates a new scan handler
func NewScanHandler(strategyService *services.StrategyService, unifiedPermissionService *services.UnifiedPermissionService, schedulerService *services.SchedulerService, quotaService *services.QuotaService) *ScanHandler {
	return &ScanHandler{
		strategyService:          strategyService,
		unifiedPermissionService: unifiedPermissionService,
		schedulerService:         schedulerService,
		quotaService:             quotaService,
	}
}

// ScanRequest represents the scan request body
type ScanRequest struct {
	Type string `json:"type" validate:"required,oneof=strategy employee-sync expire-quotas sync-quotas"`
}

// TriggerScan handles unified scan triggering
func (h *ScanHandler) TriggerScan(c *gin.Context) {
	var req ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "Invalid request body: "+err.Error()))
		return
	}

	switch req.Type {
	case "strategy":
		go h.strategyService.TraverseSingleStrategies()
		c.JSON(http.StatusOK, response.NewSuccessResponse(nil, "Strategy scan triggered successfully"))
	case "employee-sync":
		if err := h.unifiedPermissionService.TriggerEmployeeSync(); err != nil {
			c.JSON(http.StatusInternalServerError, response.NewErrorResponse(response.EmployeeSyncFailedCode, "Failed to sync employees: "+err.Error()))
			return
		}
		c.JSON(http.StatusOK, response.NewSuccessResponse(nil, "Employee sync triggered successfully"))
	case "expire-quotas":
		go h.schedulerService.ExpireQuotasTask()
		c.JSON(http.StatusOK, response.NewSuccessResponse(nil, "Quota expiry task triggered successfully"))
	case "sync-quotas":
		go h.quotaService.SyncQuotasWithAiGateway()
		c.JSON(http.StatusOK, response.NewSuccessResponse(nil, "Quota sync task triggered successfully"))
	default:
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "Invalid scan type: "+req.Type))
	}
}
