package config

import (
	"sync"
)

// Manager 配置管理器，用于管理配置的访问和更新
type Manager struct {
	config *Config
	mutex  sync.RWMutex
}

// NewManager 创建一个新的配置管理器
func NewManager(cfg *Config) *Manager {
	return &Manager{
		config: cfg,
	}
}

// Get 获取当前配置的副本
func (m *Manager) Get() *Config {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 返回配置的副本以避免外部修改直接影响原始配置
	// 注意：对于包含指针或切片的复杂结构体，可能需要深度复制
	configCopy := *m.config
	return &configCopy
}

// Update 更新配置值
func (m *Manager) Update(updater func(*Config)) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	updater(m.config)
}

// GetDirect 获取当前配置的直接引用（谨慎使用）
func (m *Manager) GetDirect() *Config {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.config
}
