package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"quota-manager/internal/config"
	"quota-manager/internal/database"
	"quota-manager/internal/models"
	"quota-manager/pkg/logger"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// EmployeeSyncService handles employee synchronization
type EmployeeSyncService struct {
	db               *database.DB
	employeeSyncConf *config.EmployeeSyncConfig
	permissionSvc    *PermissionService
	cron             *cron.Cron
}

// NewEmployeeSyncService creates a new employee sync service
func NewEmployeeSyncService(db *database.DB, employeeSyncConf *config.EmployeeSyncConfig, permissionSvc *PermissionService) *EmployeeSyncService {
	return &EmployeeSyncService{
		db:               db,
		employeeSyncConf: employeeSyncConf,
		permissionSvc:    permissionSvc,
		cron:             cron.New(cron.WithSeconds()),
	}
}

// IsEmployeeDepartmentTableEmpty checks if the employee_department table is empty
func (s *EmployeeSyncService) IsEmployeeDepartmentTableEmpty() (bool, error) {
	var count int64
	if err := s.db.Model(&models.EmployeeDepartment{}).Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to count employee_department records: %w", err)
	}
	return count == 0, nil
}

// StartCron starts the employee sync cron job
func (s *EmployeeSyncService) StartCron() error {
	if !s.employeeSyncConf.Enabled {
		logger.Logger.Info("Employee sync is disabled, skipping cron setup")
		return nil
	}

	// Fixed schedule: every day at 1:00 AM (6 fields with seconds)
	syncInterval := "0 0 1 * * *"
	logger.Logger.Info("Setting up employee sync cron", zap.String("schedule", "every day at 1:00 AM"))

	// Add employee sync task
	_, err := s.cron.AddFunc(syncInterval, func() {
		logger.Logger.Info("Starting scheduled employee synchronization")
		if err := s.SyncEmployees(); err != nil {
			logger.Logger.Error("Scheduled employee sync failed", zap.Error(err))
		} else {
			logger.Logger.Info("Scheduled employee synchronization completed successfully")
		}
	})
	if err != nil {
		return fmt.Errorf("failed to add employee sync task: %w", err)
	}

	s.cron.Start()
	logger.Logger.Info("Employee sync cron started", zap.String("schedule", "daily at 1:00 AM"))
	return nil
}

// StopCron stops the employee sync cron job
func (s *EmployeeSyncService) StopCron() {
	if s.cron != nil {
		s.cron.Stop()
		logger.Logger.Info("Employee sync cron stopped")
	}
}

// TriggerInitialSyncIfNeeded checks if employee_department table is empty and triggers sync if needed
func (s *EmployeeSyncService) TriggerInitialSyncIfNeeded() error {
	if !s.employeeSyncConf.Enabled {
		logger.Logger.Info("Employee sync is disabled, skipping initial sync check")
		return nil
	}

	isEmpty, err := s.IsEmployeeDepartmentTableEmpty()
	if err != nil {
		return fmt.Errorf("failed to check if employee_department table is empty: %w", err)
	}

	if isEmpty {
		logger.Logger.Info("Employee department table is empty, triggering initial sync")
		if err := s.SyncEmployees(); err != nil {
			return fmt.Errorf("initial employee sync failed: %w", err)
		}
		logger.Logger.Info("Initial employee synchronization completed successfully")
	} else {
		logger.Logger.Info("Employee department table already has data, skipping initial sync")
	}

	return nil
}

// HREmployee represents employee data from HR system
type HREmployee struct {
	EmployeeNumber string `json:"employeeNumber"`
	Username       string `json:"username"`
	FullName       string `json:"fullName"`
	DeptName       string `json:"deptName"`
	Level          int    `json:"level"`
}

// HRDepartment represents department data from HR system
type HRDepartment struct {
	DeptName       string `json:"deptName"`
	ParentDeptName string `json:"parentDeptName"`
	Level          int    `json:"level"`
}

// HRResponse represents HR API response
type HRResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Message string          `json:"message"`
}

