package services

import (
	"encoding/json"
	"fmt"
	"quota-manager/internal/config"
	"quota-manager/internal/database"
	"quota-manager/internal/models"
	"quota-manager/pkg/logger"

	"go.uber.org/zap"
)

// PermissionService handles permission management
type PermissionService struct {
	db              *database.DB
	aiGatewayConf   *config.AiGatewayConfig
	aigatewayClient HigressClient
}

// HigressClient interface for Higress permission management
type HigressClient interface {
	SetUserPermission(employeeNumber string, modelList []string) error
}

// NewPermissionService creates a new permission service
func NewPermissionService(db *database.DB, aiGatewayConf *config.AiGatewayConfig, aigatewayClient HigressClient) *PermissionService {
	return &PermissionService{
		db:              db,
		aiGatewayConf:   aiGatewayConf,
		aigatewayClient: aigatewayClient,
	}
}

// SetUserWhitelist sets whitelist for a user
func (s *PermissionService) SetUserWhitelist(employeeNumber string, modelList []string) error {
	// Check if whitelist already exists
	var whitelist models.ModelWhitelist
	err := s.db.DB.Where("target_type = ? AND target_identifier = ?",
		models.TargetTypeUser, employeeNumber).First(&whitelist).Error

	if err == nil {
		// Check if models are the same
		if s.slicesEqual(whitelist.GetAllowedModelsAsSlice(), modelList) {
			return fmt.Errorf("whitelist already exists with same models")
		}

		// Update existing whitelist
		whitelist.SetAllowedModelsFromSlice(modelList)
		if err := s.db.DB.Save(&whitelist).Error; err != nil {
			return fmt.Errorf("failed to update whitelist: %w", err)
		}
	} else {
		// Create new whitelist
		whitelist = models.ModelWhitelist{
			TargetType:       models.TargetTypeUser,
			TargetIdentifier: employeeNumber,
		}
		whitelist.SetAllowedModelsFromSlice(modelList)
		if err := s.db.DB.Create(&whitelist).Error; err != nil {
			return fmt.Errorf("failed to create whitelist: %w", err)
		}
	}

	// Update employee permissions
	if err := s.UpdateEmployeePermissions(employeeNumber); err != nil {
		logger.Logger.Error("Failed to update employee permissions",
			zap.String("employee_number", employeeNumber),
			zap.Error(err))
		// Continue execution - whitelist is already saved
	}

	// Record audit
	auditDetails := map[string]interface{}{
		"employee_number": employeeNumber,
		"models":          modelList,
	}
	s.recordAudit(models.OperationWhitelistSet, models.TargetTypeUser, employeeNumber, auditDetails)

	return nil
}

// SetDepartmentWhitelist sets whitelist for a department
func (s *PermissionService) SetDepartmentWhitelist(departmentName string, modelList []string) error {
	// Validate department exists - check if any employee belongs to this department
	var employeeCount int64
	err := s.db.DB.Model(&models.EmployeeDepartment{}).Where("dept_full_level_names LIKE ?", "%"+departmentName+"%").Count(&employeeCount).Error
	if err != nil {
		return fmt.Errorf("failed to validate department existence: %w", err)
	}

	if employeeCount == 0 {
		return fmt.Errorf("department not found: no employees belong to department '%s'", departmentName)
	}

	// Check if whitelist already exists
	var whitelist models.ModelWhitelist
	err = s.db.DB.Where("target_type = ? AND target_identifier = ?",
		models.TargetTypeDepartment, departmentName).First(&whitelist).Error

	if err == nil {
		// Check if models are the same
		if s.slicesEqual(whitelist.GetAllowedModelsAsSlice(), modelList) {
			return fmt.Errorf("whitelist already exists with same models")
		}

		// Update existing whitelist
		whitelist.SetAllowedModelsFromSlice(modelList)
		if err := s.db.DB.Save(&whitelist).Error; err != nil {
			return fmt.Errorf("failed to update whitelist: %w", err)
		}
	} else {
		// Create new whitelist
		whitelist = models.ModelWhitelist{
			TargetType:       models.TargetTypeDepartment,
			TargetIdentifier: departmentName,
		}
		whitelist.SetAllowedModelsFromSlice(modelList)
		if err := s.db.DB.Create(&whitelist).Error; err != nil {
			return fmt.Errorf("failed to create whitelist: %w", err)
		}
	}

	// Update permissions for all employees in this department
	if err := s.UpdateDepartmentPermissions(departmentName); err != nil {
		logger.Logger.Error("Failed to update department permissions",
			zap.String("department_name", departmentName),
			zap.Error(err))
		// Continue execution - whitelist is already saved
	}

	// Record audit
	auditDetails := map[string]interface{}{
		"department_name": departmentName,
		"models":          modelList,
	}
	s.recordAudit(models.OperationWhitelistSet, models.TargetTypeDepartment, departmentName, auditDetails)

	return nil
}

