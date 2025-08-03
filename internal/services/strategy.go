package services

import (
	"fmt"
	"quota-manager/internal/condition"
	"quota-manager/internal/config"
	"quota-manager/internal/database"
	"quota-manager/internal/models"
	"quota-manager/pkg/aigateway"
	"quota-manager/pkg/logger"
	"strings"
	"sync"
	"time"

	"database/sql"
	"errors"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// StrategyDatabaseQuerier implements condition.DatabaseQuerier interface
type StrategyDatabaseQuerier struct {
	db *database.DB
}

func (q *StrategyDatabaseQuerier) QueryEmployeeDepartment(employeeNumber string) ([]string, error) {
	var employee models.EmployeeDepartment
	err := q.db.DB.Where("employee_number = ?", employeeNumber).First(&employee).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return []string{}, nil // Return empty slice for non-existent employees
		}
		return nil, fmt.Errorf("failed to query employee department: %w", err)
	}

	return employee.GetDeptFullLevelNamesAsSlice(), nil
}

// StrategyConfigQuerier implements condition.ConfigQuerier interface
type StrategyConfigQuerier struct {
	employeeSyncConfig *config.EmployeeSyncConfig
}

func (q *StrategyConfigQuerier) IsEmployeeSyncEnabled() bool {
	return q.employeeSyncConfig != nil && q.employeeSyncConfig.Enabled
}

type StrategyService struct {
	db                 *database.DB
	gateway            *aigateway.Client
	quotaQuerier       condition.QuotaQuerier
	quotaService       *QuotaService
	cron               *cron.Cron
	cronJobs           map[int]cron.EntryID // strategyID -> cronEntryID
	mu                 sync.RWMutex         // protect cronJobs map
	databaseQuerier    condition.DatabaseQuerier
	configQuerier      condition.ConfigQuerier
	employeeSyncConfig *config.EmployeeSyncConfig
}

// NewStrategyService creates a new strategy service
func NewStrategyService(db *database.DB, gateway *aigateway.Client, quotaService *QuotaService, employeeSyncConfig *config.EmployeeSyncConfig) *StrategyService {
	// Create queriers for condition evaluation
	dbQuerier := &StrategyDatabaseQuerier{db: db}
	cfgQuerier := &StrategyConfigQuerier{employeeSyncConfig: employeeSyncConfig}

	return &StrategyService{
		db:                 db,
		gateway:            gateway,
		quotaQuerier:       condition.NewAiGatewayQuotaQuerier(gateway),
		quotaService:       quotaService,
		cron:               cron.New(cron.WithSeconds()),
		cronJobs:           make(map[int]cron.EntryID),
		databaseQuerier:    dbQuerier,
		configQuerier:      cfgQuerier,
		employeeSyncConfig: employeeSyncConfig,
	}
}

// StartCron starts the cron scheduler
func (s *StrategyService) StartCron() error {
	// Load all enabled periodic strategies and register them
	strategies, err := s.loadEnabledPeriodicStrategies()
	if err != nil {
		return fmt.Errorf("failed to load enabled periodic strategies: %w", err)
	}

	for _, strategy := range strategies {
		if err := s.registerPeriodicStrategy(&strategy); err != nil {
			logger.Error("Failed to register periodic strategy",
				zap.String("strategy", strategy.Name),
				zap.Error(err))
		}
	}

	s.cron.Start()
	logger.Info("Strategy cron scheduler started", zap.Int("periodic_strategies", len(strategies)))
	return nil
}

// StopCron stops the cron scheduler
func (s *StrategyService) StopCron() {
	s.cron.Stop()
	logger.Info("Strategy cron scheduler stopped")
}