// SyncEmployees synchronizes employees from HR system
func (s *EmployeeSyncService) SyncEmployees() error {
	if !s.employeeSyncConf.Enabled {
		logger.Logger.Info("Employee sync is disabled")
		return nil
	}

	logger.Logger.Info("Starting employee synchronization")

	// Get employees from HR system
	employees, err := s.fetchEmployeesFromHR()
	if err != nil {
		return fmt.Errorf("failed to fetch employees: %w", err)
	}

	// Get departments from HR system
	departments, err := s.fetchDepartmentsFromHR()
	if err != nil {
		return fmt.Errorf("failed to fetch departments: %w", err)
	}

	// Build department hierarchy
	deptHierarchy := s.buildDepartmentHierarchy(departments)

	// Process employees
	updatedEmployees, err := s.processEmployees(employees, deptHierarchy)
	if err != nil {
		return fmt.Errorf("failed to process employees: %w", err)
	}

	// Update permissions for changed employees
	if err := s.updatePermissionsForChangedEmployees(updatedEmployees); err != nil {
		logger.Logger.Error("Failed to update permissions for changed employees", zap.Error(err))
		// Continue execution even if permission update fails
	}

	// Record audit
	auditDetails := map[string]interface{}{
		"total_employees":   len(employees),
		"updated_employees": len(updatedEmployees),
		"departments":       len(departments),
	}
	s.recordAudit(models.OperationEmployeeSync, "", "", auditDetails)

	logger.Logger.Info("Employee synchronization completed",
		zap.Int("total_employees", len(employees)),
		zap.Int("updated_employees", len(updatedEmployees)))

	return nil
}

// fetchEmployeesFromHR fetches employees from HR system
func (s *EmployeeSyncService) fetchEmployeesFromHR() ([]HREmployee, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequest("GET", s.employeeSyncConf.HrURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", s.employeeSyncConf.HrKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HR API returned status: %d", resp.StatusCode)
	}

	var hrResp HRResponse
	if err := json.NewDecoder(resp.Body).Decode(&hrResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !hrResp.Success {
		return nil, fmt.Errorf("HR API error: %s", hrResp.Message)
	}

	var employees []HREmployee
	if err := json.Unmarshal(hrResp.Data, &employees); err != nil {
		return nil, fmt.Errorf("failed to unmarshal employees: %w", err)
	}

	return employees, nil
}

// fetchDepartmentsFromHR fetches departments from HR system
func (s *EmployeeSyncService) fetchDepartmentsFromHR() ([]HRDepartment, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequest("GET", s.employeeSyncConf.DeptURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", s.employeeSyncConf.DeptKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Department API returned status: %d", resp.StatusCode)
	}

	var hrResp HRResponse
	if err := json.NewDecoder(resp.Body).Decode(&hrResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !hrResp.Success {
		return nil, fmt.Errorf("Department API error: %s", hrResp.Message)
	}

	var departments []HRDepartment
	if err := json.Unmarshal(hrResp.Data, &departments); err != nil {
		return nil, fmt.Errorf("failed to unmarshal departments: %w", err)
	}

	return departments, nil
}

// buildDepartmentHierarchy builds department hierarchy from flat list
func (s *EmployeeSyncService) buildDepartmentHierarchy(departments []HRDepartment) map[string][]string {
	hierarchy := make(map[string][]string)

	// Create a map of department to parent mapping
	deptToParent := make(map[string]string)
	for _, dept := range departments {
		if dept.ParentDeptName != "" {
			deptToParent[dept.DeptName] = dept.ParentDeptName
		}
	}

	// Build full hierarchy path for each department
	for _, dept := range departments {
		hierarchy[dept.DeptName] = s.buildFullPath(dept.DeptName, deptToParent)
	}

	return hierarchy
}

// buildFullPath builds full department path
func (s *EmployeeSyncService) buildFullPath(deptName string, deptToParent map[string]string) []string {
	var path []string
	current := deptName
	visited := make(map[string]bool)

	for current != "" {
		if visited[current] {
			// Circular reference detected, break
			break
		}
		visited[current] = true
		path = append([]string{current}, path...)
		current = deptToParent[current]
	}

	return path
}

