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

// QuotaCheckPermissionService handles quota check permission management
type QuotaCheckPermissionService struct {
	db               *database.DB
	aiGatewayConf    *config.AiGatewayConfig
	employeeSyncConf *config.EmployeeSyncConfig
	higressClient    HigressQuotaCheckClient
}

// HigressQuotaCheckClient interface for Higress quota check permission management
type HigressQuotaCheckClient interface {
	SetUserQuotaCheckPermission(employeeNumber string, enabled bool) error
}

// NewQuotaCheckPermissionService creates a new quota check permission service
func NewQuotaCheckPermissionService(db *database.DB, aiGatewayConf *config.AiGatewayConfig, employeeSyncConf *config.EmployeeSyncConfig, higressClient HigressQuotaCheckClient) *QuotaCheckPermissionService {
	return &QuotaCheckPermissionService{
		db:               db,
		aiGatewayConf:    aiGatewayConf,
		employeeSyncConf: employeeSyncConf,
		higressClient:    higressClient,
	}
}

func (s *QuotaCheckPermissionService) resolveEmployeeNumber(identifier string) (string, error) {
	// When employee_sync is disabled, identifier is employee_number. Validate existence.
	if s.employeeSyncConf == nil || !s.employeeSyncConf.Enabled {
		var user models.UserInfo
		if err := s.db.AuthDB.Where("id = ?", identifier).First(&user).Error; err != nil {
			return "", NewUserNotFoundError(identifier)
		}
		return identifier, nil
	}

	// When employee_sync is enabled, identifier is user_id. Map and validate mapped employee exists.
	var user models.UserInfo
	if err := s.db.AuthDB.Where("id = ?", identifier).First(&user).Error; err != nil {
		return "", NewUserNotFoundError(identifier)
	}
	if user.EmployeeNumber == "" {
		return "", NewUserNotFoundError(identifier)
	}
	var emp models.EmployeeDepartment
	if err := s.db.DB.Where("employee_number = ?", user.EmployeeNumber).First(&emp).Error; err != nil {
		return "", NewUserNotFoundError(user.EmployeeNumber)
	}
	return user.EmployeeNumber, nil
}

// SetUserQuotaCheckSetting sets quota check setting for a user
func (s *QuotaCheckPermissionService) SetUserQuotaCheckSetting(employeeNumber string, enabled bool) error {
	// Resolve identifier to employee number if needed
	if resolved, err := s.resolveEmployeeNumber(employeeNumber); err != nil {
		return err
	} else {
		employeeNumber = resolved
	}

	// Validate employee exists only when employee sync is enabled. When disabled,
	// skip existence validation to allow creating settings before HR sync.
	if s.employeeSyncConf != nil && s.employeeSyncConf.Enabled {
		var employee models.EmployeeDepartment
		if err := s.db.DB.Where("employee_number = ?", employeeNumber).First(&employee).Error; err != nil {
			return NewUserNotFoundError(employeeNumber)
		}
	}

	// Check if setting already exists
	var setting models.QuotaCheckSetting
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
			return NewDatabaseError("update quota check setting", err)
		}
	} else {
		// Create new setting
		setting = models.QuotaCheckSetting{
			TargetType:       models.TargetTypeUser,
			TargetIdentifier: employeeNumber,
			Enabled:          enabled,
		}
		if err := s.db.DB.Create(&setting).Error; err != nil {
			return NewDatabaseError("create quota check setting", err)
		}
	}

	// Update employee quota check permissions
	if err := s.UpdateEmployeeQuotaCheckPermissions(employeeNumber); err != nil {
		logger.Logger.Error("Failed to update employee quota check permissions",
			zap.String("employee_number", employeeNumber),
			zap.Error(err))
		// Continue execution - setting is already saved
	}

	// Record audit
	auditDetails := map[string]interface{}{
		"employee_number": employeeNumber,
		"enabled":         enabled,
	}
	s.recordAudit(models.OperationQuotaCheckSet, models.TargetTypeUser, employeeNumber, auditDetails)

	return nil
}

// SetDepartmentQuotaCheckSetting sets quota check setting for a department
func (s *QuotaCheckPermissionService) SetDepartmentQuotaCheckSetting(departmentName string, enabled bool) error {
	// Validate department exists - check if any employee belongs to this department
	var employeeCount int64
	err := s.db.DB.Model(&models.EmployeeDepartment{}).Where("dept_full_level_names LIKE ?", "%"+departmentName+"%").Count(&employeeCount).Error
	if err != nil {
		return NewDatabaseError("validate department existence", err)
	}

	if employeeCount == 0 {
		return NewDepartmentNotFoundError(departmentName)
	}

	// Check if setting already exists
	var setting models.QuotaCheckSetting
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
			return NewDatabaseError("update quota check setting", err)
		}
	} else {
		// Create new setting
		setting = models.QuotaCheckSetting{
			TargetType:       models.TargetTypeDepartment,
			TargetIdentifier: departmentName,
			Enabled:          enabled,
		}
		if err := s.db.DB.Create(&setting).Error; err != nil {
			return NewDatabaseError("create quota check setting", err)
		}
	}

	// Update permissions for all employees in this department
	if err := s.UpdateDepartmentQuotaCheckPermissions(departmentName); err != nil {
		logger.Logger.Error("Failed to update department quota check permissions",
			zap.String("department_name", departmentName),
			zap.Error(err))
		// Continue execution - setting is already saved
	}

	// Record audit
	auditDetails := map[string]interface{}{
		"department_name": departmentName,
		"enabled":         enabled,
	}
	s.recordAudit(models.OperationQuotaCheckSet, models.TargetTypeDepartment, departmentName, auditDetails)

	return nil
}