// GetUserEffectivePermissions gets effective permissions for a user
func (s *PermissionService) GetUserEffectivePermissions(employeeNumber string) ([]string, error) {
	// Get effective permissions directly, no need to check if employee exists
	var effectivePermission models.EffectivePermission
	err := s.db.DB.Where("employee_number = ?", employeeNumber).First(&effectivePermission).Error
	if err != nil {
		return []string{}, nil // Return empty slice if no permissions found
	}

	return effectivePermission.GetEffectiveModelsAsSlice(), nil
}

// GetDepartmentEffectivePermissions gets effective permissions for a department
func (s *PermissionService) GetDepartmentEffectivePermissions(departmentName string) ([]string, error) {
	var whitelist models.ModelWhitelist
	err := s.db.DB.Where("target_type = ? AND target_identifier = ?",
		models.TargetTypeDepartment, departmentName).First(&whitelist).Error
	if err != nil {
		return []string{}, nil // Return empty slice if no permissions found
	}

	return whitelist.GetAllowedModelsAsSlice(), nil
}

// UpdateEmployeePermissions updates effective permissions for an employee
func (s *PermissionService) UpdateEmployeePermissions(employeeNumber string) error {
	// Get employee info (optional for non-existent users)
	var employee models.EmployeeDepartment
	var departments []string
	var err error

	err = s.db.DB.Where("employee_number = ?", employeeNumber).First(&employee).Error
	if err != nil {
		// Employee doesn't exist, use empty department list
		// This allows whitelist to be set for non-existent users
		departments = []string{}
	} else {
		// Employee exists, use their department hierarchy
		departments = employee.GetDeptFullLevelNamesAsSlice()
	}

	// Calculate effective permissions
	effectiveModels, whitelistID := s.calculateEffectivePermissions(employeeNumber, departments)

	// Update or create effective permissions
	var effectivePermission models.EffectivePermission
	err = s.db.DB.Where("employee_number = ?", employeeNumber).First(&effectivePermission).Error

	if err == nil {
		// Update existing record
		effectivePermission.SetEffectiveModelsFromSlice(effectiveModels)
		effectivePermission.WhitelistID = whitelistID
		if err := s.db.DB.Save(&effectivePermission).Error; err != nil {
			return fmt.Errorf("failed to update effective permissions: %w", err)
		}
	} else {
		// Create new record
		effectivePermission = models.EffectivePermission{
			EmployeeNumber: employeeNumber,
			WhitelistID:    whitelistID,
		}
		effectivePermission.SetEffectiveModelsFromSlice(effectiveModels)
		if err := s.db.DB.Create(&effectivePermission).Error; err != nil {
			return fmt.Errorf("failed to create effective permissions: %w", err)
		}
	}

	// Update Higress with current permissions (including empty list to clear permissions)
	if s.aigatewayClient != nil {
		if err := s.aigatewayClient.SetUserPermission(employeeNumber, effectiveModels); err != nil {
			logger.Logger.Error("Failed to update Higress permissions",
				zap.String("employee_number", employeeNumber),
				zap.Strings("models", effectiveModels),
				zap.Error(err))
		}
	}

	// Record audit
	auditDetails := map[string]interface{}{
		"employee_number":  employeeNumber,
		"effective_models": effectiveModels,
		"whitelist_id":     whitelistID,
	}
	s.recordAudit(models.OperationPermissionUpdate, models.TargetTypeUser, employeeNumber, auditDetails)

	return nil
}

// UpdateDepartmentPermissions updates permissions for all employees in a department
func (s *PermissionService) UpdateDepartmentPermissions(departmentName string) error {
	// Find all employees in this department or its subdepartments
	var employees []models.EmployeeDepartment
	if err := s.db.DB.Where("dept_full_level_names LIKE ?", "%"+departmentName+"%").Find(&employees).Error; err != nil {
		return fmt.Errorf("failed to find employees in department: %w", err)
	}

	// Update permissions for each employee
	for _, employee := range employees {
		if err := s.UpdateEmployeePermissions(employee.EmployeeNumber); err != nil {
			logger.Logger.Error("Failed to update employee permissions",
				zap.String("employee_number", employee.EmployeeNumber),
				zap.Error(err))
		}
	}

	return nil
}