// registerPeriodicStrategy registers a periodic strategy to cron
func (s *StrategyService) registerPeriodicStrategy(strategy *models.QuotaStrategy) error {
	if strategy.Type != "periodic" || strategy.PeriodicExpr == "" {
		return fmt.Errorf("strategy %s is not a valid periodic strategy", strategy.Name)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove existing job if any
	if entryID, exists := s.cronJobs[strategy.ID]; exists {
		s.cron.Remove(entryID)
		delete(s.cronJobs, strategy.ID)
	}

	// Add new job
	entryID, err := s.cron.AddFunc(strategy.PeriodicExpr, func() {
		s.executePeriodicStrategy(strategy.ID)
	})
	if err != nil {
		return fmt.Errorf("failed to add cron job for strategy %s: %w", strategy.Name, err)
	}

	s.cronJobs[strategy.ID] = entryID
	logger.Info("Registered periodic strategy to cron",
		zap.String("strategy", strategy.Name),
		zap.String("expression", strategy.PeriodicExpr))
	return nil
}

// unregisterPeriodicStrategy removes a periodic strategy from cron
func (s *StrategyService) unregisterPeriodicStrategy(strategyID int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, exists := s.cronJobs[strategyID]; exists {
		s.cron.Remove(entryID)
		delete(s.cronJobs, strategyID)
		logger.Info("Unregistered periodic strategy from cron", zap.Int("strategy_id", strategyID))
	}
}

// executePeriodicStrategy executes a specific periodic strategy
func (s *StrategyService) executePeriodicStrategy(strategyID int) {
	// Get strategy details
	strategy, err := s.GetStrategy(strategyID)
	if err != nil {
		logger.Error("Failed to get strategy for execution",
			zap.Int("strategy_id", strategyID),
			zap.Error(err))
		return
	}

	// Check if strategy is still enabled
	if !strategy.IsEnabled() {
		logger.Warn("Skipping disabled strategy", zap.String("strategy", strategy.Name))
		return
	}

	// Get users
	users, err := s.loadUsers()
	if err != nil {
		logger.Error("Failed to load users for strategy execution",
			zap.String("strategy", strategy.Name),
			zap.Error(err))
		return
	}

	logger.Info("Executing periodic strategy",
		zap.String("strategy", strategy.Name),
		zap.Int("user_count", len(users)))

	// Execute strategy
	s.ExecStrategy(strategy, users)
}

// loadEnabledPeriodicStrategies loads enabled periodic strategies with retry mechanism
func (s *StrategyService) loadEnabledPeriodicStrategies() ([]models.QuotaStrategy, error) {
	var strategies []models.QuotaStrategy
	var err error

	// Retry mechanism, maximum 3 attempts
	for i := 0; i < 3; i++ {
		// Check if connection is healthy
		if sqlDB, dbErr := s.db.DB.DB(); dbErr == nil {
			if pingErr := sqlDB.Ping(); pingErr != nil {
				logger.Warn("DB connection ping failed in loadEnabledPeriodicStrategies, attempting to reconnect",
					zap.Int("attempt", i+1), zap.Error(pingErr))
				time.Sleep(time.Duration(i+1) * time.Second) // Exponential backoff
				continue
			}
		}

		err = s.db.Where("status = ? AND type = ?", true, "periodic").Find(&strategies).Error
		if err == nil {
			logger.Info("Successfully loaded enabled periodic strategies", zap.Int("count", len(strategies)))
			return strategies, nil
		}

		// Retry if it's a network-related error
		if isNetworkError(err) {
			logger.Warn("Network error occurred while loading enabled periodic strategies, retrying",
				zap.Int("attempt", i+1),
				zap.Error(err),
				zap.Bool("is_network_error", true))
			time.Sleep(time.Duration(i+1) * time.Second) // Exponential backoff
			continue
		}

		// Non-network error, return directly
		break
	}

	return nil, fmt.Errorf("failed to query enabled periodic strategies after retries: %w", err)
}

// TraverseSingleStrategies traverses single-type strategies only
// Periodic strategies are now handled by cron directly
func (s *StrategyService) TraverseSingleStrategies() {
	logger.Info("Starting single strategy traversal")

	// 1. Get user list
	users, err := s.loadUsers()
	if err != nil {
		logger.Error("Failed to load users", zap.Error(err))
		return
	}

	// 2. Get enabled single-type strategies
	strategies, err := s.loadEnabledSingleStrategies()
	if err != nil {
		logger.Error("Failed to load enabled single strategies", zap.Error(err))
		return
	}

	logger.Info("Found enabled single strategies", zap.Int("count", len(strategies)))

	// 3. Execute single strategies
	for _, strategy := range strategies {
		logger.Info("Processing single strategy",
			zap.String("strategy", strategy.Name))
		s.ExecStrategy(&strategy, users)
	}

	logger.Info("Single strategy traversal completed")
}

// loadEnabledSingleStrategies loads enabled single-type strategies with retry mechanism
func (s *StrategyService) loadEnabledSingleStrategies() ([]models.QuotaStrategy, error) {
	var strategies []models.QuotaStrategy
	var err error

	// Retry mechanism, maximum 3 attempts
	for i := 0; i < 3; i++ {
		// Check if connection is healthy
		if sqlDB, dbErr := s.db.DB.DB(); dbErr == nil {
			if pingErr := sqlDB.Ping(); pingErr != nil {
				logger.Warn("DB connection ping failed in loadEnabledSingleStrategies, attempting to reconnect",
					zap.Int("attempt", i+1), zap.Error(pingErr))
				time.Sleep(time.Duration(i+1) * time.Second) // Exponential backoff
				continue
			}
		}

		err = s.db.Where("status = ? AND type = ?", true, "single").Find(&strategies).Error
		if err == nil {
			logger.Info("Successfully loaded enabled single strategies", zap.Int("count", len(strategies)))
			return strategies, nil
		}

		// Retry if it's a network-related error
		if isNetworkError(err) {
			logger.Warn("Network error occurred while loading enabled single strategies, retrying",
				zap.Int("attempt", i+1),
				zap.Error(err),
				zap.Bool("is_network_error", true))
			time.Sleep(time.Duration(i+1) * time.Second) // Exponential backoff
			continue
		}

		// Non-network error, return directly
		break
	}

	logger.Error("Failed to load enabled single strategies after all retries",
		zap.Error(err),
		zap.Bool("is_network_error", isNetworkError(err)),
		zap.Int("retry_attempts", 3))

	return nil, fmt.Errorf("failed to query enabled single strategies after retries: %w", err)
}

// loadUsers loads the user list with retry mechanism
func (s *StrategyService) loadUsers() ([]models.UserInfo, error) {
	var users []models.UserInfo
	var err error

	// Retry mechanism, maximum 3 attempts
	for i := 0; i < 3; i++ {
		// Check if connection is healthy
		if sqlDB, dbErr := s.db.AuthDB.DB(); dbErr == nil {
			if pingErr := sqlDB.Ping(); pingErr != nil {
				logger.Warn("AuthDB connection ping failed, attempting to reconnect",
					zap.Int("attempt", i+1), zap.Error(pingErr))
				time.Sleep(time.Duration(i+1) * time.Second) // Exponential backoff
				continue
			}
		}

		err = s.db.AuthDB.Find(&users).Error
		if err == nil {
			return users, nil
		}

		// Retry if it's a network-related error
		if isNetworkError(err) {
			logger.Warn("Network error occurred while loading users, retrying",
				zap.Int("attempt", i+1), zap.Error(err))
			time.Sleep(time.Duration(i+1) * time.Second) // Exponential backoff
			continue
		}

		// Non-network error, return directly
		break
	}

	return nil, fmt.Errorf("failed to query users after retries: %w", err)
}

// isNetworkError checks if the error is network-related
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's a network connection related error
	var netErr error
	if errors.As(err, &netErr) {
		return true
	}

	// Check specific database errors
	if errors.Is(err, sql.ErrConnDone) ||
		errors.Is(err, sql.ErrTxDone) ||
		errors.Is(err, gorm.ErrInvalidDB) {
		return true
	}

	// Check if error message contains network-related keywords
	errMsg := strings.ToLower(err.Error())
	networkKeywords := []string{
		"connection reset", "connection refused", "connection timeout",
		"network is unreachable", "no route to host", "broken pipe",
		"unexpected eof", "connection closed", "timeout",
		"temporary failure", "resource temporarily unavailable",
	}

	for _, keyword := range networkKeywords {
		if strings.Contains(errMsg, keyword) {
			return true
		}
	}

	return false
}

