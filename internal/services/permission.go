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
	db               *database.DB
	aiGatewayConf    *config.AiGatewayConfig
	employeeSyncConf *config.EmployeeSyncConfig
	aigatewayClient  HigressClient
}

// HigressClient interface for Higress permission management
type HigressClient interface {
	SetUserPermission(employeeNumber string, modelList []string) error
}

// NewPermissionService creates a new permission service
func NewPermissionService(db *database.DB, aiGatewayConf *config.AiGatewayConfig, employeeSyncConf *config.EmployeeSyncConfig, aigatewayClient HigressClient) *PermissionService {
	return &PermissionService{
		db:               db,
		aiGatewayConf:    aiGatewayConf,
		employeeSyncConf: employeeSyncConf,
		aigatewayClient:  aigatewayClient,
	}
}

// resolveEmployeeNumber resolves the input identifier to an employee number based on configuration.
// When employee sync is enabled via configManager, the input is treated as user_id and mapped via auth_users.
// Otherwise, the input is treated directly as an employee_number for backward compatibility.
func (s *PermissionService) resolveEmployeeNumber(identifier string) (string, error) {
	if s.employeeSyncConf == nil || !s.employeeSyncConf.Enabled {
		return identifier, nil
	}

	// employee sync enabled: identifier is user_id -> map to employee_number via auth_users
	var user models.UserInfo
	if err := s.db.AuthDB.Where("id = ?", identifier).First(&user).Error; err != nil {
		return "", NewUserNotFoundError(identifier)
	}
	if user.EmployeeNumber == "" {
		return "", NewUserNotFoundError(identifier)
	}
	return user.EmployeeNumber, nil
}

