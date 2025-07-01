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
	db           *database.DB
	gateway      *aigateway.Client
	quotaQuerier condition.QuotaQuerier
	quotaService *QuotaService
}

func NewStrategyService(db *database.DB, gateway *aigateway.Client, quotaService *QuotaService) *StrategyService {
	return &StrategyService{
		db:           db,
		gateway:      gateway,
		quotaQuerier: condition.NewAiGatewayQuotaQuerier(gateway),
		quotaService: quotaService,
	}
}

// TraverseStrategy traverses the strategy table
// This method is called by SchedulerService based on configured interval
func (s *StrategyService) TraverseStrategy() {
	logger.Info("Starting strategy traversal")

	// 1. Get user list
	users, err := s.loadUsers()
	if err != nil {
		logger.Error("Failed to load users", zap.Error(err))
		return
	}

	// 2. Get enabled strategy list (snapshot at this moment)
	strategies, err := s.loadEnabledStrategies()
	if err != nil {
		logger.Error("Failed to load enabled strategies", zap.Error(err))
		return
	}

	logger.Info("Found enabled strategies", zap.Int("count", len(strategies)))

	// 3. Execute strategies based on current snapshot
	// Note: We use the strategy data from the snapshot to avoid redundant database queries
	// and ensure consistent execution within this traversal cycle
	for _, strategy := range strategies {
		logger.Info("Processing strategy",
			zap.String("strategy", strategy.Name),
			zap.String("type", strategy.Type),
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
	if err := s.db.AuthDB.Find(&users).Error; err != nil {
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

	now := time.Now().Truncate(time.Second)

	// Get the current batch number (based on current time)
	currentBatch := s.generateBatchNumber()

	// Check if this strategy has already been executed in the current batch
	var executeCount int64
	err = s.db.Model(&models.QuotaExecute{}).
		Where("strategy_id = ? AND batch_number = ?", strategy.ID, currentBatch).
		Count(&executeCount).Error

	if err != nil {
		logger.Error("Failed to check strategy execution status",
			zap.String("strategy", strategy.Name),
			zap.Error(err))
		return false
	}

	// If already executed in this batch, don't execute again
	if executeCount > 0 {
		logger.Debug("Strategy already executed in current batch",
			zap.String("strategy", strategy.Name),
			zap.String("batch", currentBatch))
		return false
	}

	// Check if current time matches the cron schedule
	// Get the last scheduled time before now
	lastScheduledTime := schedule.Next(now.Add(-24 * time.Hour))

	// If the last scheduled time is within the current hour, execute the strategy
	currentHourStart := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
	nextHourStart := currentHourStart.Add(time.Hour)

	shouldExecute := lastScheduledTime.After(currentHourStart.Add(-time.Minute)) &&
		lastScheduledTime.Before(nextHourStart)

	if shouldExecute {
		logger.Info("Periodic strategy should execute",
			zap.String("strategy", strategy.Name),
			zap.Time("scheduled_time", lastScheduledTime),
			zap.Time("current_time", now))
	}

	return shouldExecute
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
		} else if strategy.Type == "periodic" {
			// For periodic strategy, check if this user has been processed in current batch
			if s.hasExecutedInBatch(strategy.ID, user.ID, batchNumber) {
				continue
			}
		}

		// Check condition
		ctx := &condition.EvaluationContext{
			QuotaQuerier: s.quotaQuerier,
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

// hasExecutedInBatch checks if a user has been processed in the current batch
func (s *StrategyService) hasExecutedInBatch(strategyID int, userID string, batchNumber string) bool {
	var count int64

	// Check if there is a completed or processing record in the current batch
	err := s.db.Model(&models.QuotaExecute{}).
		Where("strategy_id = ? AND user_id = ? AND status IN (?) AND batch_number = ?",
			strategyID, userID, []string{"completed", "processing"}, batchNumber).
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

	// If less than 30 days until end of month, set expiry to end of next month
	endOfMonth := time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, now.Location())
	if endOfMonth.Sub(now).Hours() < 24*30 {
		expiryDate = time.Date(now.Year(), now.Month()+2, 0, 23, 59, 59, 0, now.Location())
	} else {
		expiryDate = endOfMonth
	}

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
	err := s.quotaService.AddQuotaForStrategy(user.ID, strategy.Amount, strategy.Name)
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
		zap.Int("amount", strategy.Amount),
		zap.String("model", strategy.Model),
		zap.Time("expiry_date", expiryDate))

	return nil
}

// generateBatchNumber generates batch number with second precision
func (s *StrategyService) generateBatchNumber() string {
	now := time.Now().Truncate(time.Second)
	return now.Format("20060102150405") // YearMonthDayHourMinuteSecond
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
	return s.db.Delete(&models.QuotaStrategy{}, id).Error
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

// ShouldExecutePeriodicForTest is a public wrapper for shouldExecutePeriodic for testing purposes
func (s *StrategyService) ShouldExecutePeriodicForTest(strategy *models.QuotaStrategy) bool {
	return s.shouldExecutePeriodic(strategy)
}
