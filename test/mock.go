package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
)

// SetStarCall represents a SetGithubStar call for testing
type SetStarCall struct {
	UserID    string
	StarValue bool
}

// MockQuotaStore mock quota storage
type MockQuotaStore struct {
	data         map[string]int  // Total quota
	usedData     map[string]int  // Used quota
	starData     map[string]bool // GitHub star status
	setStarCalls []SetStarCall   // Track SetGithubStar calls
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

func (m *MockQuotaStore) SetGithubStar(userID string, starValue bool) {
	m.starData[userID] = starValue
	m.setStarCalls = append(m.setStarCalls, SetStarCall{
		UserID:    userID,
		StarValue: starValue,
	})
}

func (m *MockQuotaStore) GetSetStarCalls() []SetStarCall {
	return m.setStarCalls
}

func (m *MockQuotaStore) ClearSetStarCalls() {
	m.setStarCalls = []SetStarCall{}
}

var mockStore = &MockQuotaStore{
	data:         make(map[string]int),
	usedData:     make(map[string]int),
	starData:     make(map[string]bool),
	setStarCalls: []SetStarCall{},
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

			quota.POST("/star/set", func(c *gin.Context) {
				if shouldFail {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
					return
				}

				userID := c.PostForm("user_id")
				starValueStr := c.PostForm("star_value")

				if userID == "" || starValueStr == "" {
					c.JSON(http.StatusBadRequest, gin.H{"error": "missing parameters"})
					return
				}

				starValue := starValueStr == "true"
				mockStore.SetGithubStar(userID, starValue)

				c.JSON(http.StatusOK, gin.H{
					"message":    "success",
					"user_id":    userID,
					"star_value": starValue,
				})
			})
		}
	}

	return httptest.NewServer(router)
}
