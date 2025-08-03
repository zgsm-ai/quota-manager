package main

import (
	"crypto/aes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// SetStarProjectsCall represents a SetGithubStarProjects call for testing
type SetStarProjectsCall struct {
	EmployeeNumber  string
	StarredProjects string
}

// PermissionCall represents a permission management call for testing
type PermissionCall struct {
	EmployeeNumber string
	Models         []string
	Operation      string // "set", "query", "delete"
}

// StarCheckCall represents a star check permission call for testing
type StarCheckCall struct {
	EmployeeNumber string
	Enabled        bool
	Operation      string // "set", "query"
}

// QuotaCheckCall represents a quota check permission call for testing
type QuotaCheckCall struct {
	EmployeeNumber string
	Enabled        bool
	Operation      string // "set", "query"
}

// MockQuotaStoreDeltaCall represents a delta call for testing
type MockQuotaStoreDeltaCall struct {
	EmployeeNumber string
	Delta          float64
}

// MockQuotaStoreUsedDeltaCall represents a used delta call for testing
type MockQuotaStoreUsedDeltaCall struct {
	EmployeeNumber string
	Delta          float64
}

// MockQuotaStore mock quota storage
type MockQuotaStore struct {
	data                 map[string]float64            // Total quota
	usedData             map[string]float64            // Used quota
	starData             map[string]bool               // GitHub star status
	permissionData       map[string][]string           // User permissions (employee_number -> models)
	starCheckData        map[string]bool               // Star check permissions (employee_number -> enabled)
	quotaCheckData       map[string]bool               // Quota check permissions (employee_number -> enabled)
	setStarProjectsCalls []SetStarProjectsCall         // Track SetGithubStarProjects calls
	permissionCalls      []PermissionCall              // Track permission management calls
	starCheckCalls       []StarCheckCall               // Track star check permission calls
	quotaCheckCalls      []QuotaCheckCall              // Track quota check permission calls
	CallCount            int                           // Track call count for SyncQuota
	deltaCalls           []MockQuotaStoreDeltaCall     // Track delta calls
	usedDeltaCalls       []MockQuotaStoreUsedDeltaCall // Track used delta calls
	mock.Mock                                          // For testify/mock functionality
}

func (m *MockQuotaStore) GetQuota(consumer string) float64 {
	if quota, exists := m.data[consumer]; exists {
		return quota
	}
	return 0
}

func (m *MockQuotaStore) SetQuota(consumer string, quota float64) {
	m.data[consumer] = quota
}

func (m *MockQuotaStore) DeltaQuota(consumer string, delta float64) float64 {
	m.data[consumer] += delta
	return m.data[consumer]
}

func (m *MockQuotaStore) GetUsed(consumer string) float64 {
	if used, exists := m.usedData[consumer]; exists {
		return used
	}
	return 0
}

func (m *MockQuotaStore) SetUsed(consumer string, used float64) {
	m.usedData[consumer] = used
}

func (m *MockQuotaStore) DeltaUsed(consumer string, delta float64) float64 {
	m.usedData[consumer] += delta
	return m.usedData[consumer]
}

func (m *MockQuotaStore) SetGithubStarProjects(employeeNumber string, starredProjects string) error {
	// For simplicity in mock, just store true if there are any starred projects
	m.starData[employeeNumber] = starredProjects != ""
	m.setStarProjectsCalls = append(m.setStarProjectsCalls, SetStarProjectsCall{
		EmployeeNumber:  employeeNumber,
		StarredProjects: starredProjects,
	})
	return nil
}

func (m *MockQuotaStore) GetSetStarProjectsCalls() []SetStarProjectsCall {
	return m.setStarProjectsCalls
}

func (m *MockQuotaStore) ClearSetStarProjectsCalls() {
	m.setStarProjectsCalls = []SetStarProjectsCall{}
}

// Permission management methods
func (m *MockQuotaStore) SetPermission(employeeNumber string, models []string) {
	m.permissionData[employeeNumber] = models
	m.permissionCalls = append(m.permissionCalls, PermissionCall{
		EmployeeNumber: employeeNumber,
		Models:         models,
		Operation:      "set",
	})
}

func (m *MockQuotaStore) GetPermission(employeeNumber string) []string {
	m.permissionCalls = append(m.permissionCalls, PermissionCall{
		EmployeeNumber: employeeNumber,
		Models:         nil,
		Operation:      "query",
	})
	if models, exists := m.permissionData[employeeNumber]; exists {
		return models
	}
	return []string{}
}

func (m *MockQuotaStore) DeletePermission(employeeNumber string) {
	delete(m.permissionData, employeeNumber)
	m.permissionCalls = append(m.permissionCalls, PermissionCall{
		EmployeeNumber: employeeNumber,
		Models:         nil,
		Operation:      "delete",
	})
}

func (m *MockQuotaStore) GetPermissionCalls() []PermissionCall {
	return m.permissionCalls
}

func (m *MockQuotaStore) ClearPermissionCalls() {
	m.permissionCalls = []PermissionCall{}
}

func (m *MockQuotaStore) GetAllPermissions() map[string][]string {
	return m.permissionData
}

func (m *MockQuotaStore) ClearAllPermissions() {
	m.permissionData = make(map[string][]string)
}

// Star check permission methods
func (m *MockQuotaStore) SetUserStarCheckPermission(employeeNumber string, enabled bool) error {
	if m.starCheckData == nil {
		m.starCheckData = make(map[string]bool)
	}
	m.starCheckData[employeeNumber] = enabled

	// Track the call
	call := StarCheckCall{
		EmployeeNumber: employeeNumber,
		Enabled:        enabled,
		Operation:      "set",
	}
	m.starCheckCalls = append(m.starCheckCalls, call)
	return nil
}

func (m *MockQuotaStore) GetStarCheckCalls() []StarCheckCall {
	return m.starCheckCalls
}

func (m *MockQuotaStore) ClearStarCheckCalls() {
	m.starCheckCalls = []StarCheckCall{}
}

// Quota check permission methods
func (m *MockQuotaStore) SetUserQuotaCheckPermission(employeeNumber string, enabled bool) error {
	if m.quotaCheckData == nil {
		m.quotaCheckData = make(map[string]bool)
	}
	m.quotaCheckData[employeeNumber] = enabled

	// Track the call
	call := QuotaCheckCall{
		EmployeeNumber: employeeNumber,
		Enabled:        enabled,
		Operation:      "set",
	}
	m.quotaCheckCalls = append(m.quotaCheckCalls, call)
	return nil
}

func (m *MockQuotaStore) GetQuotaCheckCalls() []QuotaCheckCall {
	return m.quotaCheckCalls
}

func (m *MockQuotaStore) ClearQuotaCheckCalls() {
	m.quotaCheckCalls = []QuotaCheckCall{}
}

// SyncQuota 同步配额到 AiGateway
func (m *MockQuotaStore) SyncQuota(userID string, quotaType string, amount float64) error {
	m.CallCount++

	// 记录调用
	args := m.Called(userID, quotaType, amount)

	// 如果设置了返回错误，则返回错误
	if err := args.Error(0); err != nil {
		return err
	}

	// 根据类型更新配额
	if quotaType == "total_quota" {
		m.deltaCalls = append(m.deltaCalls, MockQuotaStoreDeltaCall{
			EmployeeNumber: userID,
			Delta:          amount,
		})
		m.DeltaQuota(userID, amount)
	} else if quotaType == "used_quota" {
		m.usedDeltaCalls = append(m.usedDeltaCalls, MockQuotaStoreUsedDeltaCall{
			EmployeeNumber: userID,
			Delta:          amount,
		})
		m.DeltaUsed(userID, amount)
	}

	return nil
}

// GetDeltaCalls 获取 delta 调用记录
func (m *MockQuotaStore) GetDeltaCalls() []MockQuotaStoreDeltaCall {
	return m.deltaCalls
}

// GetUsedDeltaCalls 获取 used delta 调用记录
func (m *MockQuotaStore) GetUsedDeltaCalls() []MockQuotaStoreUsedDeltaCall {
	return m.usedDeltaCalls
}

// ClearDeltaCalls 清除 delta 调用记录
func (m *MockQuotaStore) ClearDeltaCalls() {
	m.deltaCalls = []MockQuotaStoreDeltaCall{}
}

// ClearUsedDeltaCalls 清除 used delta 调用记录
func (m *MockQuotaStore) ClearUsedDeltaCalls() {
	m.usedDeltaCalls = []MockQuotaStoreUsedDeltaCall{}
}

// ClearAllCalls 清除所有调用记录
func (m *MockQuotaStore) ClearAllCalls() {
	m.CallCount = 0
	m.ClearDeltaCalls()
	m.ClearUsedDeltaCalls()
	m.ClearSetStarProjectsCalls()
	m.ClearPermissionCalls()
	m.ClearStarCheckCalls()
	m.ClearQuotaCheckCalls()
}

// ClearData 清除所有数据
func (m *MockQuotaStore) ClearData() {
	m.data = make(map[string]float64)
	m.usedData = make(map[string]float64)
	m.starData = make(map[string]bool)
	m.permissionData = make(map[string][]string)
	m.starCheckData = make(map[string]bool)
	m.quotaCheckData = make(map[string]bool)
}

var mockStore = &MockQuotaStore{
	data:                 make(map[string]float64),
	usedData:             make(map[string]float64),
	starData:             make(map[string]bool),
	permissionData:       make(map[string][]string),
	starCheckData:        make(map[string]bool),
	quotaCheckData:       make(map[string]bool),
	setStarProjectsCalls: []SetStarProjectsCall{},
	permissionCalls:      []PermissionCall{},
	starCheckCalls:       []StarCheckCall{},
	quotaCheckCalls:      []QuotaCheckCall{},
	deltaCalls:           []MockQuotaStoreDeltaCall{},
	usedDeltaCalls:       []MockQuotaStoreUsedDeltaCall{},
}

// createMockServer create mock server
func createMockServer(shouldFail bool) *httptest.Server {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Middleware: validate Authorization
	authMiddleware := func(c *gin.Context) {
		auth := c.GetHeader("x-admin-key")
		if auth != "12345678" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization"})
			c.Abort()
			return
		}
		c.Next()
	}

	v1 := router.Group("/v1/chat/completions")
	v1.Use(authMiddleware)
	{
		// Add routes for new admin_path structure
		quota := v1.Group("/quota")
		{
			quota.POST("/refresh", func(c *gin.Context) {
				if shouldFail {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
					return
				}

				userID := c.PostForm("user_id")
				quota := c.PostForm("quota")

				if userID == "" || quota == "" {
					c.JSON(http.StatusBadRequest, gin.H{"error": "missing parameters"})
					return
				}

				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			quota.GET("", func(c *gin.Context) {
				if shouldFail {
					c.JSON(http.StatusServiceUnavailable, gin.H{
						"code":    "ai-gateway.error",
						"message": "redis error: connection failed",
						"success": false,
					})
					return
				}

				userID := c.Query("user_id")
				if userID == "" {
					c.JSON(http.StatusBadRequest, gin.H{
						"code":    "ai-gateway.invalid_params",
						"message": "user_id is required",
						"success": false,
					})
					return
				}

				quota := mockStore.GetQuota(userID)

				c.JSON(http.StatusOK, gin.H{
					"code":    "ai-gateway.queryquota",
					"message": "query quota successful",
					"success": true,
					"data": gin.H{
						"user_id": userID,
						"quota":   quota,
						"type":    "total_quota",
					},
				})
			})

			quota.POST("/delta", func(c *gin.Context) {
				if shouldFail {
					c.JSON(http.StatusServiceUnavailable, gin.H{
						"code":    "ai-gateway.error",
						"message": "redis error: connection failed",
						"success": false,
					})
					return
				}

				userID := c.PostForm("user_id")
				value := c.PostForm("value")

				if userID == "" || value == "" {
					c.JSON(http.StatusBadRequest, gin.H{
						"code":    "ai-gateway.invalid_params",
						"message": "user_id and value are required",
						"success": false,
					})
					return
				}

				// Simulate quota increase
				var delta float64
				if _, err := fmt.Sscanf(value, "%f", &delta); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"code":    "ai-gateway.invalid_params",
						"message": "value must be numeric",
						"success": false,
					})
					return
				}

				// Directly update the data instead of calling SyncQuota mock method
				mockStore.DeltaQuota(userID, delta)

				// Track the delta call manually
				call := MockQuotaStoreDeltaCall{
					EmployeeNumber: userID,
					Delta:          delta,
				}
				mockStore.deltaCalls = append(mockStore.deltaCalls, call)

				// Add logging to track delta call order
				fmt.Printf("[MOCK SERVER] POST /delta called - UserID: %s, Delta: %f, Total delta calls: %d\n",
					userID, delta, len(mockStore.deltaCalls))

				c.JSON(http.StatusOK, gin.H{
					"code":    "ai-gateway.deltaquota",
					"message": "delta quota successful",
					"success": true,
				})
			})

			quota.GET("/used", func(c *gin.Context) {
				if shouldFail {
					c.JSON(http.StatusServiceUnavailable, gin.H{
						"code":    "ai-gateway.error",
						"message": "redis error: connection failed",
						"success": false,
					})
					return
				}

				userID := c.Query("user_id")
				if userID == "" {
					c.JSON(http.StatusBadRequest, gin.H{
						"code":    "ai-gateway.invalid_params",
						"message": "user_id is required",
						"success": false,
					})
					return
				}

				used := mockStore.GetUsed(userID)

				c.JSON(http.StatusOK, gin.H{
					"code":    "ai-gateway.queryquota",
					"message": "query quota successful",
					"success": true,
					"data": gin.H{
						"user_id": userID,
						"quota":   used,
						"type":    "used_quota",
					},
				})
			})

			quota.POST("/used/delta", func(c *gin.Context) {
				if shouldFail {
					c.JSON(http.StatusServiceUnavailable, gin.H{
						"code":    "ai-gateway.error",
						"message": "redis error: connection failed",
						"success": false,
					})
					return
				}

				userID := c.PostForm("user_id")
				value := c.PostForm("value")

				if userID == "" || value == "" {
					c.JSON(http.StatusBadRequest, gin.H{
						"code":    "ai-gateway.invalid_params",
						"message": "user_id and value are required",
						"success": false,
					})
					return
				}

				// Parse and update used quota
				var delta float64
				if _, err := fmt.Sscanf(value, "%f", &delta); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"code":    "ai-gateway.invalid_params",
						"message": "value must be numeric",
						"success": false,
					})
					return
				}

				// Directly update the data instead of calling SyncQuota mock method
				mockStore.DeltaUsed(userID, delta)

				// Track the used delta call manually
				call := MockQuotaStoreUsedDeltaCall{
					EmployeeNumber: userID,
					Delta:          delta,
				}
				mockStore.usedDeltaCalls = append(mockStore.usedDeltaCalls, call)

				// Add logging to track used delta call order
				fmt.Printf("[MOCK SERVER] POST /used/delta called - UserID: %s, Delta: %f, Total used delta calls: %d\n",
					userID, delta, len(mockStore.usedDeltaCalls))

				c.JSON(http.StatusOK, gin.H{
					"code":    "ai-gateway.deltausedquota",
					"message": "delta used quota successful",
					"success": true,
				})
			})

			// GitHub star related APIs
			quota.GET("/star", func(c *gin.Context) {
				if shouldFail {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
					return
				}

				userID := c.Query("user_id")
				if userID == "" {
					c.JSON(http.StatusBadRequest, gin.H{"error": "missing user_id parameter"})
					return
				}

				// For testing, always return true for starred status
				c.JSON(http.StatusOK, gin.H{
					"star_value": true,
					"user_id":    userID,
				})
			})

			quota.POST("/star/projects/set", func(c *gin.Context) {
				if shouldFail {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
					return
				}

				employeeNumber := c.PostForm("employee_number")
				starredProjects := c.PostForm("starred_projects")

				if employeeNumber == "" {
					c.JSON(http.StatusBadRequest, gin.H{
						"code":    "ai-gateway.invalid_params",
						"message": "employee_number is required",
						"success": false,
					})
					return
				}

				err := mockStore.SetGithubStarProjects(employeeNumber, starredProjects)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{
						"code":    "ai-gateway.error",
						"message": err.Error(),
						"success": false,
					})
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"code":    "ai-gateway.setstarprojects",
					"message": "set GitHub star projects successful",
					"success": true,
					"data": gin.H{
						"employee_number":  employeeNumber,
						"starred_projects": starredProjects,
					},
				})
			})
		}
	}

	// Add Higress permission management API endpoints
	router.POST("/model-permission/set", func(c *gin.Context) {
		// Skip auth check for this endpoint as we're testing the permission management
		if shouldFail {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		employeeNumber := c.PostForm("employee_number")
		modelsParam := c.PostForm("models")

		if employeeNumber == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    "ai-quota.invalid_params",
				"message": "employee_number is required",
				"success": false,
			})
			return
		}

		// Parse models JSON
		var models []string
		if modelsParam != "" {
			if err := json.Unmarshal([]byte(modelsParam), &models); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"code":    "ai-quota.invalid_params",
					"message": "invalid models format",
					"success": false,
				})
				return
			}
		}

		// Store permission in mock store
		mockStore.SetPermission(employeeNumber, models)

		c.JSON(http.StatusOK, gin.H{
			"code":    "ai-quota.setpermission",
			"message": "set user permission successful",
			"success": true,
			"data": gin.H{
				"employee_number": employeeNumber,
				"models":          models,
			},
		})
	})

	router.GET("/model-permission/query", func(c *gin.Context) {
		// Skip auth check for this endpoint as we're testing the permission management
		if shouldFail {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		employeeNumber := c.Query("employee_number")
		if employeeNumber == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    "ai-quota.invalid_params",
				"message": "employee_number is required",
				"success": false,
			})
			return
		}

		// Get permission from mock store
		models := mockStore.GetPermission(employeeNumber)

		c.JSON(http.StatusOK, gin.H{
			"code":    "ai-quota.querypermission",
			"message": "query user permission successful",
			"success": true,
			"data": gin.H{
				"employee_number": employeeNumber,
				"models":          models,
			},
		})
	})

	router.DELETE("/model-permission/delete", func(c *gin.Context) {
		// Skip auth check for this endpoint as we're testing the permission management
		if shouldFail {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		employeeNumber := c.Query("employee_number")
		if employeeNumber == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    "ai-quota.invalid_params",
				"message": "employee_number is required",
				"success": false,
			})
			return
		}

		// Delete permission from mock store
		mockStore.DeletePermission(employeeNumber)

		c.JSON(http.StatusOK, gin.H{
			"code":    "ai-quota.deletepermission",
			"message": "delete user permission successful",
			"success": true,
			"data": gin.H{
				"employee_number": employeeNumber,
			},
		})
	})

	// Add star check permission endpoints
	router.POST("/check-star/set", func(c *gin.Context) {
		// Skip auth check for this endpoint as we're testing the star check permission management
		if shouldFail {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		employeeNumber := c.PostForm("employee_number")
		enabledParam := c.PostForm("enabled")

		if employeeNumber == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "employee_number is required",
			})
			return
		}

		enabled := enabledParam == "true"

		// Store star check setting in mock store
		if err := mockStore.SetUserStarCheckPermission(employeeNumber, enabled); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to set star check permission: " + err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Star check permission set successfully",
			"data": gin.H{
				"employee_number": employeeNumber,
				"enabled":         enabled,
			},
		})
	})

	// Add quota check permission endpoints
	router.POST("/check-quota/set", func(c *gin.Context) {
		// Skip auth check for this endpoint as we're testing the quota check permission management
		if shouldFail {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		employeeNumber := c.PostForm("employee_number")
		enabledParam := c.PostForm("enabled")

		if employeeNumber == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "employee_number is required",
			})
			return
		}

		enabled := enabledParam == "true"

		// Store quota check setting in mock store
		if err := mockStore.SetUserQuotaCheckPermission(employeeNumber, enabled); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to set quota check permission: " + err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Quota check permission set successfully",
			"data": gin.H{
				"employee_number": employeeNumber,
				"enabled":         enabled,
			},
		})
	})

	// Add HR API endpoints for employee sync testing (encrypted responses)
	// Employee API endpoint for testing
	router.GET("/api/test/employees", func(c *gin.Context) {
		if shouldFail {
			c.String(http.StatusInternalServerError, "Internal server error")
			return
		}

		// Use test encryption key for employees
		empKey := "TEST_EMP_KEY_32_BYTES_1234567890"

		xmlResponse, err := createEncryptedXMLResponse(mockHREmployees, empKey)
		if err != nil {
			c.String(http.StatusInternalServerError, "Encryption failed")
			return
		}

		c.Header("Content-Type", "application/xml; charset=utf-8")
		c.String(http.StatusOK, xmlResponse)
	})

	// Department API endpoint for testing
	router.GET("/api/test/departments", func(c *gin.Context) {
		if shouldFail {
			c.String(http.StatusInternalServerError, "Internal server error")
			return
		}

		// Use test encryption key for departments
		deptKey := "TEST_DEPT_KEY_32_BYTES_123456789"

		xmlResponse, err := createEncryptedXMLResponse(mockHRDepartments, deptKey)
		if err != nil {
			c.String(http.StatusInternalServerError, "Encryption failed")
			return
		}

		c.Header("Content-Type", "application/xml; charset=utf-8")
		c.String(http.StatusOK, xmlResponse)
	})

	return httptest.NewServer(router)
}

