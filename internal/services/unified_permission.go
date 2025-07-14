package services

import (
	"quota-manager/internal/models"
)

// UnifiedPermissionService handles unified permission queries and sync
type UnifiedPermissionService struct {
	permissionService           *PermissionService
	starCheckPermissionService  *StarCheckPermissionService
	quotaCheckPermissionService *QuotaCheckPermissionService
	employeeSyncService         *EmployeeSyncService
}

// NewUnifiedPermissionService creates a new unified permission service
func NewUnifiedPermissionService(
	permissionService *PermissionService,
	starCheckPermissionService *StarCheckPermissionService,
	quotaCheckPermissionService *QuotaCheckPermissionService,
	employeeSyncService *EmployeeSyncService,
) *UnifiedPermissionService {
	return &UnifiedPermissionService{
		permissionService:           permissionService,
		starCheckPermissionService:  starCheckPermissionService,
		quotaCheckPermissionService: quotaCheckPermissionService,
		employeeSyncService:         employeeSyncService,
	}
}

// GetModelEffectivePermissions gets effective model permissions
func (s *UnifiedPermissionService) GetModelEffectivePermissions(targetType, targetIdentifier string) ([]string, error) {
	if targetType == models.TargetTypeUser {
		return s.permissionService.GetUserEffectivePermissions(targetIdentifier)
	} else {
		return s.permissionService.GetDepartmentEffectivePermissions(targetIdentifier)
	}
}

// GetStarCheckEffectivePermissions gets effective star check settings
func (s *UnifiedPermissionService) GetStarCheckEffectivePermissions(targetType, targetIdentifier string) (bool, error) {
	if targetType == models.TargetTypeUser {
		return s.starCheckPermissionService.GetUserEffectiveStarCheckSetting(targetIdentifier)
	} else {
		return s.starCheckPermissionService.GetDepartmentStarCheckSetting(targetIdentifier)
	}
}

// GetQuotaCheckEffectivePermissions gets effective quota check settings
func (s *UnifiedPermissionService) GetQuotaCheckEffectivePermissions(targetType, targetIdentifier string) (bool, error) {
	if targetType == models.TargetTypeUser {
		return s.quotaCheckPermissionService.GetUserEffectiveQuotaCheckSetting(targetIdentifier)
	} else {
		return s.quotaCheckPermissionService.GetDepartmentQuotaCheckSetting(targetIdentifier)
	}
}

// TriggerEmployeeSync triggers comprehensive employee synchronization
func (s *UnifiedPermissionService) TriggerEmployeeSync() error {
	return s.employeeSyncService.SyncEmployees()
}