// calculateEffectivePermissions calculates effective permissions for an employee
func (s *PermissionService) calculateEffectivePermissions(employeeNumber string, departments []string) ([]string, *int) {
	// Priority: User whitelist > Department whitelist (most specific department first)

	// Check user whitelist first
	var userWhitelist models.ModelWhitelist
	err := s.db.DB.Where("target_type = ? AND target_identifier = ?",
		models.TargetTypeUser, employeeNumber).First(&userWhitelist).Error
	if err == nil {
		return userWhitelist.GetAllowedModelsAsSlice(), &userWhitelist.ID
	}

	// Check department whitelists (from most specific to most general)
	for i := len(departments) - 1; i >= 0; i-- {
		var deptWhitelist models.ModelWhitelist
		err := s.db.DB.Where("target_type = ? AND target_identifier = ?",
			models.TargetTypeDepartment, departments[i]).First(&deptWhitelist).Error
		if err == nil {
			return deptWhitelist.GetAllowedModelsAsSlice(), &deptWhitelist.ID
		}
	}

	// No whitelist found
	return []string{}, nil
}

// ClearUserWhitelist clears personal whitelist for a user (used when department changes)
func (s *PermissionService) ClearUserWhitelist(employeeNumber string) error {
	// Check if user whitelist exists
	var userWhitelist models.ModelWhitelist
	err := s.db.DB.Where("target_type = ? AND target_identifier = ?",
		models.TargetTypeUser, employeeNumber).First(&userWhitelist).Error
	if err != nil {
		// No user whitelist found, nothing to clear
		return nil
	}

	// Delete user whitelist
	if err := s.db.DB.Delete(&userWhitelist).Error; err != nil {
		return fmt.Errorf("failed to delete user whitelist: %w", err)
	}

	// Record audit
	auditDetails := map[string]interface{}{
		"employee_number": employeeNumber,
		"reason":          "department_change",
		"cleared_models":  userWhitelist.GetAllowedModelsAsSlice(),
	}
	s.recordAudit("whitelist_clear", models.TargetTypeUser, employeeNumber, auditDetails)

	logger.Logger.Info("Cleared user personal whitelist due to department change",
		zap.String("employee_number", employeeNumber),
		zap.Strings("cleared_models", userWhitelist.GetAllowedModelsAsSlice()))

	return nil
}

// RemoveUserCompletely removes all data associated with a user when they are deleted
func (s *PermissionService) RemoveUserCompletely(employeeNumber string) error {
	// Clear user whitelist (if exists)
	if err := s.ClearUserWhitelist(employeeNumber); err != nil {
		logger.Logger.Error("Failed to clear user whitelist during complete removal",
			zap.String("employee_number", employeeNumber),
			zap.Error(err))
		// Continue with removal even if whitelist clearing fails
	}

	// Remove effective permissions
	var effectivePermission models.EffectivePermission
	err := s.db.DB.Where("employee_number = ?", employeeNumber).First(&effectivePermission).Error
	if err == nil {
		// Record what we're removing for audit
		removedModels := effectivePermission.GetEffectiveModelsAsSlice()

		if err := s.db.DB.Delete(&effectivePermission).Error; err != nil {
			return fmt.Errorf("failed to delete effective permissions: %w", err)
		}

		// Record audit
		auditDetails := map[string]interface{}{
			"employee_number": employeeNumber,
			"reason":          "employee_removal",
			"removed_models":  removedModels,
		}
		s.recordAudit("user_complete_removal", models.TargetTypeUser, employeeNumber, auditDetails)

		logger.Logger.Info("Completely removed user data",
			zap.String("employee_number", employeeNumber),
			zap.Strings("removed_models", removedModels))
	}
	// If no effective permissions found, that's fine - nothing to remove

	return nil
}

// slicesEqual checks if two string slices are equal
func (s *PermissionService) slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// recordAudit records an audit log entry
func (s *PermissionService) recordAudit(operation, targetType, targetIdentifier string, details map[string]interface{}) {
	detailsJSON, _ := json.Marshal(details)
	audit := &models.PermissionAudit{
		Operation:        operation,
		TargetType:       targetType,
		TargetIdentifier: targetIdentifier,
		Details:          string(detailsJSON),
	}

	if err := s.db.DB.Create(audit).Error; err != nil {
		logger.Logger.Error("Failed to record audit", zap.Error(err))
	}
}
