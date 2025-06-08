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
		auth := c.GetHeader("Authorization")
		if auth != "Bearer credential3" {
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

			consumer := c.PostForm("consumer")
			quota := c.PostForm("quota")

			if consumer == "" || quota == "" {
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

			consumer := c.Query("consumer")
			quota := mockStore.GetQuota(consumer)

			c.JSON(http.StatusOK, gin.H{
				"quota":    quota,
				"consumer": consumer,
			})
		})

		v1.POST("/quota/delta", func(c *gin.Context) {
			if shouldFail {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				return
			}

			consumer := c.PostForm("consumer")
			value := c.PostForm("value")

			if consumer == "" || value == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "missing parameters"})
				return
			}

			// Simulate quota increase
			var delta int
			if _, err := fmt.Sscanf(value, "%d", &delta); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid value"})
				return
			}

			newQuota := mockStore.DeltaQuota(consumer, delta)

			c.JSON(http.StatusOK, gin.H{
				"message":   "success",
				"consumer":  consumer,
				"new_quota": newQuota,
			})
		})

		v1.GET("/quota/used", func(c *gin.Context) {
			if shouldFail {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				return
			}

			consumer := c.Query("consumer")
			used := mockStore.GetUsed(consumer)

			c.JSON(http.StatusOK, gin.H{
				"quota":    used,
				"consumer": consumer,
			})
		})

		v1.POST("/quota/used/delta", func(c *gin.Context) {
			if shouldFail {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				return
			}

			consumer := c.PostForm("consumer")
			value := c.PostForm("value")

			if consumer == "" || value == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "missing parameters"})
				return
			}

			// Parse and update used quota
			var delta int
			if _, err := fmt.Sscanf(value, "%d", &delta); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid value"})
				return
			}

			newUsed := mockStore.DeltaUsed(consumer, delta)

			c.JSON(http.StatusOK, gin.H{
				"message":  "success",
				"consumer": consumer,
				"new_used": newUsed,
			})
		})
	}

	return httptest.NewServer(router)
}