// Mock HR data for testing
var mockHREmployees []map[string]interface{}
var mockHRDepartments []map[string]interface{}

// Helper functions for converting between old and new data structures

// CreateMockEmployee creates employee data with new structure fields
func CreateMockEmployee(employeeNumber, username, email, mobile string, deptID int) map[string]interface{} {
	return map[string]interface{}{
		"badge": employeeNumber,            // New field name for employeeNumber
		"Name":  username,                  // New field name for username
		"DepID": fmt.Sprintf("%d", deptID), // Department ID as string
		"email": email,
		"TEL":   mobile,
	}
}

// CreateMockDepartment creates department data with new structure fields
func CreateMockDepartment(id, adminId int, name string, depGrade, status int) map[string]interface{} {
	return map[string]interface{}{
		"Id":               fmt.Sprintf("%d", id), // ID as string
		"AdminId":          adminId,               // Parent department ID
		"Name":             name,                  // Department name
		"DepGrade":         depGrade,              // Department grade/level
		"DepartmentStatus": status,                // Department status
	}
}

// AddMockEmployee adds an employee using the new data structure
func AddMockEmployee(employeeNumber, username, email, mobile string, deptID int) {
	employee := CreateMockEmployee(employeeNumber, username, email, mobile, deptID)
	mockHREmployees = append(mockHREmployees, employee)
}

