package services

import (
	"crypto/aes"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"quota-manager/internal/config"
	"quota-manager/internal/database"
	"quota-manager/internal/models"
	"quota-manager/pkg/logger"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// EmployeeSyncService handles employee synchronization
type EmployeeSyncService struct {
	db                     *database.DB
	employeeSyncConf       *config.EmployeeSyncConfig
	permissionSvc          *PermissionService
	starCheckPermissionSvc *StarCheckPermissionService
	cron                   *cron.Cron
}

// NewEmployeeSyncService creates a new employee sync service
func NewEmployeeSyncService(
	db *database.DB,
	employeeSyncConf *config.EmployeeSyncConfig,
	permissionSvc *PermissionService,
	starCheckPermissionSvc *StarCheckPermissionService,
) *EmployeeSyncService {
	return &EmployeeSyncService{
		db:                     db,
		employeeSyncConf:       employeeSyncConf,
		permissionSvc:          permissionSvc,
		starCheckPermissionSvc: starCheckPermissionSvc,
		cron:                   cron.New(cron.WithSeconds()),
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

// HREmployee represents employee data from HR system (matching reference code structure)
type HREmployee struct {
	EmployeeNumber string `json:"badge"`
	Username       string `json:"Name"`
	DeptID         int    `json:"DepID,string"`
	Email          string `json:"email"`
	Mobile         string `json:"TEL"`

	// Fields populated by our program.
	FullName           string   `json:"-"`
	UserID             string   `json:"-"`
	DeptName           string   `json:"-"`
	DeptFullLevelIds   []int    `json:"-"`
	DeptFullLevelNames []string `json:"-"`
}

// HRDepartment represents department data from HR system (matching reference code structure)
type HRDepartment struct {
	ID       int    `json:"Id,string"`
	AdminId  int    `json:"AdminId"`
	Name     string `json:"Name"`
	DepGrade int    `json:"DepGrade"`
	Status   int    `json:"DepartmentStatus"`

	// Fields populated by our program.
	Level          int             `json:"-"`
	FullLevelIds   []int           `json:"-"`
	FullLevelNames []string        `json:"-"`
	Children       []*HRDepartment `json:"-"`
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
	var employees []HREmployee
	err := s.fetchAndDeserialize(s.employeeSyncConf.HrURL, s.employeeSyncConf.HrKey, &employees)
	if err != nil {
		return nil, err
	}
	return employees, nil
}

// fetchDepartmentsFromHR fetches departments from HR system
func (s *EmployeeSyncService) fetchDepartmentsFromHR() ([]*HRDepartment, error) {
	var departments []*HRDepartment
	err := s.fetchAndDeserialize(s.employeeSyncConf.DeptURL, s.employeeSyncConf.DeptKey, &departments)
	if err != nil {
		return nil, err
	}
	return departments, nil
}

// buildDepartmentHierarchy builds department hierarchy from flat list (matching reference code logic)
func (s *EmployeeSyncService) buildDepartmentHierarchy(depts []*HRDepartment) []*HRDepartment {
	nodeMap := make(map[int]*HRDepartment)
	for _, d := range depts {
		nodeMap[d.ID] = d
	}

	for _, node := range nodeMap {
		level := 1
		fullIds := []int{node.ID}
		fullNames := []string{node.Name}
		parentId := node.AdminId

		for parentId != 0 {
			parent, ok := nodeMap[parentId]
			if !ok {
				break
			}
			level++
			fullIds = append(fullIds, parent.ID)
			fullNames = append(fullNames, parent.Name)
			parentId = parent.AdminId
		}
		node.Level = level
		s.reverseInts(fullIds)
		s.reverseStrings(fullNames)
		node.FullLevelIds = fullIds
		node.FullLevelNames = fullNames
	}

	var rootNodes []*HRDepartment
	for _, node := range nodeMap {
		if node.AdminId == 0 || nodeMap[node.AdminId] == nil {
			rootNodes = append(rootNodes, node)
		} else {
			parent := nodeMap[node.AdminId]
			if parent.Children == nil {
				parent.Children = make([]*HRDepartment, 0)
			}
			parent.Children = append(parent.Children, node)
		}
	}

	return rootNodes
}

// flattenDepartmentTree flattens department tree into a map for quick lookup
func (s *EmployeeSyncService) flattenDepartmentTree(deptTree []*HRDepartment) map[int]*HRDepartment {
	flatMap := make(map[int]*HRDepartment)
	var dfs func(depts []*HRDepartment)
	dfs = func(depts []*HRDepartment) {
		if len(depts) == 0 {
			return
		}
		for _, dept := range depts {
			flatMap[dept.ID] = dept
			dfs(dept.Children)
		}
	}
	dfs(deptTree)
	return flatMap
}

// processEmployees processes employees and updates database
func (s *EmployeeSyncService) processEmployees(employees []HREmployee, deptHierarchy []*HRDepartment) ([]string, error) {
	// Create a flat map of department ID to department for quick lookup
	deptMap := s.flattenDepartmentTree(deptHierarchy)

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
		var deptFullPath []string
		if dept, ok := deptMap[emp.DeptID]; ok {
			deptFullPath = dept.FullLevelNames
		} else {
			// If department not found, skip this employee
			logger.Logger.Warn("Department not found for employee",
				zap.String("employee_number", emp.EmployeeNumber),
				zap.Int("dept_id", emp.DeptID))
			continue
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
	// Update model permissions
	if s.permissionSvc != nil {
		for _, empNum := range employeeNumbers {
			if err := s.permissionSvc.UpdateEmployeePermissions(empNum); err != nil {
				logger.Logger.Error("Failed to update model permissions for employee",
					zap.String("employee_number", empNum),
					zap.Error(err))
			}
		}
	}

	// Update star check permissions
	if s.starCheckPermissionSvc != nil {
		for _, empNum := range employeeNumbers {
			if err := s.starCheckPermissionSvc.UpdateEmployeeStarCheckPermissions(empNum); err != nil {
				logger.Logger.Error("Failed to update star check permissions for employee",
					zap.String("employee_number", empNum),
					zap.Error(err))
			}
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

// reverseInts reverses an integer slice in place
func (s *EmployeeSyncService) reverseInts(slice []int) {
	for i, j := 0, len(slice)-1; i < j; i, j = i+1, j-1 {
		slice[i], slice[j] = slice[j], slice[i]
	}
}

// reverseStrings reverses a string slice in place
func (s *EmployeeSyncService) reverseStrings(slice []string) {
	for i, j := 0, len(slice)-1; i < j; i, j = i+1, j-1 {
		slice[i], slice[j] = slice[j], slice[i]
	}
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

// fetchAndDeserialize fetches data from URL, decrypts and deserializes it
func (s *EmployeeSyncService) fetchAndDeserialize(url, key string, target interface{}) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("http get request to %s failed: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http request to %s returned non-200 status: %s", url, resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body from %s: %w", url, err)
	}

	encryptedString := strings.TrimSpace(string(body))
	if encryptedString == "" {
		return fmt.Errorf("received empty response body from %s", url)
	}

	// Check if response is XML and extract content
	if strings.HasPrefix(encryptedString, "<?xml") {
		encryptedString, err = s.parseXMLContent(encryptedString)
		if err != nil {
			return fmt.Errorf("failed to parse xml content from %s: %w", url, err)
		}
	}

	// Decrypt the response
	decryptedJSON, err := s.DecryptAES(key, encryptedString)
	if err != nil {
		return fmt.Errorf("failed to decrypt response from %s: %w", url, err)
	}

	return json.Unmarshal([]byte(decryptedJSON), target)
}

// parseXMLContent extracts content from XML response
func (s *EmployeeSyncService) parseXMLContent(xmlString string) (string, error) {
	var result struct {
		Content string `xml:",chardata"`
	}
	if err := xml.Unmarshal([]byte(xmlString), &result); err != nil {
		return "", err
	}
	return result.Content, nil
}

// DecryptAES decrypts AES encrypted data
func (s *EmployeeSyncService) DecryptAES(key string, encryptedBase64 string) (string, error) {
	encryptedData, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		return "", fmt.Errorf("base64 decode failed: %w", err)
	}

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", fmt.Errorf("failed to create new cipher: %w", err)
	}

	blockSize := block.BlockSize()
	if len(encryptedData)%blockSize != 0 {
		return "", errors.New("encrypted data is not a multiple of the block size")
	}

	decryptedData := make([]byte, len(encryptedData))
	for bs, be := 0, blockSize; bs < len(encryptedData); bs, be = bs+blockSize, be+blockSize {
		block.Decrypt(decryptedData[bs:be], encryptedData[bs:be])
	}

	unpaddedData, err := s.unpadPKCS7(decryptedData, blockSize)
	if err != nil {
		return "", fmt.Errorf("failed to unpad data: %w", err)
	}

	return string(unpaddedData), nil
}

// unpadPKCS7 removes PKCS7 padding
func (s *EmployeeSyncService) unpadPKCS7(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("pkcs7: data is empty")
	}
	if len(data)%blockSize != 0 {
		return nil, errors.New("pkcs7: data is not a multiple of the block size")
	}

	paddingLen := int(data[len(data)-1])
	if paddingLen > blockSize || paddingLen == 0 {
		return nil, errors.New("pkcs7: invalid padding")
	}

	for i := 0; i < paddingLen; i++ {
		if data[len(data)-paddingLen+i] != byte(paddingLen) {
			return nil, errors.New("pkcs7: invalid padding")
		}
	}

	return data[:len(data)-paddingLen], nil
}
