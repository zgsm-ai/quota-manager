package services

import (
	"fmt"
	"strconv"
	"time"
	"quota-manager/internal/condition"
	"quota-manager/internal/database"
	"quota-manager/internal/models"
	"quota-manager/pkg/aigateway"
	"quota-manager/pkg/logger"

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

// Start 启动策略扫描服务
func (s *StrategyService) Start() error {
	// 添加每小时扫描一次的任务
	_, err := s.cron.AddFunc("0 * * * *", s.TraverseStrategy)
	if err != nil {
		return fmt.Errorf("failed to add cron job: %w", err)
	}

	s.cron.Start()
	logger.Info("Strategy service started")
	return nil
}

// Stop 停止策略扫描服务
func (s *StrategyService) Stop() {
	s.cron.Stop()
	logger.Info("Strategy service stopped")
}

// TraverseStrategy 遍历策略表
func (s *StrategyService) TraverseStrategy() {
	logger.Info("Starting strategy traversal")

	// 1. 获取用户列表
	users, err := s.loadUsers()
	if err != nil {
		logger.Error("Failed to load users", zap.Error(err))
		return
	}

	// 2. 获取策略列表
	strategies, err := s.loadStrategies()
	if err != nil {
		logger.Error("Failed to load strategies", zap.Error(err))
		return
	}

	// 3. 执行策略
	for _, strategy := range strategies {
		logger.Info("Processing strategy", zap.String("strategy", strategy.Name))

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

// loadUsers 加载用户列表
func (s *StrategyService) loadUsers() ([]models.UserInfo, error) {
	var users []models.UserInfo
	if err := s.db.Find(&users).Error; err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	return users, nil
}

// loadStrategies 加载策略列表
func (s *StrategyService) loadStrategies() ([]models.QuotaStrategy, error) {
	var strategies []models.QuotaStrategy
	if err := s.db.Find(&strategies).Error; err != nil {
		return nil, fmt.Errorf("failed to query strategies: %w", err)
	}
	return strategies, nil
}

// shouldExecutePeriodic 判断定时策略是否应该执行
func (s *StrategyService) shouldExecutePeriodic(strategy *models.QuotaStrategy) bool {
	if strategy.PeriodicExpr == "" {
		return false
	}

	// 解析cron表达式，判断是否应该执行
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

	// 如果下次执行时间在当前时间之前或相等，说明应该执行
	return next.Before(now) || next.Equal(now)
}

// ExecStrategy 执行策略
func (s *StrategyService) ExecStrategy(strategy *models.QuotaStrategy, users []models.UserInfo) {
	batchNumber := s.generateBatchNumber()

	for _, user := range users {
		// 对于single策略，检查是否已经执行过
		if strategy.Type == "single" {
			if s.hasExecuted(strategy.ID, user.ID) {
				continue
			}
		}

		// 检查条件
		match, err := condition.CalcCondition(&user, strategy.Condition, s.gateway)
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

		// 执行充值
		if err := s.executeRecharge(strategy, &user, batchNumber); err != nil {
			logger.Error("Failed to execute recharge",
				zap.String("user", user.ID),
				zap.String("strategy", strategy.Name),
				zap.Error(err))
		}
	}
}

// hasExecuted 检查单次策略是否已经执行过
func (s *StrategyService) hasExecuted(strategyID int, userID string) bool {
	var count int64
	err := s.db.Model(&models.QuotaExecute{}).
		Where("strategy_id = ? AND user = ? AND status = ?", strategyID, userID, "completed").
		Count(&count).Error

	if err != nil {
		logger.Error("Failed to check execution status", zap.Error(err))
		return false
	}

	return count > 0
}

// executeRecharge 执行充值
func (s *StrategyService) executeRecharge(strategy *models.QuotaStrategy, user *models.UserInfo, batchNumber string) error {
	// 1. 记录执行状态为进行中
	execute := &models.QuotaExecute{
		StrategyID:  strategy.ID,
		User:        user.ID,
		BatchNumber: batchNumber,
		Status:      "processing",
	}

	if err := s.db.Create(execute).Error; err != nil {
		return fmt.Errorf("failed to create execute record: %w", err)
	}

	// 2. 调用AiGateway进行充值
	err := s.gateway.DeltaQuota(user.ID, strategy.Amount)
	if err != nil {
		// 更新执行状态为失败
		s.db.Model(execute).Update("status", "failed")
		return fmt.Errorf("failed to recharge quota: %w", err)
	}

	// 3. 更新执行状态为已完成
	if err := s.db.Model(execute).Update("status", "completed").Error; err != nil {
		logger.Error("Failed to update execute status", zap.Error(err))
	}

	logger.Info("Recharge completed",
		zap.String("user", user.ID),
		zap.String("strategy", strategy.Name),
		zap.Int("amount", strategy.Amount),
		zap.String("model", strategy.Model))

	return nil
}

// generateBatchNumber 生成批次号
func (s *StrategyService) generateBatchNumber() string {
	now := time.Now()
	return now.Format("2006010215") // 年月日时
}

// CreateStrategy 创建策略
func (s *StrategyService) CreateStrategy(strategy *models.QuotaStrategy) error {
	if err := s.db.Create(strategy).Error; err != nil {
		return fmt.Errorf("failed to create strategy: %w", err)
	}
	return nil
}

// GetStrategies 获取策略列表
func (s *StrategyService) GetStrategies() ([]models.QuotaStrategy, error) {
	var strategies []models.QuotaStrategy
	if err := s.db.Find(&strategies).Error; err != nil {
		return nil, fmt.Errorf("failed to get strategies: %w", err)
	}
	return strategies, nil
}

// GetStrategy 获取单个策略
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

// UpdateStrategy 更新策略
func (s *StrategyService) UpdateStrategy(id int, updates map[string]interface{}) error {
	if err := s.db.Model(&models.QuotaStrategy{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update strategy: %w", err)
	}
	return nil
}

// DeleteStrategy 删除策略
func (s *StrategyService) DeleteStrategy(id int) error {
	if err := s.db.Delete(&models.QuotaStrategy{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete strategy: %w", err)
	}
	return nil
}