// AddMockDepartment adds a department using the new data structure
func AddMockDepartment(id, adminId int, name string, depGrade, status int) {
	dept := CreateMockDepartment(id, adminId, name, depGrade, status)
	mockHRDepartments = append(mockHRDepartments, dept)
}

// RemoveMockEmployeeByNumber removes an employee by employee number (badge)
func RemoveMockEmployeeByNumber(employeeNumber string) {
	for i, emp := range mockHREmployees {
		if badge, ok := emp["badge"].(string); ok && badge == employeeNumber {
			mockHREmployees = append(mockHREmployees[:i], mockHREmployees[i+1:]...)
			break
		}
	}
}

// UpdateMockEmployeeDepartment updates an employee's department
func UpdateMockEmployeeDepartment(employeeNumber string, newDeptID int) {
	for i, emp := range mockHREmployees {
		if badge, ok := emp["badge"].(string); ok && badge == employeeNumber {
			mockHREmployees[i]["DepID"] = fmt.Sprintf("%d", newDeptID)
			break
		}
	}
}

// ClearMockData clears all mock data
func ClearMockData() {
	mockHREmployees = []map[string]interface{}{}
	mockHRDepartments = []map[string]interface{}{}
}

// SetupDefaultDepartmentHierarchy sets up a default department hierarchy for testing
func SetupDefaultDepartmentHierarchy() {
	// Clear existing data first
	mockHRDepartments = []map[string]interface{}{}

	// Create department hierarchy: Tech_Group -> R&D_Center -> [UX_Dept, QA_Dept, Testing_Dept] -> Teams
	AddMockDepartment(1, 0, "Tech_Group", 1, 1)             // Root department
	AddMockDepartment(2, 1, "R&D_Center", 2, 1)             // Second level
	AddMockDepartment(3, 2, "UX_Dept", 3, 1)                // Third level - UX
	AddMockDepartment(4, 3, "UX_Dept_Team1", 4, 1)          // Fourth level - UX Team
	AddMockDepartment(5, 2, "QA_Dept", 3, 1)                // Third level - QA
	AddMockDepartment(6, 5, "QA_Dept_Team1", 4, 1)          // Fourth level - QA Team
	AddMockDepartment(7, 2, "Testing_Dept", 3, 1)           // Third level - Testing
	AddMockDepartment(8, 7, "Testing_Dept_Team1", 4, 1)     // Fourth level - Testing Team
	AddMockDepartment(9, 2, "Operations_Dept", 3, 1)        // Third level - Operations
	AddMockDepartment(10, 9, "Operations_Dept_Team1", 4, 1) // Fourth level - Operations Team
}