// SetUserWhitelist sets whitelist for a user
func (s *PermissionService) SetUserWhitelist(employeeNumber string, modelList []string) error {
	// Resolve identifier to employee number when needed
	if resolved, err := s.resolveEmployeeNumber(employeeNumber); err != nil {
		return err
	} else {
		employeeNumber = resolved
	}
	// Check if user exists when employee_sync is enabled
	if s.employeeSyncConf.Enabled {
		var employee models.EmployeeDepartment
		err := s.db.DB.Where("employee_number = ?", employeeNumber).First(&employee).Error
		if err != nil {
			return NewUserNotFoundError(employeeNumber)
		}
	}

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
			return NewDatabaseError("update whitelist", err)
		}
	} else {
		// Create new whitelist
		whitelist = models.ModelWhitelist{
			TargetType:       models.TargetTypeUser,
			TargetIdentifier: employeeNumber,
		}
		whitelist.SetAllowedModelsFromSlice(modelList)
		if err := s.db.DB.Create(&whitelist).Error; err != nil {
			return NewDatabaseError("create whitelist", err)
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
		return NewDatabaseError("validate department existence", err)
	}

	if employeeCount == 0 {
		return NewDepartmentNotFoundError(departmentName)
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
			return NewDatabaseError("update whitelist", err)
		}
	} else {
		// Create new whitelist
		whitelist = models.ModelWhitelist{
			TargetType:       models.TargetTypeDepartment,
			TargetIdentifier: departmentName,
		}
		whitelist.SetAllowedModelsFromSlice(modelList)
		if err := s.db.DB.Create(&whitelist).Error; err != nil {
			return NewDatabaseError("create whitelist", err)
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

	// Get current effective permissions from database (if exists)
	var currentEffectiveModels []string
	var existingEffectivePermission models.EffectivePermission
	err = s.db.DB.Where("employee_number = ?", employeeNumber).First(&existingEffectivePermission).Error
	if err == nil {
		currentEffectiveModels = existingEffectivePermission.GetEffectiveModelsAsSlice()
	} else {
		// No existing effective permissions, treat as empty
		currentEffectiveModels = []string{}
	}

	// Calculate new effective permissions
	newEffectiveModels, whitelistID := s.calculateEffectivePermissions(employeeNumber, departments)

	// Check if permissions have actually changed
	permissionsChanged := !s.slicesEqual(currentEffectiveModels, newEffectiveModels)

	// For new users (no existing effective permission record), only notify if they have permissions
	isNewUser := err != nil
	hasCurrentPermissions := len(currentEffectiveModels) > 0
	hasNewPermissions := len(newEffectiveModels) > 0

	// Update or create effective permissions in database
	if err == nil {
		// Update existing record
		existingEffectivePermission.SetEffectiveModelsFromSlice(newEffectiveModels)
		existingEffectivePermission.WhitelistID = whitelistID
		if err := s.db.DB.Save(&existingEffectivePermission).Error; err != nil {
			return fmt.Errorf("failed to update effective permissions: %w", err)
		}
	} else {
		// Create new record
		effectivePermission := models.EffectivePermission{
			EmployeeNumber: employeeNumber,
			WhitelistID:    whitelistID,
		}
		effectivePermission.SetEffectiveModelsFromSlice(newEffectiveModels)
		if err := s.db.DB.Create(&effectivePermission).Error; err != nil {
			return fmt.Errorf("failed to create effective permissions: %w", err)
		}
	}

	// Determine if we should notify Aigateway
	// Only notify when:
	// 1. Existing user with permission changes (including from something to nothing or nothing to something)
	// 2. New user who gets initial permissions (not for new users with no permissions)
	shouldNotify := false
	notificationReason := ""

	if !isNewUser && permissionsChanged {
		// Existing user with permission changes
		shouldNotify = true
		if hasCurrentPermissions && !hasNewPermissions {
			notificationReason = "permissions_cleared"
		} else if !hasCurrentPermissions && hasNewPermissions {
			notificationReason = "permissions_granted"
		} else {
			notificationReason = "permissions_modified"
		}
	} else if isNewUser && hasNewPermissions {
		// New user with initial permissions
		shouldNotify = true
		notificationReason = "new_user_with_permissions"
	}

	// DEBUG: Log detailed information for troubleshooting
	logger.Logger.Info("UpdateEmployeePermissions decision",
		zap.String("employee_number", employeeNumber),
		zap.Strings("current_models", currentEffectiveModels),
		zap.Strings("new_models", newEffectiveModels),
		zap.Bool("permissions_changed", permissionsChanged),
		zap.Bool("is_new_user", isNewUser),
		zap.Bool("has_current_permissions", hasCurrentPermissions),
		zap.Bool("has_new_permissions", hasNewPermissions),
		zap.Bool("should_notify", shouldNotify),
		zap.String("notification_reason", notificationReason))

	if shouldNotify && s.aigatewayClient != nil {
		if err := s.aigatewayClient.SetUserPermission(employeeNumber, newEffectiveModels); err != nil {
			logger.Logger.Error("Failed to update Higress permissions",
				zap.String("employee_number", employeeNumber),
				zap.Strings("previous_models", currentEffectiveModels),
				zap.Strings("new_models", newEffectiveModels),
				zap.String("reason", notificationReason),
				zap.Error(err))
		} else {
			logger.Logger.Info("Successfully updated Aigateway permissions",
				zap.String("employee_number", employeeNumber),
				zap.Strings("previous_models", currentEffectiveModels),
				zap.Strings("new_models", newEffectiveModels),
				zap.String("reason", notificationReason))
		}
	} else {
		if isNewUser && !hasNewPermissions {
			logger.Logger.Debug("Skipping Aigateway notification for new user with no permissions",
				zap.String("employee_number", employeeNumber))
		} else if !permissionsChanged {
			logger.Logger.Debug("Skipping Aigateway notification - no permission changes detected",
				zap.String("employee_number", employeeNumber),
				zap.Strings("current_models", currentEffectiveModels))
		}
	}

	// Record audit
	auditDetails := map[string]interface{}{
		"employee_number":         employeeNumber,
		"previous_models":         currentEffectiveModels,
		"new_effective_models":    newEffectiveModels,
		"whitelist_id":            whitelistID,
		"permissions_changed":     permissionsChanged,
		"is_new_user":             isNewUser,
		"has_current_permissions": hasCurrentPermissions,
		"has_new_permissions":     hasNewPermissions,
		"aigateway_notified":      shouldNotify,
		"notification_reason":     notificationReason,
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
	// Note: Empty whitelist (empty model list) is treated as "not configured", continue to check parent level

	// Check user whitelist first
	var userWhitelist models.ModelWhitelist
	err := s.db.DB.Where("target_type = ? AND target_identifier = ?",
		models.TargetTypeUser, employeeNumber).First(&userWhitelist).Error
	if err == nil {
		userModels := userWhitelist.GetAllowedModelsAsSlice()
		// Only return user whitelist if it contains models
		// Empty whitelist is treated as "not configured", fall back to department whitelist
		if len(userModels) > 0 {
			return userModels, &userWhitelist.ID
		}
		// User has empty whitelist, continue to check department whitelists
	}

	// Check department whitelists (from most specific to most general)
	for i := len(departments) - 1; i >= 0; i-- {
		var deptWhitelist models.ModelWhitelist
		err := s.db.DB.Where("target_type = ? AND target_identifier = ?",
			models.TargetTypeDepartment, departments[i]).First(&deptWhitelist).Error
		if err == nil {
			deptModels := deptWhitelist.GetAllowedModelsAsSlice()
			// Only return department whitelist if it contains models
			// Empty whitelist is treated as "not configured", check parent department
			if len(deptModels) > 0 {
				return deptModels, &deptWhitelist.ID
			}
			// Department has empty whitelist, continue to check parent department
		}
	}

	// No non-empty whitelist found
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

		// Notify Aigateway to clear permissions if user had any permissions
		if len(removedModels) > 0 && s.aigatewayClient != nil {
			if err := s.aigatewayClient.SetUserPermission(employeeNumber, []string{}); err != nil {
				logger.Logger.Error("Failed to clear Aigateway permissions for removed user",
					zap.String("employee_number", employeeNumber),
					zap.Strings("removed_models", removedModels),
					zap.Error(err))
			} else {
				logger.Logger.Info("Successfully cleared Aigateway permissions for removed user",
					zap.String("employee_number", employeeNumber),
					zap.Strings("removed_models", removedModels))
			}
		}

		if err := s.db.DB.Delete(&effectivePermission).Error; err != nil {
			return fmt.Errorf("failed to delete effective permissions: %w", err)
		}

		// Record audit
		auditDetails := map[string]interface{}{
			"employee_number":    employeeNumber,
			"reason":             "employee_removal",
			"removed_models":     removedModels,
			"aigateway_notified": len(removedModels) > 0 && s.aigatewayClient != nil,
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
