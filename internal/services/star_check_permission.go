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

// StarCheckPermissionService handles star check permission management
type StarCheckPermissionService struct {
	db               *database.DB
	aiGatewayConf    *config.AiGatewayConfig
	employeeSyncConf *config.EmployeeSyncConfig
	higressClient    HigressStarCheckClient
}

// HigressStarCheckClient interface for Higress star check permission management
type HigressStarCheckClient interface {
	SetUserStarCheckPermission(employeeNumber string, enabled bool) error
}

// NewStarCheckPermissionService creates a new star check permission service
func NewStarCheckPermissionService(db *database.DB, aiGatewayConf *config.AiGatewayConfig, employeeSyncConf *config.EmployeeSyncConfig, higressClient HigressStarCheckClient) *StarCheckPermissionService {
	return &StarCheckPermissionService{
		db:               db,
		aiGatewayConf:    aiGatewayConf,
		employeeSyncConf: employeeSyncConf,
		higressClient:    higressClient,
	}
}

// SetUserStarCheckSetting sets star check setting for a user
func (s *StarCheckPermissionService) SetUserStarCheckSetting(employeeNumber string, enabled bool) error {
	// Check if user exists when employee_sync is enabled
	if s.employeeSyncConf.Enabled {
		var employee models.EmployeeDepartment
		err := s.db.DB.Where("employee_number = ?", employeeNumber).First(&employee).Error
		if err != nil {
			return fmt.Errorf("user not found: employee number '%s' does not exist", employeeNumber)
		}
	}

	// Check if setting already exists
	var setting models.StarCheckSetting
	err := s.db.DB.Where("target_type = ? AND target_identifier = ?",
		models.TargetTypeUser, employeeNumber).First(&setting).Error

	if err == nil {
		// Check if setting is the same
		if setting.Enabled == enabled {
			// Setting already exists with same value - this is ok (idempotent operation)
			return nil
		}

		// Update existing setting
		setting.Enabled = enabled
		if err := s.db.DB.Save(&setting).Error; err != nil {
			return fmt.Errorf("failed to update star check setting: %w", err)
		}
	} else {
		// Create new setting
		setting = models.StarCheckSetting{
			TargetType:       models.TargetTypeUser,
			TargetIdentifier: employeeNumber,
			Enabled:          enabled,
		}
		if err := s.db.DB.Create(&setting).Error; err != nil {
			return fmt.Errorf("failed to create star check setting: %w", err)
		}
	}

	// Update employee star check permissions
	if err := s.UpdateEmployeeStarCheckPermissions(employeeNumber); err != nil {
		logger.Logger.Error("Failed to update employee star check permissions",
			zap.String("employee_number", employeeNumber),
			zap.Error(err))
		// Continue execution - setting is already saved
	}

	// Record audit
	auditDetails := map[string]interface{}{
		"employee_number": employeeNumber,
		"enabled":         enabled,
	}
	s.recordAudit(models.OperationStarCheckSet, models.TargetTypeUser, employeeNumber, auditDetails)

	return nil
}

// SetDepartmentStarCheckSetting sets star check setting for a department
func (s *StarCheckPermissionService) SetDepartmentStarCheckSetting(departmentName string, enabled bool) error {
	// Validate department exists - check if any employee belongs to this department
	var employeeCount int64
	err := s.db.DB.Model(&models.EmployeeDepartment{}).Where("dept_full_level_names LIKE ?", "%"+departmentName+"%").Count(&employeeCount).Error
	if err != nil {
		return fmt.Errorf("failed to validate department existence: %w", err)
	}

	if employeeCount == 0 {
		return fmt.Errorf("department not found: no employees belong to department '%s'", departmentName)
	}

	// Check if setting already exists
	var setting models.StarCheckSetting
	err = s.db.DB.Where("target_type = ? AND target_identifier = ?",
		models.TargetTypeDepartment, departmentName).First(&setting).Error

	if err == nil {
		// Check if setting is the same
		if setting.Enabled == enabled {
			// Setting already exists with same value - this is ok (idempotent operation)
			return nil
		}

		// Update existing setting
		setting.Enabled = enabled
		if err := s.db.DB.Save(&setting).Error; err != nil {
			return fmt.Errorf("failed to update star check setting: %w", err)
		}
	} else {
		// Create new setting
		setting = models.StarCheckSetting{
			TargetType:       models.TargetTypeDepartment,
			TargetIdentifier: departmentName,
			Enabled:          enabled,
		}
		if err := s.db.DB.Create(&setting).Error; err != nil {
			return fmt.Errorf("failed to create star check setting: %w", err)
		}
	}

	// Update permissions for all employees in this department
	if err := s.UpdateDepartmentStarCheckPermissions(departmentName); err != nil {
		logger.Logger.Error("Failed to update department star check permissions",
			zap.String("department_name", departmentName),
			zap.Error(err))
		// Continue execution - setting is already saved
	}

	// Record audit
	auditDetails := map[string]interface{}{
		"department_name": departmentName,
		"enabled":         enabled,
	}
	s.recordAudit(models.OperationStarCheckSet, models.TargetTypeDepartment, departmentName, auditDetails)

	return nil
}