// EncryptAES encrypts data using AES-ECB
func EncryptAES(key string, plaintext string) (string, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}

	// Add PKCS7 padding
	paddedPlaintext := addPKCS7Padding([]byte(plaintext), block.BlockSize())

	ciphertext := make([]byte, len(paddedPlaintext))
	for bs, be := 0, block.BlockSize(); bs < len(paddedPlaintext); bs, be = bs+block.BlockSize(), be+block.BlockSize() {
		block.Encrypt(ciphertext[bs:be], paddedPlaintext[bs:be])
	}

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// addPKCS7Padding adds PKCS7 padding to data
func addPKCS7Padding(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := make([]byte, padding)
	for i := range padText {
		padText[i] = byte(padding)
	}
	return append(data, padText...)
}

// createEncryptedXMLResponse creates an encrypted XML response
func createEncryptedXMLResponse(data interface{}, key string) (string, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	encryptedData, err := EncryptAES(key, string(jsonData))
	if err != nil {
		return "", err
	}

	// Wrap in XML format
	xmlResponse := fmt.Sprintf("<?xml version=\"1.0\" encoding=\"utf-8\"?><string xmlns=\"http://tempuri.org/\">%s</string>", encryptedData)
	return xmlResponse, nil
}

// Reset 重置 MockQuotaStore 的状态
func (m *MockQuotaStore) Reset() {
	m.ClearData()
	m.ClearAllCalls()
}

// expectedExpireQuotas 存储期望的过期配额调用
var expectedExpireQuotas []string

// ExpectExpireQuotas 设置期望的过期配额调用
func (m *MockQuotaStore) ExpectExpireQuotas(userID string) {
	expectedExpireQuotas = append(expectedExpireQuotas, userID)
}

// ClearExpectedExpireQuotas 清除期望的过期配额调用
func (m *MockQuotaStore) ClearExpectedExpireQuotas() {
	expectedExpireQuotas = []string{}
}

// VerifyQuotaExpired 验证配额过期是否被正确调用
func (m *MockQuotaStore) VerifyQuotaExpired(userID string) bool {
	// 检查是否有期望的过期配额调用
	for _, expectedUserID := range expectedExpireQuotas {
		if expectedUserID == userID {
			// 验证通过，清除已验证的期望
			m.ClearExpectedExpireQuotas()
			return true
		}
	}
	return false
}

// generateUUID 生成 UUID 字符串
func generateUUID() string {
	return uuid.NewString()
}
