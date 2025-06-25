package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
)

// ResponseData defines the standard API response format matching the AI Gateway documentation
type ResponseData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
}

// NewSuccessResponse creates a success response
func NewSuccessResponse(code, message string, data any) ResponseData {
	return ResponseData{
		Code:    code,
		Message: message,
		Success: true,
		Data:    data,
	}
}

// NewErrorResponse creates an error response
func NewErrorResponse(code, message string) ResponseData {
	return ResponseData{
		Code:    code,
		Message: message,
		Success: false,
	}
}

// In-memory storage, simulating Redis
type MemoryStore struct {
	quotaData map[string]int  // Total quota
	usedData  map[string]int  // Used quota
	starData  map[string]bool // GitHub star status
	mu        sync.RWMutex
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		quotaData: make(map[string]int),
		usedData:  make(map[string]int),
		starData:  make(map[string]bool),
		mu:        sync.RWMutex{},
	}
}

func (m *MemoryStore) GetQuota(key string) (int, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, exists := m.quotaData[key]
	return value, exists
}

func (m *MemoryStore) SetQuota(key string, value int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.quotaData[key] = value
}

func (m *MemoryStore) IncrQuota(key string, delta int) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.quotaData[key] += delta
	return m.quotaData[key]
}

func (m *MemoryStore) GetUsed(key string) (int, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, exists := m.usedData[key]
	return value, exists
}

func (m *MemoryStore) SetUsed(key string, value int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.usedData[key] = value
}

func (m *MemoryStore) IncrUsed(key string, delta int) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.usedData[key] += delta
	return m.usedData[key]
}

func (m *MemoryStore) GetStar(key string) (bool, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, exists := m.starData[key]
	return value, exists
}

func (m *MemoryStore) SetStar(key string, value bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.starData[key] = value
}

var store = NewMemoryStore()

func main() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// Middleware: Verify Authorization
	authMiddleware := func(c *gin.Context) {
		auth := c.GetHeader("x-admin-key")
		if auth != "credential3" {
			c.JSON(http.StatusForbidden, NewErrorResponse("ai-gateway.unauthorized", "Management API authentication failed"))
			c.Abort()
			return
		}
		c.Next()
	}

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// AiGateway API simulation
	v1 := router.Group("/v1/chat/completions")
	v1.Use(authMiddleware)
	{
		// Refresh quota
		v1.POST("/quota/refresh", refreshQuota)

		// Query quota
		v1.GET("/quota", queryQuota)

		// Increase/decrease quota
		v1.POST("/quota/delta", deltaQuota)

		// Query used quota
		v1.GET("/quota/used", queryUsedQuota)

		// Increase/decrease used quota
		v1.POST("/quota/used/delta", deltaUsedQuota)

		// Refresh used quota
		v1.POST("/quota/used/refresh", refreshUsedQuota)

		// Query GitHub star status
		v1.GET("/quota/star", queryGithubStar)

		// Set GitHub star status
		v1.POST("/quota/star/set", setGithubStar)
	}

	fmt.Println("AiGateway Mock Service starting on port 1002")
	if err := router.Run(":1002"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// refreshQuota refreshes the quota
func refreshQuota(c *gin.Context) {
	userID := c.PostForm("user_id")
	quotaStr := c.PostForm("quota")

	if userID == "" || quotaStr == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse("ai-gateway.invalid_params", "user_id and quota are required"))
		return
	}

	quota, err := strconv.Atoi(quotaStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse("ai-gateway.invalid_params", "quota must be integer"))
		return
	}

	key := fmt.Sprintf("chat_quota:%s", userID)
	store.SetQuota(key, quota)

	c.JSON(http.StatusOK, NewSuccessResponse("ai-gateway.refreshquota", "refresh quota successful", nil))
}

// queryQuota queries the quota
func queryQuota(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse("ai-gateway.invalid_params", "user_id is required"))
		return
	}

	key := fmt.Sprintf("chat_quota:%s", userID)
	quota, exists := store.GetQuota(key)
	if !exists {
		quota = 0 // Default quota is 0
	}

	data := map[string]interface{}{
		"user_id": userID,
		"quota":   quota,
		"type":    "total_quota",
	}

	c.JSON(http.StatusOK, NewSuccessResponse("ai-gateway.queryquota", "query quota successful", data))
}

