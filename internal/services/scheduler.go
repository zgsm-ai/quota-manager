package services

import (
	"quota-manager/internal/config"
	"quota-manager/pkg/logger"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// SchedulerService handles scheduled tasks
type SchedulerService struct {
	quotaService    *QuotaService
	strategyService *StrategyService
	config          *config.Config
	cron            *cron.Cron
}

// NewSchedulerService creates a new scheduler service
func NewSchedulerService(quotaService *QuotaService, strategyService *StrategyService, cfg *config.Config) *SchedulerService {
	return &SchedulerService{
		quotaService:    quotaService,
		strategyService: strategyService,
		config:          cfg,
		cron:            cron.New(cron.WithSeconds()),
	}
}

// Start starts the scheduler service
func (s *SchedulerService) Start() error {
	// Determine scan interval based on priority:
	// 1. Use configured scan_interval if present
	// 2. If not configured, use mode-based defaults
	var scanInterval string

	if s.config.Scheduler.ScanInterval != "" {
		// Use configured scan interval directly
		scanInterval = s.config.Scheduler.ScanInterval
		logger.Info("Using configured scan interval", zap.String("interval", scanInterval))
	} else {
		// Use mode-based defaults when scan_interval is not configured
		if s.config.Server.Mode == "debug" {
			scanInterval = "*/10 * * * * *" // Every 10 seconds in debug mode (6 fields with seconds)
			logger.Info("No scan interval configured, using debug mode default: every 10 seconds")
		} else {
			// Default for release mode or when mode is not configured
			scanInterval = "0 0 * * * *" // Every hour (6 fields with seconds)
			logger.Info("No scan interval configured, using default: every hour")
		}
	}

	// Add strategy scan task
	_, err := s.cron.AddFunc(scanInterval, s.strategyService.TraverseStrategy)
	if err != nil {
		logger.Error("Failed to add strategy scan task", zap.String("interval", scanInterval), zap.Error(err))
		return err
	}

	// Add quota expiry task - run at 00:01 on the first day of every month (6 fields with seconds)
	_, err = s.cron.AddFunc("0 1 0 1 * *", s.expireQuotasTask)
	if err != nil {
		logger.Error("Failed to add quota expiry task", zap.Error(err))
		return err
	}

	s.cron.Start()
	logger.Info("Scheduler service started",
		zap.String("scan_interval", scanInterval),
		zap.String("mode", s.config.Server.Mode))
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