// processEmployees processes employees and updates database
func (s *EmployeeSyncService) processEmployees(employees []HREmployee, deptHierarchy map[string][]string) ([]string, error) {
	// Get existing employees from database
	existingEmployees := make(map[string]*models.EmployeeDepartment)
	var dbEmployees []models.EmployeeDepartment
	if err := s.db.DB.Find(&dbEmployees).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch existing employees: %w", err)
	}

	for i := range dbEmployees {
		existingEmployees[dbEmployees[i].EmployeeNumber] = &dbEmployees[i]
	}

	var updatedEmployees []string

	// Process each employee
	for _, emp := range employees {
		// Get department hierarchy for this employee
		deptFullPath := deptHierarchy[emp.DeptName]
		if len(deptFullPath) == 0 {
			// If no hierarchy found, use the department name itself
			deptFullPath = []string{emp.DeptName}
		}

		// Check if employee exists
		if existing, exists := existingEmployees[emp.EmployeeNumber]; exists {
			// Check if data has changed
			oldDeptPath := existing.GetDeptFullLevelNamesAsSlice()
			isDeptChanged := !s.slicesEqual(oldDeptPath, deptFullPath)

			if existing.Username != emp.Username || isDeptChanged {
				// If department changed, clear user's personal whitelist first
				if isDeptChanged && s.permissionSvc != nil {
					logger.Logger.Info("Department change detected, clearing user personal whitelist",
						zap.String("employee_number", emp.EmployeeNumber),
						zap.Strings("old_dept", oldDeptPath),
						zap.Strings("new_dept", deptFullPath))

					if err := s.permissionSvc.ClearUserWhitelist(emp.EmployeeNumber); err != nil {
						logger.Logger.Error("Failed to clear user whitelist after department change",
							zap.String("employee_number", emp.EmployeeNumber),
							zap.Error(err))
						// Continue with the update even if whitelist clearing fails
					}
				}

				// Update existing employee
				existing.Username = emp.Username
				existing.SetDeptFullLevelNamesFromSlice(deptFullPath)
				existing.UpdateTime = time.Now()

				if err := s.db.DB.Save(existing).Error; err != nil {
					logger.Logger.Error("Failed to update employee",
						zap.String("employee_number", emp.EmployeeNumber),
						zap.Error(err))
					continue
				}

				updatedEmployees = append(updatedEmployees, emp.EmployeeNumber)
			}
		} else {
			// Create new employee
			newEmployee := &models.EmployeeDepartment{
				EmployeeNumber: emp.EmployeeNumber,
				Username:       emp.Username,
			}
			newEmployee.SetDeptFullLevelNamesFromSlice(deptFullPath)

			if err := s.db.DB.Create(newEmployee).Error; err != nil {
				logger.Logger.Error("Failed to create employee",
					zap.String("employee_number", emp.EmployeeNumber),
					zap.Error(err))
				continue
			}

			updatedEmployees = append(updatedEmployees, emp.EmployeeNumber)
		}
	}

	// Remove employees that are no longer in HR system
	currentEmployeeNumbers := make(map[string]bool)
	for _, emp := range employees {
		currentEmployeeNumbers[emp.EmployeeNumber] = true
	}

	for _, existing := range dbEmployees {
		if !currentEmployeeNumbers[existing.EmployeeNumber] {
			// Employee no longer exists in HR system, remove from database

			// First, clean up all permission-related data for this user
			if s.permissionSvc != nil {
				if err := s.permissionSvc.RemoveUserCompletely(existing.EmployeeNumber); err != nil {
					logger.Logger.Error("Failed to clean up user permissions during removal",
						zap.String("employee_number", existing.EmployeeNumber),
						zap.Error(err))
					// Continue with employee deletion even if permission cleanup fails
				}
			}

			// Then delete the employee record
			if err := s.db.DB.Delete(&existing).Error; err != nil {
				logger.Logger.Error("Failed to delete employee",
					zap.String("employee_number", existing.EmployeeNumber),
					zap.Error(err))
			} else {
				logger.Logger.Info("Deleted employee and cleaned up associated data",
					zap.String("employee_number", existing.EmployeeNumber))
			}
		}
	}

	return updatedEmployees, nil
}

// updatePermissionsForChangedEmployees updates permissions for employees whose data changed
func (s *EmployeeSyncService) updatePermissionsForChangedEmployees(employeeNumbers []string) error {
	if s.permissionSvc == nil {
		return nil
	}

	for _, empNum := range employeeNumbers {
		if err := s.permissionSvc.UpdateEmployeePermissions(empNum); err != nil {
			logger.Logger.Error("Failed to update permissions for employee",
				zap.String("employee_number", empNum),
				zap.Error(err))
		}
	}

	return nil
}

// slicesEqual checks if two string slices are equal
func (s *EmployeeSyncService) slicesEqual(a, b []string) bool {
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
func (s *EmployeeSyncService) recordAudit(operation, targetType, targetIdentifier string, details map[string]interface{}) {
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