// GetUserEffectiveStarCheckSetting gets effective star check setting for a user
func (s *StarCheckPermissionService) GetUserEffectiveStarCheckSetting(employeeNumber string) (bool, error) {
	// Get effective setting directly, no need to check if employee exists
	var effectiveSetting models.EffectiveStarCheckSetting
	err := s.db.DB.Where("employee_number = ?", employeeNumber).First(&effectiveSetting).Error
	if err != nil {
		return false, nil // Return default (disabled) if no setting found
	}

	return effectiveSetting.Enabled, nil
}

// GetDepartmentStarCheckSetting gets star check setting for a department
func (s *StarCheckPermissionService) GetDepartmentStarCheckSetting(departmentName string) (bool, error) {
	var setting models.StarCheckSetting
	err := s.db.DB.Where("target_type = ? AND target_identifier = ?",
		models.TargetTypeDepartment, departmentName).First(&setting).Error
	if err != nil {
		return false, nil // Return default (disabled) if no setting found
	}

	return setting.Enabled, nil
}

// UpdateEmployeeStarCheckPermissions updates effective star check settings for an employee
func (s *StarCheckPermissionService) UpdateEmployeeStarCheckPermissions(employeeNumber string) error {
	// Get employee info (optional for non-existent users)
	var employee models.EmployeeDepartment
	var departments []string
	var err error

	err = s.db.DB.Where("employee_number = ?", employeeNumber).First(&employee).Error
	if err != nil {
		// Employee doesn't exist, use empty department list
		departments = []string{}
	} else {
		// Employee exists, use their department hierarchy
		departments = employee.GetDeptFullLevelNamesAsSlice()
	}

	// Get current effective setting from database (if exists)
	var currentEnabled bool
	var existingEffectiveSetting models.EffectiveStarCheckSetting
	err = s.db.DB.Where("employee_number = ?", employeeNumber).First(&existingEffectiveSetting).Error
	if err == nil {
		currentEnabled = existingEffectiveSetting.Enabled
	} else {
		// No existing effective setting, treat as default (disabled)
		currentEnabled = false
	}

	// Calculate new effective setting
	newEnabled, settingID := s.calculateEffectiveStarCheckSetting(employeeNumber, departments)

	// Check if setting has actually changed
	settingChanged := currentEnabled != newEnabled

	// For new users (no existing effective setting record), only notify if they have explicit setting
	isNewUser := err != nil
	hasCurrentSetting := !currentEnabled // disabled is considered "has specific setting"
	hasNewSetting := settingID != nil    // only true if there's an explicit setting

	// Update or create effective setting in database
	if err == nil {
		// Update existing record
		existingEffectiveSetting.Enabled = newEnabled
		existingEffectiveSetting.SettingID = settingID
		if err := s.db.DB.Save(&existingEffectiveSetting).Error; err != nil {
			return fmt.Errorf("failed to update effective star check setting: %w", err)
		}
	} else {
		// Create new record
		effectiveSetting := models.EffectiveStarCheckSetting{
			EmployeeNumber: employeeNumber,
			Enabled:        newEnabled,
			SettingID:      settingID,
		}
		if err := s.db.DB.Create(&effectiveSetting).Error; err != nil {
			return fmt.Errorf("failed to create effective star check setting: %w", err)
		}
	}

	// Determine if we should notify Higress
	shouldNotify := false
	notificationReason := ""

	if !isNewUser && settingChanged {
		// Existing user with setting changes
		shouldNotify = true
		if currentEnabled && !newEnabled {
			notificationReason = "star_check_disabled"
		} else if !currentEnabled && newEnabled {
			notificationReason = "star_check_enabled"
		}
	} else if isNewUser && hasNewSetting {
		// New user with explicit star check setting
		shouldNotify = true
		if newEnabled {
			notificationReason = "new_user_star_check_enabled"
		} else {
			notificationReason = "new_user_star_check_disabled"
		}
	}

	// Notify Higress if needed
	if shouldNotify && s.higressClient != nil {
		if err := s.higressClient.SetUserStarCheckPermission(employeeNumber, newEnabled); err != nil {
			logger.Logger.Error("Failed to notify Higress about star check setting change",
				zap.String("employee_number", employeeNumber),
				zap.Bool("new_enabled", newEnabled),
				zap.String("reason", notificationReason),
				zap.Error(err))
			// Don't return error - setting is already saved in database
		} else {
			logger.Logger.Info("Successfully notified Higress about star check setting change",
				zap.String("employee_number", employeeNumber),
				zap.Bool("new_enabled", newEnabled),
				zap.String("reason", notificationReason))
		}
	}

	// Record audit
	auditDetails := map[string]interface{}{
		"employee_number":     employeeNumber,
		"previous_enabled":    currentEnabled,
		"new_enabled":         newEnabled,
		"setting_id":          settingID,
		"setting_changed":     settingChanged,
		"is_new_user":         isNewUser,
		"has_current_setting": hasCurrentSetting,
		"has_new_setting":     hasNewSetting,
		"higress_notified":    shouldNotify,
		"notification_reason": notificationReason,
	}
	s.recordAudit(models.OperationStarCheckSettingUpdate, models.TargetTypeUser, employeeNumber, auditDetails)

	return nil
}

