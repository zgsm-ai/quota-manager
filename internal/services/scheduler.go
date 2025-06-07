package services

import (
	"quota-manager/pkg/logger"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// SchedulerService handles scheduled tasks
type SchedulerService struct {
	quotaService    *QuotaService
	strategyService *StrategyService
	cron            *cron.Cron
}

// NewSchedulerService creates a new scheduler service
func NewSchedulerService(quotaService *QuotaService, strategyService *StrategyService) *SchedulerService {
	return &SchedulerService{
		quotaService:    quotaService,
		strategyService: strategyService,
		cron:            cron.New(),
	}
}

// Start starts the scheduler service
func (s *SchedulerService) Start() error {
	// Add strategy scan task - every hour
	_, err := s.cron.AddFunc("0 * * * *", s.strategyService.TraverseStrategy)
	if err != nil {
		return err
	}

	// Add quota expiry task - run at 00:01 on the first day of every month
	_, err = s.cron.AddFunc("1 0 1 * *", s.expireQuotasTask)
	if err != nil {
		return err
	}

	s.cron.Start()
	logger.Info("Scheduler service started")
	return nil
}

// Stop stops the scheduler service
func (s *SchedulerService) Stop() {
	s.cron.Stop()
	logger.Info("Scheduler service stopped")
}

// expireQuotasTask handles quota expiry task
func (s *SchedulerService) expireQuotasTask() {
	logger.Info("Starting quota expiry task")

	if err := s.quotaService.ExpireQuotas(); err != nil {
		logger.Error("Failed to expire quotas", zap.Error(err))
		return
	}

	logger.Info("Quota expiry task completed")
}