// loadStrategies loads all strategy list
func (s *StrategyService) loadStrategies() ([]models.QuotaStrategy, error) {
	var strategies []models.QuotaStrategy
	if err := s.db.Find(&strategies).Error; err != nil {
		return nil, fmt.Errorf("failed to query strategies: %w", err)
	}
	return strategies, nil
}

// ExecStrategy executes a strategy
func (s *StrategyService) ExecStrategy(strategy *models.QuotaStrategy, users []models.UserInfo) {
	// Validate strategy status (should already be enabled since we got it from loadEnabledStrategies)
	if !strategy.IsEnabled() {
		logger.Warn("Skipping disabled strategy", zap.String("strategy", strategy.Name))
		return
	}

	batchNumber := s.generateBatchNumber()

	for _, user := range users {
		// For single strategy, check if it has already been executed
		if strategy.Type == "single" {
			if s.hasExecuted(strategy.ID, user.ID) {
				continue
			}
		}
		// Periodic strategies are now handled directly by cron, no batch checking needed

		// Check condition
		ctx := &condition.EvaluationContext{
			QuotaQuerier:    s.quotaQuerier,
			DatabaseQuerier: s.databaseQuerier,
			ConfigQuerier:   s.configQuerier,
		}
		match, err := condition.CalcCondition(&user, strategy.Condition, ctx)
		if err != nil {
			logger.Error("Failed to calculate condition",
				zap.String("user", user.ID),
				zap.String("strategy", strategy.Name),
				zap.Error(err))
			continue
		}

		if !match {
			continue
		}

		// Execute recharge
		if err := s.executeRecharge(strategy, &user, batchNumber); err != nil {
			logger.Error("Failed to execute recharge",
				zap.String("user", user.ID),
				zap.String("strategy", strategy.Name),
				zap.Error(err))
		}
	}
}