// UpdateDepartmentStarCheckPermissions updates star check settings for all employees in a department
func (s *StarCheckPermissionService) UpdateDepartmentStarCheckPermissions(departmentName string) error {
	// Find all employees in this department or its subdepartments
	var employees []models.EmployeeDepartment
	if err := s.db.DB.Where("dept_full_level_names LIKE ?", "%"+departmentName+"%").Find(&employees).Error; err != nil {
		return fmt.Errorf("failed to find employees in department: %w", err)
	}

	// Update settings for each employee
	for _, employee := range employees {
		if err := s.UpdateEmployeeStarCheckPermissions(employee.EmployeeNumber); err != nil {
			logger.Logger.Error("Failed to update employee star check permissions",
				zap.String("employee_number", employee.EmployeeNumber),
				zap.Error(err))
		}
	}

	return nil
}

// calculateEffectiveStarCheckSetting calculates effective star check setting for an employee
func (s *StarCheckPermissionService) calculateEffectiveStarCheckSetting(employeeNumber string, departments []string) (bool, *int) {
	// Priority: User setting > Department setting (most specific department first)
	// Default: disabled (false)

	// Check user setting first
	var userSetting models.StarCheckSetting
	err := s.db.DB.Where("target_type = ? AND target_identifier = ?",
		models.TargetTypeUser, employeeNumber).First(&userSetting).Error
	if err == nil {
		return userSetting.Enabled, &userSetting.ID
	}

	// Check department settings (from most specific to most general)
	for i := len(departments) - 1; i >= 0; i-- {
		var deptSetting models.StarCheckSetting
		err := s.db.DB.Where("target_type = ? AND target_identifier = ?",
			models.TargetTypeDepartment, departments[i]).First(&deptSetting).Error
		if err == nil {
			return deptSetting.Enabled, &deptSetting.ID
		}
	}

	// No setting found, return default (disabled)
	return false, nil
}

// slicesEqual compares two string slices for equality
func (s *StarCheckPermissionService) slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// RemoveUserCompletely removes all star check data associated with a user when they are deleted
func (s *StarCheckPermissionService) RemoveUserCompletely(employeeNumber string) error {
	// Remove user star check settings (if exists)
	var userSetting models.StarCheckSetting
	err := s.db.DB.Where("target_type = ? AND target_identifier = ?",
		models.TargetTypeUser, employeeNumber).First(&userSetting).Error
	if err == nil {
		// Delete user star check setting
		if err := s.db.DB.Delete(&userSetting).Error; err != nil {
			logger.Logger.Error("Failed to delete user star check setting during complete removal",
				zap.String("employee_number", employeeNumber),
				zap.Error(err))
			// Continue with removal even if star check setting deletion fails
		}
	}

	// Remove effective star check settings
	var effectiveSetting models.EffectiveStarCheckSetting
	err = s.db.DB.Where("employee_number = ?", employeeNumber).First(&effectiveSetting).Error
	if err == nil {
		// Record what we're removing for audit
		removedEnabled := effectiveSetting.Enabled

		// Notify Higress to clear star check setting if user had explicit setting
		if s.higressClient != nil {
			if err := s.higressClient.SetUserStarCheckPermission(employeeNumber, false); err != nil {
				logger.Logger.Error("Failed to clear Higress star check permission for removed user",
					zap.String("employee_number", employeeNumber),
					zap.Bool("removed_enabled", removedEnabled),
					zap.Error(err))
			} else {
				logger.Logger.Info("Successfully cleared Higress star check permission for removed user",
					zap.String("employee_number", employeeNumber),
					zap.Bool("removed_enabled", removedEnabled))
			}
		}

		if err := s.db.DB.Delete(&effectiveSetting).Error; err != nil {
			return fmt.Errorf("failed to delete effective star check setting: %w", err)
		}

		// Record audit
		auditDetails := map[string]interface{}{
			"employee_number":  employeeNumber,
			"reason":           "employee_removal",
			"removed_enabled":  removedEnabled,
			"higress_notified": s.higressClient != nil,
		}
		s.recordAudit("user_star_check_complete_removal", models.TargetTypeUser, employeeNumber, auditDetails)

		logger.Logger.Info("Completely removed user star check data",
			zap.String("employee_number", employeeNumber),
			zap.Bool("removed_enabled", removedEnabled))
	}

	return nil
}

// recordAudit records audit information
func (s *StarCheckPermissionService) recordAudit(operation, targetType, targetIdentifier string, details map[string]interface{}) {
	detailsJSON, _ := json.Marshal(details)
	audit := models.PermissionAudit{
		Operation:        operation,
		TargetType:       targetType,
		TargetIdentifier: targetIdentifier,
		Details:          string(detailsJSON),
	}

	if err := s.db.DB.Create(&audit).Error; err != nil {
		logger.Logger.Error("Failed to record audit",
			zap.String("operation", operation),
			zap.String("target_type", targetType),
			zap.String("target_identifier", targetIdentifier),
			zap.Error(err))
	}
}
