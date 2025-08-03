package config

import (
	"sync"
)

// Manager configuration manager, used to manage configuration access and updates
type Manager struct {
	config *Config
	mutex  sync.RWMutex
}

// NewManager creates a new configuration manager
func NewManager(cfg *Config) *Manager {
	return &Manager{
		config: cfg,
	}
}

// Get gets a copy of the current configuration
func (m *Manager) Get() *Config {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Return a copy of the configuration to avoid external modifications directly affecting the original configuration
	// Note: For complex structures containing pointers or slices, deep copying may be required
	configCopy := *m.config
	return &configCopy
}

// Update updates configuration values
func (m *Manager) Update(updater func(*Config)) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	updater(m.config)
}

// GetDirect gets a direct reference to the current configuration (use with caution)
func (m *Manager) GetDirect() *Config {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.config
}
