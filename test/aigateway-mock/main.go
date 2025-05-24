package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
)

// In-memory storage, simulating Redis
type MemoryStore struct {
	data map[string]int
	mu   sync.RWMutex
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		data: make(map[string]int),
		mu:   sync.RWMutex{},
	}
}

func (m *MemoryStore) Get(key string) (int, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, exists := m.data[key]
	return value, exists
}

func (m *MemoryStore) Set(key string, value int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = value
}

func (m *MemoryStore) Incr(key string, delta int) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] += delta
	return m.data[key]
}

var store = NewMemoryStore()

func main() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// Middleware: Verify Authorization
	authMiddleware := func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth != "Bearer credential3" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization"})
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
	}

	fmt.Println("AiGateway Mock Service starting on port 1002")
	if err := router.Run(":1002"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// refreshQuota refreshes the quota
func refreshQuota(c *gin.Context) {
	consumer := c.PostForm("consumer")
	quotaStr := c.PostForm("quota")

	if consumer == "" || quotaStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "consumer and quota are required"})
		return
	}

	quota, err := strconv.Atoi(quotaStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid quota value"})
		return
	}

	key := fmt.Sprintf("chat_quota:%s", consumer)
	store.Set(key, quota)

	c.JSON(http.StatusOK, gin.H{
		"message":  "quota refreshed",
		"consumer": consumer,
		"quota":    quota,
	})
}

// queryQuota queries the quota
func queryQuota(c *gin.Context) {
	consumer := c.Query("consumer")
	if consumer == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "consumer is required"})
		return
	}

	key := fmt.Sprintf("chat_quota:%s", consumer)
	quota, exists := store.Get(key)
	if !exists {
		quota = 0 // Default quota is 0
	}

	c.JSON(http.StatusOK, gin.H{
		"quota":    quota,
		"consumer": consumer,
	})
}

// deltaQuota increases or decreases the quota
func deltaQuota(c *gin.Context) {
	consumer := c.PostForm("consumer")
	valueStr := c.PostForm("value")

	if consumer == "" || valueStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "consumer and value are required"})
		return
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid value"})
		return
	}

	key := fmt.Sprintf("chat_quota:%s", consumer)
	newQuota := store.Incr(key, value)

	c.JSON(http.StatusOK, gin.H{
		"message":   "quota updated",
		"consumer":  consumer,
		"delta":     value,
		"new_quota": newQuota,
	})
}