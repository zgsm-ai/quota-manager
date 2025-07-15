package main

import (
	"crypto/aes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// HREmployee represents employee data structure
type HREmployee struct {
	EmployeeNumber string `json:"badge"`
	Username       string `json:"Name"`
	DeptID         int    `json:"DepID,string"`
	Email          string `json:"email"`
	Mobile         string `json:"TEL"`
}

// HRDepartment represents department data structure
type HRDepartment struct {
	ID       int    `json:"Id,string"`
	AdminId  int    `json:"AdminId"`
	Name     string `json:"Name"`
	DepGrade int    `json:"DepGrade"`
	Status   int    `json:"DepartmentStatus"`
}

// EmployeeSyncMockServer provides mock data for employee sync
type EmployeeSyncMockServer struct {
	employees   []HREmployee
	departments []HRDepartment
}

// NewEmployeeSyncMockServer creates a new mock server instance
func NewEmployeeSyncMockServer() *EmployeeSyncMockServer {
	server := &EmployeeSyncMockServer{}
	server.initializeTestData()
	return server
}

// initializeTestData generates comprehensive test data
func (s *EmployeeSyncMockServer) initializeTestData() {
	// Generate department hierarchy (>50 departments)
	s.departments = s.generateDepartmentHierarchy()

	// Generate employee data (>200 employees)
	s.employees = s.generateEmployeeData()
}

// generateDepartmentHierarchy creates a multi-level department structure
func (s *EmployeeSyncMockServer) generateDepartmentHierarchy() []HRDepartment {
	departments := []HRDepartment{
		// Root company (Level 1)
		{ID: 1, AdminId: 0, Name: "深信服科技", DepGrade: 1, Status: 1},

		// Major divisions (Level 2)
		{ID: 2, AdminId: 1, Name: "研发中心", DepGrade: 2, Status: 1},
		{ID: 3, AdminId: 1, Name: "产品中心", DepGrade: 2, Status: 1},
		{ID: 4, AdminId: 1, Name: "销售中心", DepGrade: 2, Status: 1},
		{ID: 5, AdminId: 1, Name: "市场中心", DepGrade: 2, Status: 1},
		{ID: 6, AdminId: 1, Name: "人力资源部", DepGrade: 2, Status: 1},
		{ID: 7, AdminId: 1, Name: "财务部", DepGrade: 2, Status: 1},
		{ID: 8, AdminId: 1, Name: "法务部", DepGrade: 2, Status: 1},
		{ID: 9, AdminId: 1, Name: "运营中心", DepGrade: 2, Status: 1},
		{ID: 10, AdminId: 1, Name: "质量保证部", DepGrade: 2, Status: 1},

		// R&D departments (Level 3)
		{ID: 11, AdminId: 2, Name: "AI研发部", DepGrade: 3, Status: 1},
		{ID: 12, AdminId: 2, Name: "安全研发部", DepGrade: 3, Status: 1},
		{ID: 13, AdminId: 2, Name: "网络研发部", DepGrade: 3, Status: 1},
		{ID: 14, AdminId: 2, Name: "云计算研发部", DepGrade: 3, Status: 1},
		{ID: 15, AdminId: 2, Name: "终端研发部", DepGrade: 3, Status: 1},
		{ID: 16, AdminId: 2, Name: "基础架构部", DepGrade: 3, Status: 1},
		{ID: 17, AdminId: 2, Name: "测试部", DepGrade: 3, Status: 1},
		{ID: 18, AdminId: 2, Name: "运维部", DepGrade: 3, Status: 1},

		// Product departments (Level 3)
		{ID: 19, AdminId: 3, Name: "产品策划部", DepGrade: 3, Status: 1},
		{ID: 20, AdminId: 3, Name: "用户体验部", DepGrade: 3, Status: 1},
		{ID: 21, AdminId: 3, Name: "产品运营部", DepGrade: 3, Status: 1},
		{ID: 22, AdminId: 3, Name: "数据分析部", DepGrade: 3, Status: 1},

		// Sales departments (Level 3)
		{ID: 23, AdminId: 4, Name: "华北销售部", DepGrade: 3, Status: 1},
		{ID: 24, AdminId: 4, Name: "华南销售部", DepGrade: 3, Status: 1},
		{ID: 25, AdminId: 4, Name: "华东销售部", DepGrade: 3, Status: 1},
		{ID: 26, AdminId: 4, Name: "华西销售部", DepGrade: 3, Status: 1},
		{ID: 27, AdminId: 4, Name: "海外销售部", DepGrade: 3, Status: 1},
		{ID: 28, AdminId: 4, Name: "渠道销售部", DepGrade: 3, Status: 1},
		{ID: 29, AdminId: 4, Name: "大客户销售部", DepGrade: 3, Status: 1},
		{ID: 30, AdminId: 4, Name: "售前技术部", DepGrade: 3, Status: 1},

		// Marketing departments (Level 3)
		{ID: 31, AdminId: 5, Name: "品牌推广部", DepGrade: 3, Status: 1},
		{ID: 32, AdminId: 5, Name: "市场策划部", DepGrade: 3, Status: 1},
		{ID: 33, AdminId: 5, Name: "公关部", DepGrade: 3, Status: 1},
		{ID: 34, AdminId: 5, Name: "商务拓展部", DepGrade: 3, Status: 1},

		// AI R&D teams (Level 4)
		{ID: 35, AdminId: 11, Name: "机器学习团队", DepGrade: 4, Status: 1},
		{ID: 36, AdminId: 11, Name: "深度学习团队", DepGrade: 4, Status: 1},
		{ID: 37, AdminId: 11, Name: "自然语言处理团队", DepGrade: 4, Status: 1},
		{ID: 38, AdminId: 11, Name: "计算机视觉团队", DepGrade: 4, Status: 1},
		{ID: 39, AdminId: 11, Name: "AI平台团队", DepGrade: 4, Status: 1},
		{ID: 40, AdminId: 11, Name: "算法优化团队", DepGrade: 4, Status: 1},

		// Security R&D teams (Level 4)
		{ID: 41, AdminId: 12, Name: "威胁检测团队", DepGrade: 4, Status: 1},
		{ID: 42, AdminId: 12, Name: "漏洞研究团队", DepGrade: 4, Status: 1},
		{ID: 43, AdminId: 12, Name: "安全防护团队", DepGrade: 4, Status: 1},
		{ID: 44, AdminId: 12, Name: "安全分析团队", DepGrade: 4, Status: 1},

		// Network R&D teams (Level 4)
		{ID: 45, AdminId: 13, Name: "网络协议团队", DepGrade: 4, Status: 1},
		{ID: 46, AdminId: 13, Name: "网络优化团队", DepGrade: 4, Status: 1},
		{ID: 47, AdminId: 13, Name: "SD-WAN团队", DepGrade: 4, Status: 1},
		{ID: 48, AdminId: 13, Name: "网络安全团队", DepGrade: 4, Status: 1},

		// Cloud R&D teams (Level 4)
		{ID: 49, AdminId: 14, Name: "云原生团队", DepGrade: 4, Status: 1},
		{ID: 50, AdminId: 14, Name: "微服务团队", DepGrade: 4, Status: 1},
		{ID: 51, AdminId: 14, Name: "容器化团队", DepGrade: 4, Status: 1},
		{ID: 52, AdminId: 14, Name: "DevOps团队", DepGrade: 4, Status: 1},

		// Terminal R&D teams (Level 4)
		{ID: 53, AdminId: 15, Name: "终端安全团队", DepGrade: 4, Status: 1},
		{ID: 54, AdminId: 15, Name: "终端管理团队", DepGrade: 4, Status: 1},
		{ID: 55, AdminId: 15, Name: "移动安全团队", DepGrade: 4, Status: 1},

		// UX teams (Level 4)
		{ID: 56, AdminId: 20, Name: "前端开发团队", DepGrade: 4, Status: 1},
		{ID: 57, AdminId: 20, Name: "UI设计团队", DepGrade: 4, Status: 1},
		{ID: 58, AdminId: 20, Name: "交互设计团队", DepGrade: 4, Status: 1},

		// Sales teams (Level 4)
		{ID: 59, AdminId: 23, Name: "北京销售团队", DepGrade: 4, Status: 1},
		{ID: 60, AdminId: 23, Name: "天津销售团队", DepGrade: 4, Status: 1},
		{ID: 61, AdminId: 24, Name: "深圳销售团队", DepGrade: 4, Status: 1},
		{ID: 62, AdminId: 24, Name: "广州销售团队", DepGrade: 4, Status: 1},
		{ID: 63, AdminId: 25, Name: "上海销售团队", DepGrade: 4, Status: 1},
		{ID: 64, AdminId: 25, Name: "杭州销售团队", DepGrade: 4, Status: 1},
		{ID: 65, AdminId: 26, Name: "成都销售团队", DepGrade: 4, Status: 1},
		{ID: 66, AdminId: 26, Name: "西安销售团队", DepGrade: 4, Status: 1},
	}

	return departments
}

// generateEmployeeData creates employee data distributed across departments
func (s *EmployeeSyncMockServer) generateEmployeeData() []HREmployee {
	employees := []HREmployee{}

	// Employee counter
	empNum := 1000

	// Generate employees for each department
	deptEmployeeCounts := map[int]int{
		// AI R&D teams (larger teams)
		35: 15, 36: 12, 37: 10, 38: 8, 39: 6, 40: 5,
		// Security R&D teams
		41: 8, 42: 6, 43: 10, 44: 7,
		// Network R&D teams
		45: 6, 46: 8, 47: 10, 48: 9,
		// Cloud R&D teams
		49: 12, 50: 10, 51: 8, 52: 9,
		// Terminal R&D teams
		53: 7, 54: 6, 55: 5,
		// UX teams
		56: 8, 57: 6, 58: 5,
		// Sales teams
		59: 12, 60: 8, 61: 15, 62: 10, 63: 14, 64: 9, 65: 11, 66: 7,
		// Other departments
		16: 8, 17: 12, 18: 6, 19: 5, 20: 4, 21: 6, 22: 8,
		23: 3, 24: 3, 25: 3, 26: 3, 27: 5, 28: 8, 29: 6, 30: 10,
		31: 4, 32: 5, 33: 3, 34: 4,
		6: 8, 7: 6, 8: 3, 9: 5, 10: 4,
	}

	// Employee name prefixes for variety
	namePatterns := []string{
		"张", "王", "李", "刘", "陈", "杨", "黄", "赵", "周", "吴",
		"徐", "孙", "朱", "马", "胡", "郭", "林", "何", "高", "梁",
		"郑", "罗", "宋", "谢", "唐", "韩", "曹", "许", "邓", "萧",
	}

	nameSuffixes := []string{
		"伟", "强", "磊", "军", "洋", "勇", "艳", "娜", "静", "敏",
		"杰", "涛", "明", "超", "鹏", "华", "亮", "刚", "平", "辉",
		"丽", "红", "燕", "雪", "霞", "玲", "芳", "梅", "兰", "莹",
	}

	for deptID, count := range deptEmployeeCounts {
		for i := 0; i < count; i++ {
			// Generate employee number
			empNumStr := fmt.Sprintf("%06d", empNum)

			// Generate random name
			firstName := namePatterns[empNum%len(namePatterns)]
			lastName := nameSuffixes[(empNum*3+i)%len(nameSuffixes)]
			fullName := firstName + lastName

			// Generate username (pinyin-like)
			username := s.generateUsername(fullName, empNum)

			// Generate email
			email := fmt.Sprintf("%s@sangfor.com", username)

			// Generate mobile
			mobile := fmt.Sprintf("138%08d", empNum)

			employee := HREmployee{
				EmployeeNumber: empNumStr,
				Username:       fullName,
				DeptID:         deptID,
				Email:          email,
				Mobile:         mobile,
			}

			employees = append(employees, employee)
			empNum++
		}
	}

	return employees
}

// generateUsername creates a username from Chinese name
func (s *EmployeeSyncMockServer) generateUsername(name string, seed int) string {
	// Simple mapping for demo purposes
	nameMap := map[string]string{
		"张": "zhang", "王": "wang", "李": "li", "刘": "liu", "陈": "chen",
		"杨": "yang", "黄": "huang", "赵": "zhao", "周": "zhou", "吴": "wu",
		"徐": "xu", "孙": "sun", "朱": "zhu", "马": "ma", "胡": "hu",
		"郭": "guo", "林": "lin", "何": "he", "高": "gao", "梁": "liang",
		"郑": "zheng", "罗": "luo", "宋": "song", "谢": "xie", "唐": "tang",
		"韩": "han", "曹": "cao", "许": "xu", "邓": "deng", "萧": "xiao",
		"伟": "wei", "强": "qiang", "磊": "lei", "军": "jun", "洋": "yang",
		"勇": "yong", "艳": "yan", "娜": "na", "静": "jing", "敏": "min",
		"杰": "jie", "涛": "tao", "明": "ming", "超": "chao", "鹏": "peng",
		"华": "hua", "亮": "liang", "刚": "gang", "平": "ping", "辉": "hui",
		"丽": "li", "红": "hong", "燕": "yan", "雪": "xue", "霞": "xia",
		"玲": "ling", "芳": "fang", "梅": "mei", "兰": "lan", "莹": "ying",
	}

	result := ""
	for _, char := range name {
		if pinyin, exists := nameMap[string(char)]; exists {
			result += pinyin
		} else {
			result += "x"
		}
	}

	return fmt.Sprintf("%s_%d", result, seed%1000)
}

// encryptAES encrypts data using AES
func (s *EmployeeSyncMockServer) encryptAES(key, plaintext string) (string, error) {
	// Use key directly like quota-manager does
	keyBytes := []byte(key)

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", err
	}

	// Add PKCS7 padding
	plaintextBytes := []byte(plaintext)
	padding := aes.BlockSize - len(plaintextBytes)%aes.BlockSize
	padtext := make([]byte, padding)
	for i := range padtext {
		padtext[i] = byte(padding)
	}
	plaintextBytes = append(plaintextBytes, padtext...)

	// Encrypt
	ciphertext := make([]byte, len(plaintextBytes))
	for bs, be := 0, block.BlockSize(); bs < len(plaintextBytes); bs, be = bs+block.BlockSize(), be+block.BlockSize() {
		block.Encrypt(ciphertext[bs:be], plaintextBytes[bs:be])
	}

	// Return base64 encoded string
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// getEmployees handles employee data request
func (s *EmployeeSyncMockServer) getEmployees(c *gin.Context) {
	// Get HR key from config
	hrKey := "test-hr-key-for-aes-256-gcm-32b!"

	// Convert employees to JSON
	jsonData, err := json.Marshal(s.employees)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to marshal employee data")
		return
	}

	// Encrypt the JSON data
	encrypted, err := s.encryptAES(hrKey, string(jsonData))
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to encrypt employee data")
		return
	}

	// Return encrypted data
	c.String(http.StatusOK, encrypted)
}