// hasExecuted checks if single strategy has been executed
func (s *StrategyService) hasExecuted(strategyID int, userID string) bool {
	var count int64

	// First check if there is a completed record
	err := s.db.Model(&models.QuotaExecute{}).
		Where("strategy_id = ? AND user_id = ? AND status = ?",
			strategyID, userID, "completed").
		Count(&count).Error

	if err != nil {
		logger.Error("Failed to check execution status", zap.Error(err))
		return false
	}

	if count > 0 {
		return true
	}

	// If there is no completed record, check for processing records in the current batch
	currentBatch := s.generateBatchNumber()
	err = s.db.Model(&models.QuotaExecute{}).
		Where("strategy_id = ? AND user_id = ? AND status = ? AND batch_number = ?",
			strategyID, userID, "processing", currentBatch).
		Count(&count).Error

	if err != nil {
		logger.Error("Failed to check execution status", zap.Error(err))
		return false
	}

	return count > 0
}

// executeRecharge executes recharge
func (s *StrategyService) executeRecharge(strategy *models.QuotaStrategy, user *models.UserInfo, batchNumber string) error {
	// Strategy should already be validated as enabled before reaching here
	if !strategy.IsEnabled() {
		return fmt.Errorf("strategy is disabled")
	}

	// Calculate expiry date (end of this/next month)
	now := time.Now().Truncate(time.Second)
	var expiryDate time.Time

	// Always set to end of current month
	expiryDate = time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, now.Location())

	// 1. Record execution status as processing
	execute := &models.QuotaExecute{
		StrategyID:  strategy.ID,
		User:        user.ID,
		BatchNumber: batchNumber,
		Status:      "processing",
		ExpiryDate:  expiryDate,
	}

	if err := s.db.Create(execute).Error; err != nil {
		return fmt.Errorf("failed to create execute record: %w", err)
	}

	// 2. Add quota using QuotaService
	err := s.quotaService.AddQuotaForStrategy(user.ID, strategy.Amount, strategy.ID, strategy.Name)
	if err != nil {
		// Update execution status to failed
		s.db.Model(execute).Update("status", "failed")
		return fmt.Errorf("failed to recharge quota: %w", err)
	}

	// 3. Update execution status to completed
	if err := s.db.Model(execute).Update("status", "completed").Error; err != nil {
		logger.Error("Failed to update execute status", zap.Error(err))
	}

	logger.Info("Recharge completed",
		zap.String("user", user.ID),
		zap.String("strategy", strategy.Name),
		zap.Float64("amount", strategy.Amount),
		zap.String("model", strategy.Model),
		zap.Time("expiry_date", expiryDate))

	return nil
}