// deltaQuota increases or decreases the quota
func deltaQuota(c *gin.Context) {
	userID := c.PostForm("user_id")
	valueStr := c.PostForm("value")

	if userID == "" || valueStr == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse("ai-gateway.invalid_params", "user_id and value are required"))
		return
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse("ai-gateway.invalid_params", "value must be integer"))
		return
	}

	key := fmt.Sprintf("chat_quota:%s", userID)
	store.IncrQuota(key, value)

	c.JSON(http.StatusOK, NewSuccessResponse("ai-gateway.deltaquota", "delta quota successful", nil))
}

// queryUsedQuota queries the used quota
func queryUsedQuota(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse("ai-gateway.invalid_params", "user_id is required"))
		return
	}

	key := fmt.Sprintf("chat_quota:%s", userID)
	used, exists := store.GetUsed(key)
	if !exists {
		used = 0 // Default used quota is 0
	}

	data := map[string]interface{}{
		"user_id": userID,
		"quota":   used,
		"type":    "used_quota",
	}

	c.JSON(http.StatusOK, NewSuccessResponse("ai-gateway.queryquota", "query quota successful", data))
}

// deltaUsedQuota increases or decreases the used quota
func deltaUsedQuota(c *gin.Context) {
	userID := c.PostForm("user_id")
	valueStr := c.PostForm("value")

	if userID == "" || valueStr == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse("ai-gateway.invalid_params", "user_id and value are required"))
		return
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse("ai-gateway.invalid_params", "value must be integer"))
		return
	}

	key := fmt.Sprintf("chat_quota:%s", userID)
	store.IncrUsed(key, value)

	c.JSON(http.StatusOK, NewSuccessResponse("ai-gateway.deltausedquota", "delta used quota successful", nil))
}

// refreshUsedQuota refreshes the used quota
func refreshUsedQuota(c *gin.Context) {
	userID := c.PostForm("user_id")
	quotaStr := c.PostForm("quota")

	if userID == "" || quotaStr == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse("ai-gateway.invalid_params", "user_id and quota are required"))
		return
	}

	quota, err := strconv.Atoi(quotaStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, NewErrorResponse("ai-gateway.invalid_params", "quota must be integer"))
		return
	}

	key := fmt.Sprintf("chat_quota:%s", userID)
	store.SetUsed(key, quota)

	c.JSON(http.StatusOK, NewSuccessResponse("ai-gateway.refreshusedquota", "refresh used quota successful", nil))
}

// queryGithubStar queries the GitHub star status
func queryGithubStar(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse("ai-gateway.invalid_params", "user_id is required"))
		return
	}

	key := fmt.Sprintf("chat_quota:%s", userID)
	star, exists := store.GetStar(key)
	starValue := "false"
	if exists && star {
		starValue = "true"
	}

	data := map[string]string{
		"user_id":    userID,
		"star_value": starValue,
		"type":       "star_status",
	}

	c.JSON(http.StatusOK, NewSuccessResponse("ai-gateway.querystar", "query star status successful", data))
}

// setGithubStar sets the GitHub star status
func setGithubStar(c *gin.Context) {
	userID := c.PostForm("user_id")
	starValueStr := c.PostForm("star_value")

	if userID == "" || starValueStr == "" {
		c.JSON(http.StatusBadRequest, NewErrorResponse("ai-gateway.invalid_params", "user_id and star_value are required"))
		return
	}

	// Convert string to boolean - accept "true" and "false" strings
	var starValue bool
	switch starValueStr {
	case "true":
		starValue = true
	case "false":
		starValue = false
	default:
		c.JSON(http.StatusBadRequest, NewErrorResponse("ai-gateway.invalid_params", "star_value must be 'true' or 'false'"))
		return
	}

	key := fmt.Sprintf("chat_quota:%s", userID)
	store.SetStar(key, starValue)

	c.JSON(http.StatusOK, NewSuccessResponse("ai-gateway.setstar", "set star status successful", nil))
}