// getDepartments handles department data request
func (s *EmployeeSyncMockServer) getDepartments(c *gin.Context) {
	// Get department key from config
	deptKey := "test-dept-key-for-aes-256-g-32b!"

	// Convert departments to JSON
	jsonData, err := json.Marshal(s.departments)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to marshal department data")
		return
	}

	// Encrypt the JSON data
	encrypted, err := s.encryptAES(deptKey, string(jsonData))
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to encrypt department data")
		return
	}

	// Return encrypted data
	c.String(http.StatusOK, encrypted)
}

// getStatus provides server status information
func (s *EmployeeSyncMockServer) getStatus(c *gin.Context) {
	status := map[string]interface{}{
		"service":     "employee-sync-mock",
		"status":      "running",
		"departments": len(s.departments),
		"employees":   len(s.employees),
	}

	c.JSON(http.StatusOK, status)
}

func main() {
	// Initialize mock server
	server := NewEmployeeSyncMockServer()

	// Set Gin to release mode
	gin.SetMode(gin.ReleaseMode)

	// Create Gin router
	router := gin.Default()

	// Add routes
	router.GET("/api/hr/employees", server.getEmployees)
	router.GET("/api/hr/departments", server.getDepartments)
	router.GET("/status", server.getStatus)

	// Add health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// Start server
	port := "8098"
	log.Printf("Employee Sync Mock Server starting on port %s", port)
	log.Printf("Endpoints available:")
	log.Printf("  GET /api/hr/employees - Returns encrypted employee data")
	log.Printf("  GET /api/hr/departments - Returns encrypted department data")
	log.Printf("  GET /status - Returns server status")
	log.Printf("  GET /health - Health check")
	log.Printf("Generated %d departments and %d employees", len(server.departments), len(server.employees))

	if err := http.ListenAndServe(":"+port, router); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