// generateBatchNumber generates batch number with second precision
func (s *StrategyService) generateBatchNumber() string {
	now := time.Now().Truncate(time.Second)
	return now.Format("20060102150405") // YearMonthDayHourMinuteSecond
}

// CreateStrategy creates a strategy and registers periodic ones to cron
func (s *StrategyService) CreateStrategy(strategy *models.QuotaStrategy) error {
	// Validate cron expression for periodic strategies before saving
	if strategy.Type == "periodic" {
		if strategy.PeriodicExpr == "" {
			return fmt.Errorf("periodic expression cannot be empty for periodic strategy")
		}
		// Test cron expression by trying to add it to a temporary cron instance
		tempCron := cron.New(cron.WithSeconds())
		_, err := tempCron.AddFunc(strategy.PeriodicExpr, func() {})
		if err != nil {
			return fmt.Errorf("invalid cron expression '%s': %w", strategy.PeriodicExpr, err)
		}
	}

	// Create strategy in database
	if err := s.db.Create(strategy).Error; err != nil {
		return fmt.Errorf("failed to create strategy: %w", err)
	}

	// Reload the strategy from the database to get the default value
	if err := s.db.First(strategy, strategy.ID).Error; err != nil {
		return fmt.Errorf("failed to reload strategy: %w", err)
	}

	// Register to cron if it's an enabled periodic strategy
	if strategy.Type == "periodic" && strategy.IsEnabled() {
		if err := s.registerPeriodicStrategy(strategy); err != nil {
			logger.Error("Failed to register periodic strategy to cron",
				zap.String("strategy", strategy.Name),
				zap.Error(err))
			// Don't fail the creation, just log the error
		}
	}

	return nil
}

// GetStrategies gets strategy list
func (s *StrategyService) GetStrategies() ([]models.QuotaStrategy, error) {
	var strategies []models.QuotaStrategy
	if err := s.db.Find(&strategies).Error; err != nil {
		return nil, fmt.Errorf("failed to get strategies: %w", err)
	}
	return strategies, nil
}

// GetEnabledStrategies gets enabled strategy list
func (s *StrategyService) GetEnabledStrategies() ([]models.QuotaStrategy, error) {
	var strategies []models.QuotaStrategy
	if err := s.db.Where("status = ?", true).Find(&strategies).Error; err != nil {
		return nil, fmt.Errorf("failed to query enabled strategies: %w", err)
	}
	return strategies, nil
}

// GetDisabledStrategies gets disabled strategy list
func (s *StrategyService) GetDisabledStrategies() ([]models.QuotaStrategy, error) {
	var strategies []models.QuotaStrategy
	if err := s.db.Where("status = ?", false).Find(&strategies).Error; err != nil {
		return nil, fmt.Errorf("failed to query disabled strategies: %w", err)
	}
	return strategies, nil
}

// GetStrategy gets a single strategy
func (s *StrategyService) GetStrategy(id int) (*models.QuotaStrategy, error) {
	var strategy models.QuotaStrategy
	if err := s.db.First(&strategy, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("strategy not found")
		}
		return nil, fmt.Errorf("failed to get strategy: %w", err)
	}
	return &strategy, nil
}

