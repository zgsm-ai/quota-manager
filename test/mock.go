package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
)

// MockQuotaStore mock quota storage
type MockQuotaStore struct {
	data     map[string]int // Total quota
	usedData map[string]int // Used quota
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

var mockStore = &MockQuotaStore{
	data:     make(map[string]int),
	usedData: make(map[string]int),
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
		v1.POST("/quota/refresh", func(c *gin.Context) {
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

		v1.GET("/quota", func(c *gin.Context) {
			if shouldFail {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				return
			}

			userID := c.Query("user_id")
			quota := mockStore.GetQuota(userID)

			c.JSON(http.StatusOK, gin.H{
				"quota":   quota,
				"user_id": userID,
			})
		})

		v1.POST("/quota/delta", func(c *gin.Context) {
			if shouldFail {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				return
			}

			userID := c.PostForm("user_id")
			value := c.PostForm("value")

			if userID == "" || value == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "missing parameters"})
				return
			}

			// Simulate quota increase
			var delta int
			if _, err := fmt.Sscanf(value, "%d", &delta); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid value"})
				return
			}

			newQuota := mockStore.DeltaQuota(userID, delta)

			c.JSON(http.StatusOK, gin.H{
				"message":   "success",
				"user_id":   userID,
				"new_quota": newQuota,
			})
		})

		v1.GET("/quota/used", func(c *gin.Context) {
			if shouldFail {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				return
			}

			userID := c.Query("user_id")
			used := mockStore.GetUsed(userID)

			c.JSON(http.StatusOK, gin.H{
				"quota":   used,
				"user_id": userID,
			})
		})

		v1.POST("/quota/used/delta", func(c *gin.Context) {
			if shouldFail {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				return
			}

			userID := c.PostForm("user_id")
			value := c.PostForm("value")

			if userID == "" || value == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "missing parameters"})
				return
			}

			// Parse and update used quota
			var delta int
			if _, err := fmt.Sscanf(value, "%d", &delta); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid value"})
				return
			}

			newUsed := mockStore.DeltaUsed(userID, delta)

			c.JSON(http.StatusOK, gin.H{
				"message": "success",
				"user_id": userID,
				"new_used": newUsed,
			})
		})
	}

	return httptest.NewServer(router)
}