// GetUserEffectiveQuotaCheckSetting gets effective quota check setting for a user
func (s *QuotaCheckPermissionService) GetUserEffectiveQuotaCheckSetting(employeeNumber string) (bool, error) {
	// Resolve identifier to employee number when needed
	if resolved, err := s.resolveEmployeeNumber(employeeNumber); err != nil {
		return false, err
	} else {
		employeeNumber = resolved
	}

	// Validate employee exists only when employee sync is enabled. When disabled,
	// skip existence validation and fall back to default behavior.
	if s.employeeSyncConf != nil && s.employeeSyncConf.Enabled {
		var emp models.EmployeeDepartment
		if err := s.db.DB.Where("employee_number = ?", employeeNumber).First(&emp).Error; err != nil {
			return false, NewUserNotFoundError(employeeNumber)
		}
	}

	// Query effective setting; default to disabled if none
	var effectiveSetting models.EffectiveQuotaCheckSetting
	if err := s.db.DB.Where("employee_number = ?", employeeNumber).First(&effectiveSetting).Error; err != nil {
		return false, nil
	}

	return effectiveSetting.Enabled, nil
}

// GetDepartmentQuotaCheckSetting gets quota check setting for a department
func (s *QuotaCheckPermissionService) GetDepartmentQuotaCheckSetting(departmentName string) (bool, error) {
	// Validate department exists - check if any employee belongs to this department
	var employeeCount int64
	if err := s.db.DB.Model(&models.EmployeeDepartment{}).
		Where("dept_full_level_names LIKE ?", "%"+departmentName+"%").
		Count(&employeeCount).Error; err != nil {
		return false, NewDatabaseError("validate department existence", err)
	}

	if employeeCount == 0 {
		return false, NewDepartmentNotFoundError(departmentName)
	}

	var setting models.QuotaCheckSetting
	err := s.db.DB.Where("target_type = ? AND target_identifier = ?",
		models.TargetTypeDepartment, departmentName).First(&setting).Error
	if err != nil {
		return false, nil // Return default (disabled) if no setting found
	}

	return setting.Enabled, nil
}

// GetUserQuotaCheckSetting returns the explicit quota check setting for a user (not effective value).
// When employee_sync is enabled, the input is treated as user_id and mapped to employee_number.
// If user not found (under employee_sync), returns ErrorUserNotFound.
// If not configured, returns false, nil.
func (s *QuotaCheckPermissionService) GetUserQuotaCheckSetting(identifier string) (bool, error) {
	// Resolve identifier to employee number when needed
	if resolved, err := s.resolveEmployeeNumber(identifier); err != nil {
		return false, err
	} else {
		identifier = resolved
	}

	// Query explicit user setting
	var setting models.QuotaCheckSetting
	err := s.db.DB.Where("target_type = ? AND target_identifier = ?",
		models.TargetTypeUser, identifier).First(&setting).Error
	if err != nil {
		return false, nil
	}
	return setting.Enabled, nil
}