// UpdateStrategy updates a strategy and manages cron registration
func (s *StrategyService) UpdateStrategy(id int, updates map[string]interface{}) error {
	// Get current strategy
	oldStrategy, err := s.GetStrategy(id)
	if err != nil {
		return fmt.Errorf("failed to get strategy: %w", err)
	}

	// Validate cron expression if being updated for periodic strategies
	if periodicExpr, exists := updates["periodic_expr"]; exists {
		if periodicExprStr, ok := periodicExpr.(string); ok {
			// Check if this is a periodic strategy or being changed to periodic
			strategyType := oldStrategy.Type
			if newType, typeExists := updates["type"]; typeExists {
				if newTypeStr, ok := newType.(string); ok {
					strategyType = newTypeStr
				}
			}

			if strategyType == "periodic" {
				if periodicExprStr == "" {
					return fmt.Errorf("periodic expression cannot be empty for periodic strategy")
				}
				// Test cron expression by trying to add it to a temporary cron instance
				tempCron := cron.New(cron.WithSeconds())
				_, err := tempCron.AddFunc(periodicExprStr, func() {})
				if err != nil {
					return fmt.Errorf("invalid cron expression '%s': %w", periodicExprStr, err)
				}
			}
		}
	}

	// Update strategy in database
	if err := s.db.Model(&models.QuotaStrategy{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update strategy: %w", err)
	}

	// Get updated strategy
	newStrategy, err := s.GetStrategy(id)
	if err != nil {
		return fmt.Errorf("failed to get updated strategy: %w", err)
	}

	// Handle cron registration changes
	if newStrategy.Type == "periodic" {
		if newStrategy.IsEnabled() {
			// Register or re-register to cron
			if err := s.registerPeriodicStrategy(newStrategy); err != nil {
				logger.Error("Failed to register updated periodic strategy to cron",
					zap.String("strategy", newStrategy.Name),
					zap.Error(err))
			}
		} else {
			// Unregister from cron if disabled
			s.unregisterPeriodicStrategy(newStrategy.ID)
		}
	} else if oldStrategy.Type == "periodic" {
		// Strategy type changed from periodic to single, unregister
		s.unregisterPeriodicStrategy(id)
	}

	return nil
}

// EnableStrategy enables a strategy and registers periodic ones to cron
func (s *StrategyService) EnableStrategy(id int) error {
	// UpdateStrategy already handles cron registration for periodic strategies
	return s.UpdateStrategy(id, map[string]interface{}{"status": true})
}

// DisableStrategy disables a strategy and unregisters periodic ones from cron
func (s *StrategyService) DisableStrategy(id int) error {
	// UpdateStrategy already handles cron unregistration for periodic strategies
	return s.UpdateStrategy(id, map[string]interface{}{"status": false})
}

// DeleteStrategy deletes a strategy and unregisters periodic ones from cron
func (s *StrategyService) DeleteStrategy(id int) error {
	// Unregister from cron first
	s.unregisterPeriodicStrategy(id)

	// Use transaction to ensure data consistency
	return s.db.Transaction(func(tx *gorm.DB) error {
		// First, delete all related execution records
		if err := tx.Where("strategy_id = ?", id).Delete(&models.QuotaExecute{}).Error; err != nil {
			return fmt.Errorf("failed to delete related execution records: %w", err)
		}

		// Then delete the strategy itself
		if err := tx.Delete(&models.QuotaStrategy{}, id).Error; err != nil {
			return fmt.Errorf("failed to delete strategy: %w", err)
		}

		return nil
	})
}

// GetStrategyExecuteRecords gets execution records for a strategy
func (s *StrategyService) GetStrategyExecuteRecords(strategyID int, page, pageSize int) ([]models.QuotaExecute, int64, error) {
	var records []models.QuotaExecute
	var total int64

	// Get total count
	if err := s.db.Model(&models.QuotaExecute{}).Where("strategy_id = ?", strategyID).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count execution records: %w", err)
	}

	// Get records with pagination
	offset := (page - 1) * pageSize
	if err := s.db.Where("strategy_id = ?", strategyID).
		Order("create_time DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&records).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to query execution records: %w", err)
	}

	return records, total, nil
}
