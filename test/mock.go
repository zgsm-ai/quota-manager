package main

import (
	"crypto/aes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
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

// MockQuotaStore mock quota storage
type MockQuotaStore struct {
	data                 map[string]int        // Total quota
	usedData             map[string]int        // Used quota
	starData             map[string]bool       // GitHub star status
	permissionData       map[string][]string   // User permissions (employee_number -> models)
	setStarProjectsCalls []SetStarProjectsCall // Track SetGithubStarProjects calls
	permissionCalls      []PermissionCall      // Track permission management calls
}

func (m *MockQuotaStore) GetQuota(consumer string) int {
	if quota, exists := m.data[consumer]; exists {
		return quota
	}
	return 0
}

func (m *MockQuotaStore) SetQuota(consumer string, quota int) {
	m.data[consumer] = quota
}

func (m *MockQuotaStore) DeltaQuota(consumer string, delta int) int {
	m.data[consumer] += delta
	return m.data[consumer]
}

func (m *MockQuotaStore) GetUsed(consumer string) int {
	if used, exists := m.usedData[consumer]; exists {
		return used
	}
	return 0
}

func (m *MockQuotaStore) SetUsed(consumer string, used int) {
	m.usedData[consumer] = used
}

func (m *MockQuotaStore) DeltaUsed(consumer string, delta int) int {
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

var mockStore = &MockQuotaStore{
	data:                 make(map[string]int),
	usedData:             make(map[string]int),
	starData:             make(map[string]bool),
	permissionData:       make(map[string][]string),
	setStarProjectsCalls: []SetStarProjectsCall{},
	permissionCalls:      []PermissionCall{},
}

// createMockServer create mock server
func createMockServer(shouldFail bool) *httptest.Server {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Middleware: validate Authorization
	authMiddleware := func(c *gin.Context) {
		auth := c.GetHeader("X-Auth-Key")
		if auth != "credential3" {
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
				var delta int
				if _, err := fmt.Sscanf(value, "%d", &delta); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"code":    "ai-gateway.invalid_params",
						"message": "value must be integer",
						"success": false,
					})
					return
				}

				mockStore.DeltaQuota(userID, delta)

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
				var delta int
				if _, err := fmt.Sscanf(value, "%d", &delta); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{
						"code":    "ai-gateway.invalid_params",
						"message": "value must be integer",
						"success": false,
					})
					return
				}

				mockStore.DeltaUsed(userID, delta)

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
					c.JSON(http.StatusBadRequest, gin.H{"error": "missing employee_number parameter"})
					return
				}

				err := mockStore.SetGithubStarProjects(employeeNumber, starredProjects)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}

				c.JSON(http.StatusOK, gin.H{
					"message":          "success",
					"employee_number":  employeeNumber,
					"starred_projects": starredProjects,
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