// UpdateEmployeeQuotaCheckPermissions updates effective quota check settings for an employee
func (s *QuotaCheckPermissionService) UpdateEmployeeQuotaCheckPermissions(employeeNumber string) error {
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
	var existingEffectiveSetting models.EffectiveQuotaCheckSetting
	err = s.db.DB.Where("employee_number = ?", employeeNumber).First(&existingEffectiveSetting).Error
	if err == nil {
		currentEnabled = existingEffectiveSetting.Enabled
	} else {
		// No existing effective setting, treat as default (disabled)
		currentEnabled = false
	}

	// Calculate new effective setting
	newEnabled, settingID := s.calculateEffectiveQuotaCheckSetting(employeeNumber, departments)

	// Check if setting has actually changed
	settingChanged := currentEnabled != newEnabled

	// For new users (no existing effective setting record), only notify if they have explicit setting
	isNewUser := err != nil
	hasNewSetting := settingID != nil // only true if there's an explicit setting

	// Update or create effective setting in database
	if err == nil {
		// Update existing record
		existingEffectiveSetting.Enabled = newEnabled
		existingEffectiveSetting.SettingID = settingID
		if err := s.db.DB.Save(&existingEffectiveSetting).Error; err != nil {
			return fmt.Errorf("failed to update effective quota check setting: %w", err)
		}
	} else {
		// Create new record
		effectiveSetting := models.EffectiveQuotaCheckSetting{
			EmployeeNumber: employeeNumber,
			Enabled:        newEnabled,
			SettingID:      settingID,
		}
		if err := s.db.DB.Create(&effectiveSetting).Error; err != nil {
			return fmt.Errorf("failed to create effective quota check setting: %w", err)
		}
	}

	// Determine if we should notify Higress
	shouldNotify := false
	notificationReason := ""

	if !isNewUser && settingChanged {
		// Existing user with setting changes
		shouldNotify = true
		if currentEnabled && !newEnabled {
			notificationReason = "quota_check_disabled"
		} else if !currentEnabled && newEnabled {
			notificationReason = "quota_check_enabled"
		}
	} else if isNewUser && hasNewSetting {
		// New user with explicit quota check setting
		shouldNotify = true
		if newEnabled {
			notificationReason = "new_user_quota_check_enabled"
		} else {
			notificationReason = "new_user_quota_check_disabled"
		}
	}

	// Notify Higress if needed
	if shouldNotify && s.higressClient != nil {
		if err := s.higressClient.SetUserQuotaCheckPermission(employeeNumber, newEnabled); err != nil {
			logger.Logger.Error("Failed to notify Higress about quota check setting change",
				zap.String("employee_number", employeeNumber),
				zap.Bool("new_enabled", newEnabled),
				zap.String("reason", notificationReason),
				zap.Error(err))
			// Don't return error - setting is already saved in database
		} else {
			logger.Logger.Info("Successfully notified Higress about quota check setting change",
				zap.String("employee_number", employeeNumber),
				zap.Bool("new_enabled", newEnabled),
				zap.String("reason", notificationReason))
		}
	}

	// Record audit
	auditDetails := map[string]interface{}{
		"employee_number": employeeNumber,
		"enabled":         newEnabled,
		"setting_changed": settingChanged,
		"reason":          notificationReason,
	}
	s.recordAudit(models.OperationQuotaCheckSettingUpdate, models.TargetTypeUser, employeeNumber, auditDetails)

	return nil
}

// UpdateDepartmentQuotaCheckPermissions updates permissions for all employees in a department
func (s *QuotaCheckPermissionService) UpdateDepartmentQuotaCheckPermissions(departmentName string) error {
	// Get all employees in this department
	var employees []models.EmployeeDepartment
	err := s.db.DB.Where("dept_full_level_names LIKE ?", "%"+departmentName+"%").Find(&employees).Error
	if err != nil {
		return fmt.Errorf("failed to get employees in department: %w", err)
	}

	// Update permissions for each employee
	for _, employee := range employees {
		if err := s.UpdateEmployeeQuotaCheckPermissions(employee.EmployeeNumber); err != nil {
			logger.Logger.Error("Failed to update quota check permissions for employee",
				zap.String("employee_number", employee.EmployeeNumber),
				zap.String("department_name", departmentName),
				zap.Error(err))
		}
	}

	return nil
}

// calculateEffectiveQuotaCheckSetting calculates effective quota check setting for an employee
func (s *QuotaCheckPermissionService) calculateEffectiveQuotaCheckSetting(employeeNumber string, departments []string) (bool, *int) {
	// Priority: User setting > Department setting (most specific department first)
	// Default: disabled (false)

	// Check user setting first
	var userSetting models.QuotaCheckSetting
	err := s.db.DB.Where("target_type = ? AND target_identifier = ?",
		models.TargetTypeUser, employeeNumber).First(&userSetting).Error
	if err == nil {
		return userSetting.Enabled, &userSetting.ID
	}

	// Check department settings (from most specific to most general)
	for i := len(departments) - 1; i >= 0; i-- {
		var deptSetting models.QuotaCheckSetting
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
func (s *QuotaCheckPermissionService) slicesEqual(a, b []string) bool {
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

// RemoveUserCompletely removes all quota check data associated with a user when they are deleted
func (s *QuotaCheckPermissionService) RemoveUserCompletely(employeeNumber string) error {
	logger.Logger.Info("Removing all quota check data for user",
		zap.String("employee_number", employeeNumber))

	// Remove user quota check setting
	if err := s.db.DB.Where("target_type = ? AND target_identifier = ?",
		models.TargetTypeUser, employeeNumber).Delete(&models.QuotaCheckSetting{}).Error; err != nil {
		logger.Logger.Error("Failed to remove user quota check setting",
			zap.String("employee_number", employeeNumber),
			zap.Error(err))
		// Continue with other cleanup even if this fails
	}

	// Remove effective quota check setting
	if err := s.db.DB.Where("employee_number = ?", employeeNumber).Delete(&models.EffectiveQuotaCheckSetting{}).Error; err != nil {
		logger.Logger.Error("Failed to remove effective quota check setting",
			zap.String("employee_number", employeeNumber),
			zap.Error(err))
		// Continue with other cleanup even if this fails
	}

	logger.Logger.Info("Successfully removed quota check data for user",
		zap.String("employee_number", employeeNumber))

	return nil
}

// recordAudit records an audit log entry
func (s *QuotaCheckPermissionService) recordAudit(operation, targetType, targetIdentifier string, details map[string]interface{}) {
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
