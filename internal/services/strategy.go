package services

import (
	"fmt"
	"quota-manager/internal/condition"
	"quota-manager/internal/database"
	"quota-manager/internal/models"
	"quota-manager/pkg/aigateway"
	"quota-manager/pkg/logger"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type StrategyService struct {
	db      *database.DB
	gateway *aigateway.Client
	cron    *cron.Cron
}

func NewStrategyService(db *database.DB, gateway *aigateway.Client) *StrategyService {
	return &StrategyService{
		db:      db,
		gateway: gateway,
		cron:    cron.New(),
	}
}

// Start starts the strategy scan service
func (s *StrategyService) Start() error {
	// Add task to scan every hour
	_, err := s.cron.AddFunc("0 * * * *", s.TraverseStrategy)
	if err != nil {
		return fmt.Errorf("failed to add cron job: %w", err)
	}

	s.cron.Start()
	logger.Info("Strategy service started")
	return nil
}

// Stop stops the strategy scan service
func (s *StrategyService) Stop() {
	s.cron.Stop()
	logger.Info("Strategy service stopped")
}

// TraverseStrategy traverses the strategy table
func (s *StrategyService) TraverseStrategy() {
	logger.Info("Starting strategy traversal")

	// 1. Get user list
	users, err := s.loadUsers()
	if err != nil {
		logger.Error("Failed to load users", zap.Error(err))
		return
	}

	// 2. Get enabled strategy list
	strategies, err := s.loadEnabledStrategies()
	if err != nil {
		logger.Error("Failed to load enabled strategies", zap.Error(err))
		return
	}

	logger.Info("Found enabled strategies", zap.Int("count", len(strategies)))

	// 3. Execute strategies
	for _, strategy := range strategies {
		logger.Info("Processing strategy",
			zap.String("strategy", strategy.Name),
			zap.Bool("status", strategy.Status))

		switch strategy.Type {
		case "periodic":
			if s.shouldExecutePeriodic(&strategy) {
				s.ExecStrategy(&strategy, users)
			}
		case "single":
			s.ExecStrategy(&strategy, users)
		}
	}

	logger.Info("Strategy traversal completed")
}

// loadUsers loads the user list
func (s *StrategyService) loadUsers() ([]models.UserInfo, error) {
	var users []models.UserInfo
	if err := s.db.Find(&users).Error; err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	return users, nil
}

// loadStrategies loads all strategy list
func (s *StrategyService) loadStrategies() ([]models.QuotaStrategy, error) {
	var strategies []models.QuotaStrategy
	if err := s.db.Find(&strategies).Error; err != nil {
		return nil, fmt.Errorf("failed to query strategies: %w", err)
	}
	return strategies, nil
}

// loadEnabledStrategies loads enabled strategy list
func (s *StrategyService) loadEnabledStrategies() ([]models.QuotaStrategy, error) {
	var strategies []models.QuotaStrategy
	if err := s.db.Where("status = ?", true).Find(&strategies).Error; err != nil {
		return nil, fmt.Errorf("failed to query enabled strategies: %w", err)
	}
	return strategies, nil
}

// shouldExecutePeriodic determines if periodic strategy should be executed
func (s *StrategyService) shouldExecutePeriodic(strategy *models.QuotaStrategy) bool {
	if strategy.PeriodicExpr == "" {
		return false
	}

	// Parse cron expression to determine if it should be executed
	schedule, err := cron.ParseStandard(strategy.PeriodicExpr)
	if err != nil {
		logger.Error("Invalid cron expression",
			zap.String("strategy", strategy.Name),
			zap.String("expr", strategy.PeriodicExpr),
			zap.Error(err))
		return false
	}

	now := time.Now()
	next := schedule.Next(now.Add(-time.Hour))

	// If next execution time is before or equal to current time, it should be executed
	return next.Before(now) || next.Equal(now)
}

// ExecStrategy executes a strategy
func (s *StrategyService) ExecStrategy(strategy *models.QuotaStrategy, users []models.UserInfo) {
	// Get the latest strategy status from the database
	var latestStrategy models.QuotaStrategy
	if err := s.db.First(&latestStrategy, strategy.ID).Error; err != nil {
		logger.Error("Failed to get latest strategy status", zap.Error(err))
		return
	}

	// Use the latest strategy status for checking
	if !latestStrategy.IsEnabled() {
		logger.Warn("Skipping disabled strategy", zap.String("strategy", latestStrategy.Name))
		return
	}

	batchNumber := s.generateBatchNumber()

	for _, user := range users {
		// For single strategy, check if it has already been executed
		if latestStrategy.Type == "single" {
			if s.hasExecuted(latestStrategy.ID, user.ID) {
				continue
			}
		}

		// Check condition
		match, err := condition.CalcCondition(&user, latestStrategy.Condition, s.gateway)
		if err != nil {
			logger.Error("Failed to calculate condition",
				zap.String("user", user.ID),
				zap.String("strategy", latestStrategy.Name),
				zap.Error(err))
			continue
		}

		if !match {
			continue
		}

		// Execute recharge
		if err := s.executeRecharge(&latestStrategy, &user, batchNumber); err != nil {
			logger.Error("Failed to execute recharge",
				zap.String("user", user.ID),
				zap.String("strategy", latestStrategy.Name),
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
	// Check strategy status again
	var latestStrategy models.QuotaStrategy
	if err := s.db.First(&latestStrategy, strategy.ID).Error; err != nil {
		return fmt.Errorf("failed to get latest strategy status: %w", err)
	}

	if !latestStrategy.IsEnabled() {
		return fmt.Errorf("strategy is disabled")
	}

	// 1. Record execution status as processing
	execute := &models.QuotaExecute{
		StrategyID:  latestStrategy.ID,
		User:        user.ID,
		BatchNumber: batchNumber,
		Status:      "processing",
	}

	if err := s.db.Create(execute).Error; err != nil {
		return fmt.Errorf("failed to create execute record: %w", err)
	}

	// 2. Call AiGateway for recharge
	err := s.gateway.DeltaQuota(user.ID, latestStrategy.Amount)
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
		zap.String("strategy", latestStrategy.Name),
		zap.Int("amount", latestStrategy.Amount),
		zap.String("model", latestStrategy.Model))

	return nil
}

// generateBatchNumber generates batch number
func (s *StrategyService) generateBatchNumber() string {
	now := time.Now()
	return now.Format("2006010215") // YearMonthDayHour
}

// CreateStrategy creates a strategy
func (s *StrategyService) CreateStrategy(strategy *models.QuotaStrategy) error {
	// Use GORM's default value mechanism
	// The Status field is already defined as default:true in the model
	if err := s.db.Create(strategy).Error; err != nil {
		return fmt.Errorf("failed to create strategy: %w", err)
	}

	// Reload the strategy from the database to get the default value
	return s.db.First(strategy, strategy.ID).Error
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
	return s.loadEnabledStrategies()
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

// UpdateStrategy updates a strategy
func (s *StrategyService) UpdateStrategy(id int, updates map[string]interface{}) error {
	if err := s.db.Model(&models.QuotaStrategy{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update strategy: %w", err)
	}
	return nil
}

// EnableStrategy enables a strategy
func (s *StrategyService) EnableStrategy(id int) error {
	return s.UpdateStrategy(id, map[string]interface{}{"status": true})
}

// DisableStrategy disables a strategy
func (s *StrategyService) DisableStrategy(id int) error {
	return s.UpdateStrategy(id, map[string]interface{}{"status": false})
}

// DeleteStrategy deletes a strategy
func (s *StrategyService) DeleteStrategy(id int) error {
	if err := s.db.Delete(&models.QuotaStrategy{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete strategy: %w", err)
	}
	return nil
}
