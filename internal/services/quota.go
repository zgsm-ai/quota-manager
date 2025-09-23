package services

import (
	"errors"
	"fmt"
	"quota-manager/internal/config"
	"quota-manager/internal/database"
	"quota-manager/internal/models"
	"quota-manager/internal/utils"
	"quota-manager/pkg/aigateway"
	"quota-manager/pkg/logger"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// QuotaService handles quota-related operations
type QuotaService struct {
	db              *database.DB
	aiGatewayConf   *config.AiGatewayConfig
	configManager   *config.Manager
	aiGatewayClient *aigateway.Client
	voucherSvc      *VoucherService
}

// GetConfigManager returns the config manager
func (s *QuotaService) GetConfigManager() *config.Manager {
	return s.configManager
}

// NewQuotaService creates a new quota service
func NewQuotaService(db *database.DB, configManager *config.Manager, aiGatewayClient *aigateway.Client, voucherSvc *VoucherService) *QuotaService {
	return &QuotaService{
		db:              db,
		aiGatewayConf:   &configManager.GetDirect().AiGateway,
		configManager:   configManager,
		aiGatewayClient: aiGatewayClient,
		voucherSvc:      voucherSvc,
	}
}

// QuotaInfo represents user quota information
type QuotaInfo struct {
	TotalQuota float64           `json:"total_quota"`
	UsedQuota  float64           `json:"used_quota"`
	QuotaList  []QuotaDetailItem `json:"quota_list"`
	IsStar     string            `json:"is_star,omitempty"`
}

// QuotaDetailItem represents quota detail item
type QuotaDetailItem struct {
	Amount     float64   `json:"amount"`
	ExpiryDate time.Time `json:"expiry_date"`
}

// QuotaAuditRecord represents quota audit record
type QuotaAuditRecord struct {
	Amount       float64                   `json:"amount"`
	Operation    string                    `json:"operation"`
	VoucherCode  string                    `json:"voucher_code,omitempty"`
	RelatedUser  string                    `json:"related_user,omitempty"`
	StrategyName string                    `json:"strategy_name,omitempty"`
	ExpiryDate   time.Time                 `json:"expiry_date"`
	Details      *models.QuotaAuditDetails `json:"details,omitempty"`
	CreateTime   time.Time                 `json:"create_time"`
}

// TransferOutRequest represents transfer out request
type TransferOutRequest struct {
	ReceiverID string              `json:"receiver_id" validate:"required,uuid"`
	QuotaList  []TransferQuotaItem `json:"quota_list" validate:"required,min=1,dive"`
}

// TransferQuotaItem represents quota item for transfer
type TransferQuotaItem struct {
	Amount     float64   `json:"amount" validate:"required,gt=0"`
	ExpiryDate time.Time `json:"expiry_date" validate:"required"`
}

// TransferOutResponse represents transfer out response
type TransferOutResponse struct {
	VoucherCode string              `json:"voucher_code"`
	RelatedUser string              `json:"related_user"`
	Operation   string              `json:"operation"`
	QuotaList   []TransferQuotaItem `json:"quota_list"`
}

// TransferInRequest represents transfer in request
type TransferInRequest struct {
	VoucherCode string `json:"voucher_code" validate:"required,min=10,max=2000"`
}

// TransferStatus represents the transfer status
type TransferStatus string

const (
	TransferStatusSuccess         TransferStatus = "SUCCESS"
	TransferStatusPartialSuccess  TransferStatus = "PARTIAL_SUCCESS"
	TransferStatusFailed          TransferStatus = "FAILED"
	TransferStatusAlreadyRedeemed TransferStatus = "ALREADY_REDEEMED"
)

// TransferFailureReason represents the reason for transfer failure
type TransferFailureReason string

const (
	TransferFailureReasonExpired TransferFailureReason = "EXPIRED"
	TransferFailureReasonPending TransferFailureReason = "PENDING"
)

// TransferInResponse represents transfer in response
type TransferInResponse struct {
	GiverID     string                `json:"giver_id"`
	GiverName   string                `json:"giver_name"`
	GiverPhone  string                `json:"giver_phone"`
	GiverGithub string                `json:"giver_github"`
	ReceiverID  string                `json:"receiver_id"`
	QuotaList   []TransferQuotaResult `json:"quota_list"`
	VoucherCode string                `json:"voucher_code"`
	Operation   string                `json:"operation"`
	Amount      float64               `json:"amount"`
	Status      TransferStatus        `json:"status"`
	Message     string                `json:"message,omitempty"`
}

// TransferQuotaResult represents transfer quota result
type TransferQuotaResult struct {
	Amount        float64                `json:"amount"`
	ExpiryDate    time.Time              `json:"expiry_date"`
	IsExpired     bool                   `json:"is_expired"`
	Success       bool                   `json:"success"`
	FailureReason *TransferFailureReason `json:"failure_reason,omitempty"`
}

// MergeQuotaRequest represents merge quota request
type MergeQuotaRequest struct {
	MainUserID  string `json:"main_user_id" validate:"required,uuid"`  // 主用户（保留用户）
	OtherUserID string `json:"other_user_id" validate:"required,uuid"` // 其他用户（被合并的用户）
}

// MergeQuotaResponse represents merge quota response
type MergeQuotaResponse struct {
	MainUserID  string  `json:"main_user_id"`
	OtherUserID string  `json:"other_user_id"`
	Amount      float64 `json:"amount"`
	Operation   string  `json:"operation"`
	Status      string  `json:"status"`
	Message     string  `json:"message,omitempty"`
}

// GetUserQuota retrieves user quota information
func (s *QuotaService) GetUserQuota(userID string) (*QuotaInfo, error) {
	// Get total quota from AiGateway
	totalQuota, err := s.aiGatewayClient.QueryQuotaValue(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get total quota: %w", err)
	}

	// Get used quota from AiGateway
	usedQuota, err := s.aiGatewayClient.QueryUsedQuotaValue(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get used quota: %w", err)
	}

	// Get quota list from database
	var quotas []models.Quota
	if err := s.db.DB.Where("user_id = ? AND status = ?", userID, models.StatusValid).
		Order("expiry_date ASC").Find(&quotas).Error; err != nil {
		return nil, fmt.Errorf("failed to get quota list: %w", err)
	}

	// Calculate remaining quotas considering used quota
	quotaList := make([]QuotaDetailItem, 0)
	remainingUsed := usedQuota

	for _, quota := range quotas {
		if remainingUsed <= 0 {
			// No more used quota to deduct
			quotaList = append(quotaList, QuotaDetailItem{
				Amount:     quota.Amount,
				ExpiryDate: quota.ExpiryDate,
			})
		} else if quota.Amount > remainingUsed {
			// This quota is partially consumed
			quotaList = append(quotaList, QuotaDetailItem{
				Amount:     quota.Amount - remainingUsed,
				ExpiryDate: quota.ExpiryDate,
			})
			remainingUsed = 0
		} else {
			// This quota is fully consumed
			remainingUsed -= quota.Amount
		}
	}

	// checkGithubStar checks if user has starred the required GitHub repository
	if s.configManager.GetDirect().GithubStarCheck.Enabled {
		// Get giver's starred projects from database
		var giverGithubStar string
		var userInfo models.UserInfo
		if err := s.db.AuthDB.Where("id = ?", userID).First(&userInfo).Error; err == nil {
			// Store all starred projects as comma-separated string
			giverGithubStar = userInfo.GithubStar
		}

		isStar := "false"
		// Parse comma-separated starred projects
		starredProjects := strings.Split(giverGithubStar, ",")

		// Check if required repo is starred
		requiredRepo := strings.TrimSpace(s.configManager.GetDirect().GithubStarCheck.RequiredRepo)
		for _, project := range starredProjects {
			project = strings.TrimSpace(project)
			if project == requiredRepo {
				isStar = "true"
			}
		}
		return &QuotaInfo{
			TotalQuota: totalQuota,
			UsedQuota:  usedQuota,
			QuotaList:  quotaList,
			IsStar:     isStar,
		}, nil
	}

	return &QuotaInfo{
		TotalQuota: totalQuota,
		UsedQuota:  usedQuota,
		QuotaList:  quotaList,
	}, nil
}

// GetQuotaAuditRecords retrieves quota audit records
func (s *QuotaService) GetQuotaAuditRecords(userID string, page, pageSize int) ([]QuotaAuditRecord, int64, error) {
	var records []models.QuotaAudit
	var total int64

	offset := (page - 1) * pageSize

	// Get total count
	if err := s.db.DB.Model(&models.QuotaAudit{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count audit records: %w", err)
	}

	// Get records with pagination
	if err := s.db.DB.Where("user_id = ?", userID).
		Order("create_time DESC, id DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&records).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get audit records: %w", err)
	}

	result := make([]QuotaAuditRecord, len(records))
	for i, record := range records {
		// Parse details if available
		var details *models.QuotaAuditDetails
		if record.Details != "" {
			parsedDetails, err := record.UnmarshalDetails()
			if err == nil {
				details = parsedDetails
			}
		}

		result[i] = QuotaAuditRecord{
			Amount:       record.Amount,
			Operation:    record.Operation,
			VoucherCode:  record.VoucherCode,
			RelatedUser:  record.RelatedUser,
			StrategyName: record.StrategyName,
			ExpiryDate:   record.ExpiryDate,
			Details:      details,
			CreateTime:   record.CreateTime,
		}
	}

	return result, total, nil
}

// TransferOut handles quota transfer out
func (s *QuotaService) TransferOut(giver *models.AuthUser, req *TransferOutRequest) (*TransferOutResponse, error) {
	// Check if receiver_id is empty
	if req.ReceiverID == "" {
		return nil, fmt.Errorf("receiver_id cannot be empty")
	}

	// Get used quota from AiGateway to check availability
	usedQuota, err := s.aiGatewayClient.QueryUsedQuotaValue(giver.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get used quota: %w", err)
	}

	// Get quota list ordered by expiry date to check availability
	var quotas []models.Quota
	if err := s.db.DB.Where("user_id = ? AND status = ?", giver.ID, models.StatusValid).
		Order("expiry_date ASC").Find(&quotas).Error; err != nil {
		return nil, fmt.Errorf("failed to get quota list: %w", err)
	}

	// Calculate remaining quotas for each expiry date
	quotaAvailabilityMap := make(map[string]float64) // key: expiry_date as string, value: available amount
	remainingUsed := usedQuota

	for _, quota := range quotas {
		dateKey := quota.ExpiryDate.Format("2006-01-02T15:04:05Z07:00")
		var availableFromThisQuota float64
		if remainingUsed <= 0 {
			availableFromThisQuota = quota.Amount
		} else if quota.Amount > remainingUsed {
			availableFromThisQuota = quota.Amount - remainingUsed
			remainingUsed = 0
		} else {
			availableFromThisQuota = 0
			remainingUsed -= quota.Amount
		}

		// Add to existing amount for the same expiry date (accumulate instead of overwriting)
		quotaAvailabilityMap[dateKey] += availableFromThisQuota
	}

	// Start transaction
	tx := s.db.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Validate quota availability for each requested quota
	for _, quotaItem := range req.QuotaList {
		dateKey := quotaItem.ExpiryDate.Format("2006-01-02T15:04:05Z07:00")
		available, exists := quotaAvailabilityMap[dateKey]
		if !exists {
			tx.Rollback()
			return nil, fmt.Errorf("quota not found for expiry date %v", quotaItem.ExpiryDate)
		}

		if available < quotaItem.Amount {
			tx.Rollback()
			return nil, fmt.Errorf("insufficient available quota for expiry date %v: have %g, need %g",
				quotaItem.ExpiryDate, available, quotaItem.Amount)
		}

		// Also validate the total quota exists in database for this expiry date
		var totalQuotaAmount float64
		if err := tx.Model(&models.Quota{}).
			Where("user_id = ? AND expiry_date = ? AND status = ?",
				giver.ID, quotaItem.ExpiryDate, models.StatusValid).
			Select("COALESCE(SUM(amount), 0)").
			Scan(&totalQuotaAmount).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to check quota for expiry date %v: %w", quotaItem.ExpiryDate, err)
		}

		if totalQuotaAmount < quotaItem.Amount {
			tx.Rollback()
			return nil, fmt.Errorf("insufficient quota for expiry date %v: have %f, need %f",
				quotaItem.ExpiryDate, totalQuotaAmount, quotaItem.Amount)
		}
	}

	// Generate voucher code
	voucherQuotaList := make([]VoucherQuotaItem, len(req.QuotaList))
	for i, item := range req.QuotaList {
		voucherQuotaList[i] = VoucherQuotaItem{
			Amount:     item.Amount,
			ExpiryDate: item.ExpiryDate,
		}
	}

	// Get giver's starred projects from database
	var giverGithubStar string
	var userInfo models.UserInfo
	if err := s.db.AuthDB.Where("id = ?", giver.ID).First(&userInfo).Error; err == nil {
		// Store all starred projects as comma-separated string
		giverGithubStar = userInfo.GithubStar
	}

	// Clean receiver_id to remove leading/trailing whitespace before generating voucher
	cleanReceiverID := strings.TrimSpace(req.ReceiverID)

	voucherData := &VoucherData{
		GiverID:         giver.ID,
		GiverName:       giver.Name,
		GiverPhone:      giver.Phone,
		GiverGithub:     giver.Github,
		GiverGithubStar: giverGithubStar, // Now stores comma-separated list of starred projects
		ReceiverID:      cleanReceiverID,
		QuotaList:       voucherQuotaList,
	}

	voucherCode, err := s.voucherSvc.GenerateVoucher(voucherData)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to generate voucher: %w", err)
	}

	// Update quota table - reduce giver's quota
	for _, quotaItem := range req.QuotaList {
		if err := tx.Model(&models.Quota{}).
			Where("user_id = ? AND expiry_date = ? AND status = ?",
				giver.ID, quotaItem.ExpiryDate, models.StatusValid).
			Update("amount", gorm.Expr("amount - ?", quotaItem.Amount)).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to update quota: %w", err)
		}

		// Delete quota records with zero or negative amounts
		if err := tx.Where("user_id = ? AND expiry_date = ? AND status = ? AND amount <= 0",
			giver.ID, quotaItem.ExpiryDate, models.StatusValid).Delete(&models.Quota{}).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to delete zero quota records: %w", err)
		}
	}

	// Calculate total amount for audit record
	totalAmount := 0.0
	// Find earliest expiry date for audit record
	var earliestExpiryDate time.Time
	for i, item := range req.QuotaList {
		totalAmount += item.Amount
		if i == 0 || item.ExpiryDate.Before(earliestExpiryDate) {
			earliestExpiryDate = item.ExpiryDate
		}
	}

	// Prepare detailed audit information
	auditDetails := &models.QuotaAuditDetails{
		Operation: models.OperationTransferOut,
		Summary: models.QuotaAuditSummary{
			TotalAmount:        totalAmount,
			TotalItems:         len(req.QuotaList),
			SuccessfulItems:    len(req.QuotaList), // All items are successful in transfer out
			EarliestExpiryDate: earliestExpiryDate.Format(time.RFC3339),
		},
		Items: make([]models.QuotaAuditDetailItem, len(req.QuotaList)),
	}

	// Record each quota item detail
	for i, item := range req.QuotaList {
		auditDetails.Items[i] = models.QuotaAuditDetailItem{
			Amount:     item.Amount,
			ExpiryDate: item.ExpiryDate.Format(time.RFC3339),
			Status:     models.AuditStatusSuccess,
		}
	}

	// Record audit log
	auditRecord := &models.QuotaAudit{
		UserID:      giver.ID,
		Amount:      -totalAmount,
		Operation:   models.OperationTransferOut,
		VoucherCode: voucherCode,
		RelatedUser: cleanReceiverID,
		ExpiryDate:  earliestExpiryDate, // Use earliest expiry date for audit
		// StrategyName is empty for transfer operations
	}
	if err := auditRecord.MarshalDetails(auditDetails); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to marshal audit details: %w", err)
	}
	if err := tx.Create(auditRecord).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create audit record: %w", err)
	}

	// Update AiGateway quota
	if err := s.aiGatewayClient.DeltaQuota(giver.ID, -totalAmount); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update AiGateway quota: %w", err)
	}

	tx.Commit()

	return &TransferOutResponse{
		VoucherCode: voucherCode,
		RelatedUser: cleanReceiverID,
		Operation:   models.OperationTransferOut,
		QuotaList:   req.QuotaList,
	}, nil
}

// TransferIn handles quota transfer in
func (s *QuotaService) TransferIn(receiver *models.AuthUser, req *TransferInRequest) (*TransferInResponse, error) {
	// Validate voucher
	voucherData, err := s.voucherSvc.ValidateAndDecodeVoucher(req.VoucherCode)
	if err != nil {
		return &TransferInResponse{
			Status:  TransferStatusFailed,
			Message: "Invalid voucher code",
		}, nil
	}

	// Check if voucher is for the correct receiver
	if voucherData.ReceiverID != receiver.ID {
		return &TransferInResponse{
			Status:  TransferStatusFailed,
			Message: "Voucher is not for this user",
		}, nil
	}

	// Check if voucher has already been redeemed
	var existingRedemption models.VoucherRedemption
	if err := s.db.DB.Where("voucher_code = ?", req.VoucherCode).First(&existingRedemption).Error; err == nil {
		return &TransferInResponse{
			GiverID:     voucherData.GiverID,
			GiverName:   voucherData.GiverName,
			GiverPhone:  voucherData.GiverPhone,
			GiverGithub: voucherData.GiverGithub,
			ReceiverID:  receiver.ID,
			VoucherCode: req.VoucherCode,
			Operation:   models.OperationTransferIn,
			Status:      TransferStatusAlreadyRedeemed,
			Message:     "Voucher has already been redeemed",
		}, nil
	}

	// Start transaction
	tx := s.db.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Record redemption to prevent duplicate usage
	redemption := &models.VoucherRedemption{
		VoucherCode: req.VoucherCode,
		ReceiverID:  receiver.ID,
	}
	if err := tx.Create(redemption).Error; err != nil {
		tx.Rollback()
		return &TransferInResponse{
			Status:  TransferStatusFailed,
			Message: "Failed to record voucher redemption",
		}, nil
	}

	totalAmount := 0.0
	successCount := 0
	quotaResults := make([]TransferQuotaResult, len(voucherData.QuotaList))
	var earliestExpiryDate time.Time
	hasValidQuota := false

	// Process quota transfer
	for i, quotaItem := range voucherData.QuotaList {
		isExpired := utils.NowInConfigTimezone(s.configManager.GetDirect()).Truncate(time.Second).After(quotaItem.ExpiryDate.Truncate(time.Second))

		quotaResult := TransferQuotaResult{
			Amount:     quotaItem.Amount,
			ExpiryDate: quotaItem.ExpiryDate,
			IsExpired:  isExpired,
			Success:    false,
		}

		// Only process valid quota
		if !isExpired {
			var existingQuota models.Quota
			if err := tx.Where("user_id = ? AND expiry_date = ? AND status = ?",
				receiver.ID, quotaItem.ExpiryDate, models.StatusValid).First(&existingQuota).Error; err != nil {
				// Create new quota record
				newQuota := &models.Quota{
					UserID:     receiver.ID,
					Amount:     quotaItem.Amount,
					ExpiryDate: quotaItem.ExpiryDate,
					Status:     models.StatusValid,
				}
				if err := tx.Create(newQuota).Error; err != nil {
					// Individual quota creation failed, mark as pending
					reason := TransferFailureReasonPending
					quotaResult.FailureReason = &reason
				} else {
					quotaResult.Success = true
					successCount++
					totalAmount += quotaItem.Amount

					// Track earliest expiry date for valid quota
					if !hasValidQuota || quotaItem.ExpiryDate.Before(earliestExpiryDate) {
						earliestExpiryDate = quotaItem.ExpiryDate
						hasValidQuota = true
					}
				}
			} else {
				// Update existing quota
				if err := tx.Model(&existingQuota).Update("amount", existingQuota.Amount+quotaItem.Amount).Error; err != nil {
					// Individual quota update failed, mark as pending
					reason := TransferFailureReasonPending
					quotaResult.FailureReason = &reason
				} else {
					quotaResult.Success = true
					successCount++
					totalAmount += quotaItem.Amount

					// Track earliest expiry date for valid quota
					if !hasValidQuota || quotaItem.ExpiryDate.Before(earliestExpiryDate) {
						earliestExpiryDate = quotaItem.ExpiryDate
						hasValidQuota = true
					}
				}
			}
		} else {
			// Mark expired quota
			reason := TransferFailureReasonExpired
			quotaResult.FailureReason = &reason
		}

		quotaResults[i] = quotaResult
	}

	// Record audit log for valid quota only if there's valid quota
	if hasValidQuota {
		// Prepare detailed audit information
		expiredCount := 0
		failedCount := 0
		auditDetails := &models.QuotaAuditDetails{
			Operation: models.OperationTransferIn,
			Items:     make([]models.QuotaAuditDetailItem, len(quotaResults)),
		}

		// Record each quota item detail
		for i, result := range quotaResults {
			item := models.QuotaAuditDetailItem{
				Amount:     result.Amount,
				ExpiryDate: result.ExpiryDate.Format(time.RFC3339),
			}

			if result.IsExpired {
				item.Status = models.AuditStatusExpired
				item.FailureReason = "Quota expired"
				expiredCount++
			} else if result.Success {
				item.Status = models.AuditStatusSuccess
			} else {
				item.Status = models.AuditStatusFailed
				if result.FailureReason != nil {
					item.FailureReason = string(*result.FailureReason)
				}
				failedCount++
			}

			auditDetails.Items[i] = item
		}

		auditDetails.Summary = models.QuotaAuditSummary{
			TotalAmount:        totalAmount,
			TotalItems:         len(voucherData.QuotaList),
			SuccessfulItems:    successCount,
			FailedItems:        failedCount,
			ExpiredItems:       expiredCount,
			EarliestExpiryDate: earliestExpiryDate.Format(time.RFC3339),
		}

		auditRecord := &models.QuotaAudit{
			UserID:      receiver.ID,
			Amount:      totalAmount,
			Operation:   models.OperationTransferIn,
			VoucherCode: req.VoucherCode,
			RelatedUser: voucherData.GiverID,
			ExpiryDate:  earliestExpiryDate, // Use earliest expiry date from valid quota
			// StrategyName is empty for transfer operations
		}
		if err := auditRecord.MarshalDetails(auditDetails); err != nil {
			tx.Rollback()
			return &TransferInResponse{
				Status:  TransferStatusFailed,
				Message: "Failed to marshal audit details",
			}, nil
		}
		if err := tx.Create(auditRecord).Error; err != nil {
			tx.Rollback()
			return &TransferInResponse{
				Status:  TransferStatusFailed,
				Message: "Failed to create audit record",
			}, nil
		}
	}

	// Update AiGateway quota only for valid quota
	if totalAmount > 0 {
		if err := s.aiGatewayClient.DeltaQuota(receiver.ID, totalAmount); err != nil {
			tx.Rollback()
			return &TransferInResponse{
				Status:  TransferStatusFailed,
				Message: "Failed to update AiGateway quota",
			}, nil
		}
	}

	// Check and handle GitHub star status if giver has starred projects
	if voucherData.GiverGithubStar != "" && s.aiGatewayClient != nil {
		// If giver has starred projects, set starred projects in AiGateway for receiver
		// This is best effort - we don't want to fail the transfer if AI Gateway call fails
		if err := s.aiGatewayClient.SetGithubStarProjects(receiver.ID, voucherData.GiverGithubStar); err != nil {
			logger.Warn("Failed to set GitHub star projects in AiGateway",
				zap.String("user_id", receiver.ID),
				zap.String("starred_projects", voucherData.GiverGithubStar),
				zap.Error(err))
		}
	}

	tx.Commit()

	// Determine overall transfer status
	var status TransferStatus
	var message string
	totalQuotas := len(voucherData.QuotaList)
	expiredCount := 0
	for _, result := range quotaResults {
		if result.IsExpired {
			expiredCount++
		}
	}

	if successCount == 0 {
		status = TransferStatusFailed
		message = "All quota transfers failed"
	} else if successCount == totalQuotas {
		status = TransferStatusSuccess
		message = "All quota transfers completed successfully"
	} else if successCount > 0 && expiredCount > 0 {
		status = TransferStatusPartialSuccess
		message = fmt.Sprintf("%d of %d quota transfers completed successfully, %d expired", successCount, totalQuotas, expiredCount)
	} else {
		status = TransferStatusPartialSuccess
		message = fmt.Sprintf("%d of %d quota transfers completed successfully", successCount, totalQuotas)
	}

	return &TransferInResponse{
		GiverID:     voucherData.GiverID,
		GiverName:   voucherData.GiverName,
		GiverPhone:  voucherData.GiverPhone,
		GiverGithub: voucherData.GiverGithub,
		ReceiverID:  receiver.ID,
		QuotaList:   quotaResults,
		VoucherCode: req.VoucherCode,
		Operation:   models.OperationTransferIn,
		Amount:      totalAmount,
		Status:      status,
		Message:     message,
	}, nil
}

// AddQuotaForStrategy adds quota for strategy execution
func (s *QuotaService) AddQuotaForStrategy(userID string, amount float64, strategyID int, strategyName string, relatedUserID *string) error {
	now := utils.NowInConfigTimezone(s.configManager.GetDirect()).Truncate(time.Second)

	// Get strategy information to determine expiry date
	var strategy models.QuotaStrategy
	if err := s.db.DB.First(&strategy, strategyID).Error; err != nil {
		return fmt.Errorf("failed to get strategy: %w", err)
	}

	// Calculate expiry date based on strategy's ExpiryDays
	expiryDate := utils.CalculateExpiryDate(now, strategy.ExpiryDays)

	// Start transaction
	tx := s.db.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Add or update quota
	var quota models.Quota
	err := tx.Where("user_id = ? AND expiry_date = ? AND status = ?",
		userID, expiryDate, models.StatusValid).First(&quota).Error

	if err == gorm.ErrRecordNotFound {
		// Create new quota record
		quota = models.Quota{
			UserID:     userID,
			Amount:     amount,
			ExpiryDate: expiryDate,
			Status:     models.StatusValid,
		}
		if err := tx.Create(&quota).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create quota: %w", err)
		}
	} else if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to query quota: %w", err)
	} else {
		// Update existing quota
		if err := tx.Model(&quota).Update("amount", quota.Amount+amount).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to update quota: %w", err)
		}
	}

	// Prepare detailed audit information for recharge
	auditDetails := &models.QuotaAuditDetails{
		Operation: models.OperationRecharge,
		Summary: models.QuotaAuditSummary{
			TotalAmount:        amount,
			TotalItems:         1,
			SuccessfulItems:    1,
			EarliestExpiryDate: expiryDate.Format(time.RFC3339),
		},
		Items: []models.QuotaAuditDetailItem{
			{
				Amount:        amount,
				ExpiryDate:    expiryDate.Format(time.RFC3339),
				Status:        models.AuditStatusSuccess,
				OriginalQuota: quota.Amount - amount, // Before recharge
				NewQuota:      quota.Amount,          // After recharge
			},
		},
	}

	// Add strategy information if available
	if strategyName != "" {
		auditDetails.Items[0].FailureReason = fmt.Sprintf("Strategy: %s", strategyName)
	}

	// Record audit log only if it's not expired yet
	auditRecord := &models.QuotaAudit{
		UserID:       userID,
		Amount:       amount,
		Operation:    models.OperationRecharge,
		StrategyID:   &strategyID,
		StrategyName: strategyName,
		ExpiryDate:   expiryDate,
	}
	// Add related user if available
	if relatedUserID != nil {
		auditRecord.RelatedUser = *relatedUserID
	}
	if err := auditRecord.MarshalDetails(auditDetails); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to marshal audit details: %w", err)
	}
	if err := tx.Create(auditRecord).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create audit record: %w", err)
	}

	// Update AiGateway quota
	if err := s.aiGatewayClient.DeltaQuota(userID, amount); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update AiGateway quota: %w", err)
	}

	tx.Commit()
	return nil
}

// ExpireQuotas expires quotas and synchronizes with AiGateway
func (s *QuotaService) ExpireQuotas() error {
	now := utils.NowInConfigTimezone(s.configManager.GetDirect()).Truncate(time.Second)

	// Step 1: Record monthly used quota (before finding expired but still valid quotas)
	logger.Info("Step 1: Recording monthly used quota before expiry processing")
	if err := s.recordMonthlyUsedQuota(now); err != nil {
		logger.Error("Failed to record monthly used quota", zap.Error(err))
		// Note: Do not return error directly here, continue with expiry processing but log the error
		// because monthly quota recording failure should not affect the main quota expiry functionality
	}

	// Step 2: Find expired but still valid quotas (original logic)
	logger.Info("Step 2: Finding expired but still valid quotas")
	var expiredQuotas []models.Quota
	if err := s.db.DB.Where("status = ? AND expiry_date < ?", models.StatusValid, now).Find(&expiredQuotas).Error; err != nil {
		return fmt.Errorf("failed to find expired quotas: %w", err)
	}

	if len(expiredQuotas) == 0 {
		return nil
	}

	// Group by user
	userQuotaMap := make(map[string]float64)
	for _, quota := range expiredQuotas {
		userQuotaMap[quota.UserID] += quota.Amount
	}

	// Start transaction
	tx := s.db.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Update status to expired
	if err := tx.Model(&models.Quota{}).
		Where("status = ? AND expiry_date < ?", models.StatusValid, now).
		Update("status", models.StatusExpired).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update quota status: %w", err)
	}

	// Process each user
	for userID, expiredAmount := range userQuotaMap {
		// Get user's remaining valid quota
		var validQuotaSum float64
		if err := tx.Model(&models.Quota{}).
			Where("user_id = ? AND status = ?", userID, models.StatusValid).
			Select("COALESCE(SUM(amount), 0)").Scan(&validQuotaSum).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to calculate valid quota for user %s: %w", userID, err)
		}

		// Get current quota info from AiGateway
		totalQuota, err := s.aiGatewayClient.QueryQuotaValue(userID)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to get total quota from AiGateway for user %s: %w", userID, err)
		}

		usedQuota, err := s.aiGatewayClient.QueryUsedQuotaValue(userID)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to get used quota from AiGateway for user %s: %w", userID, err)
		}

		remainingQuota := totalQuota - usedQuota

		// Reset used quota first
		if err := s.aiGatewayClient.DeltaUsedQuota(userID, -usedQuota); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to reset used quota for user %s: %w", userID, err)
		}

		// Adjust total quota
		validQuota := validQuotaSum
		var newTotalQuota float64
		if validQuota >= remainingQuota {
			newTotalQuota = remainingQuota
		} else {
			newTotalQuota = validQuota
		}

		deltaQuota := newTotalQuota - totalQuota
		if deltaQuota != 0 {
			if err := s.aiGatewayClient.DeltaQuota(userID, deltaQuota); err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to adjust total quota for user %s: %w", userID, err)
			}
		}

		// Create audit record for quota expiry
		auditRecord := &models.QuotaAudit{
			UserID:       userID,
			Amount:       -expiredAmount, // Negative amount for expiry
			Operation:    "EXPIRE",
			StrategyName: "Credit 到期失效",
			ExpiryDate:   now, // Use current time as expiry time
			CreateTime:   utils.NowInConfigTimezone(s.configManager.GetDirect()),
		}
		if err := tx.Create(auditRecord).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create expiry audit record for user %s: %w", userID, err)
		}
	}

	tx.Commit()
	return nil
}

// MergeQuotaRecords merges quota records for the same user and expiry date
func (s *QuotaService) MergeQuotaRecords() error {
	// QuotaGroup represents quota records grouped by user and expiry date
	type QuotaGroup struct {
		UserID      string    `gorm:"column:user_id"`
		ExpiryDate  time.Time `gorm:"column:expiry_date"`
		Status      string    `gorm:"column:status"`
		TotalAmount float64   `gorm:"column:total_amount"`
		RecordCount int       `gorm:"column:record_count"`
	}

	// Find groups with multiple records
	var groups []QuotaGroup
	result := s.db.DB.Model(&models.Quota{}).
		Select("user_id, expiry_date, status, SUM(amount) as total_amount, COUNT(*) as record_count").
		Group("user_id, expiry_date, status").
		Having("COUNT(*) > 1").
		Scan(&groups)

	if result.Error != nil {
		return fmt.Errorf("failed to find quota groups: %w", result.Error)
	}

	// Start transaction
	tx := s.db.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Process each group that has duplicates
	for _, group := range groups {
		// Delete all existing records for this group
		if err := tx.Where("user_id = ? AND expiry_date = ? AND status = ?",
			group.UserID, group.ExpiryDate, group.Status).Delete(&models.Quota{}).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to delete duplicate quota records: %w", err)
		}

		// Create a single merged record (only if total amount is positive)
		if group.TotalAmount > 0 {
			mergedQuota := &models.Quota{
				UserID:     group.UserID,
				Amount:     group.TotalAmount,
				ExpiryDate: group.ExpiryDate,
				Status:     group.Status,
			}
			if err := tx.Create(mergedQuota).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to create merged quota record: %w", err)
			}
		}
	}

	tx.Commit()
	return nil
}

// GetUserQuotaAuditRecords gets quota audit records for a specific user (admin function)
func (s *QuotaService) GetUserQuotaAuditRecords(userID string, page, pageSize int) ([]QuotaAuditRecord, int64, error) {
	var auditRecords []models.QuotaAudit
	var total int64

	// Get total count
	if err := s.db.Model(&models.QuotaAudit{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count quota audit records: %w", err)
	}

	// Get records with pagination
	offset := (page - 1) * pageSize
	if err := s.db.Where("user_id = ?", userID).
		Order("create_time DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&auditRecords).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to query quota audit records: %w", err)
	}

	// Convert to response format
	var records []QuotaAuditRecord
	for _, record := range auditRecords {
		auditRecord := QuotaAuditRecord{
			Amount:       record.Amount,
			Operation:    record.Operation,
			VoucherCode:  record.VoucherCode,
			RelatedUser:  record.RelatedUser,
			StrategyName: record.StrategyName,
			ExpiryDate:   record.ExpiryDate,
			CreateTime:   record.CreateTime,
		}

		// Unmarshal details if present
		if record.Details != "" {
			details, err := record.UnmarshalDetails()
			if err == nil {
				auditRecord.Details = details
			}
		}

		records = append(records, auditRecord)
	}

	return records, total, nil
}

// GetUsersWithValidQuota gets all users with valid quota
func (s *QuotaService) GetUsersWithValidQuota() ([]string, error) {
	var userIDs []string

	// Use DISTINCT to ensure each user appears only once
	if err := s.db.DB.Model(&models.Quota{}).
		Where("status = ?", models.StatusValid).
		Distinct("user_id").
		Find(&userIDs).Error; err != nil {
		return nil, fmt.Errorf("failed to get users with valid quota: %w", err)
	}

	return userIDs, nil
}

// recordUserMonthlyUsedQuota records monthly used quota for a single user
func (s *QuotaService) recordUserMonthlyUsedQuota(userID string, yearMonth string) error {
	// Get user's used quota from aigateway
	usedQuota, err := s.aiGatewayClient.QueryUsedQuotaValue(userID)
	if err != nil {
		logger.Warn("Failed to get used quota from aigateway",
			zap.String("user_id", userID),
			zap.Error(err))
		return fmt.Errorf("failed to get used quota for user %s: %w", userID, err)
	}

	// Do not record if used quota is 0 or does not exist
	if usedQuota <= 0 {
		logger.Info("Skip recording zero or negative used quota",
			zap.String("user_id", userID),
			zap.Float64("used_quota", usedQuota))
		return nil
	}

	// Create record
	record := &models.MonthlyQuotaUsage{
		UserID:     userID,
		YearMonth:  yearMonth,
		UsedQuota:  usedQuota,
		RecordTime: utils.NowInConfigTimezone(s.configManager.GetDirect()),
	}

	// Use ON CONFLICT to handle duplicate records
	if err := s.db.DB.Create(record).Error; err != nil {
		// If it's a unique constraint conflict, update the existing record
		if strings.Contains(err.Error(), "duplicate key") {
			if err := s.db.DB.Model(&models.MonthlyQuotaUsage{}).
				Where("user_id = ? AND year_month = ?", userID, yearMonth).
				Updates(map[string]interface{}{
					"used_quota":  usedQuota,
					"record_time": utils.NowInConfigTimezone(s.configManager.GetDirect()),
				}).Error; err != nil {
				return fmt.Errorf("failed to update monthly quota usage for user %s: %w", userID, err)
			}
			logger.Info("Updated existing monthly quota usage record",
				zap.String("user_id", userID),
				zap.String("year_month", yearMonth),
				zap.Float64("used_quota", usedQuota))
		} else {
			return fmt.Errorf("failed to create monthly quota usage record for user %s: %w", userID, err)
		}
	} else {
		logger.Info("Created monthly quota usage record",
			zap.String("user_id", userID),
			zap.String("year_month", yearMonth),
			zap.Float64("used_quota", usedQuota))
	}

	return nil
}

// SyncQuotasWithAiGateway synchronizes all users' quotas with AiGateway
func (s *QuotaService) SyncQuotasWithAiGateway() error {
	logger.Info("Starting quota sync task")

	// Step 1: Get all users with valid quotas from quota table
	userIDs, err := s.GetUsersWithValidQuota()
	if err != nil {
		return fmt.Errorf("failed to get users with valid quota: %w", err)
	}

	logger.Info("Found users with valid quota", zap.Int("user_count", len(userIDs)))

	// Step 2: Process each user
	for _, userID := range userIDs {
		if err := s.syncUserQuotaWithAiGateway(userID); err != nil {
			logger.Error("Failed to sync user quota",
				zap.String("user_id", userID),
				zap.Error(err))
			// Continue processing other users, don't interrupt the entire flow
			continue
		}
	}

	logger.Info("Quota sync task completed")
	return nil
}

// syncUserQuotaWithAiGateway synchronizes a single user's quota with AiGateway
func (s *QuotaService) syncUserQuotaWithAiGateway(userID string) error {
	// Step 2.1: Get total quota from AiGateway
	aigatewayTotalQuota, err := s.aiGatewayClient.QueryQuotaValue(userID)
	if err != nil {
		return fmt.Errorf("failed to get quota from AiGateway: %w", err)
	}

	// Step 2.2: Get total valid quota from quota table
	var totalValidQuota float64
	if err := s.db.DB.Model(&models.Quota{}).
		Where("user_id = ? AND status = ?", userID, models.StatusValid).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&totalValidQuota).Error; err != nil {
		return fmt.Errorf("failed to calculate user valid quota: %w", err)
	}

	// Step 2.3: If a != b, set the user's quota to b using AiGateway refresh interface
	if aigatewayTotalQuota != totalValidQuota {
		logger.Warn("Detected quota inconsistency, will sync",
			zap.String("user_id", userID),
			zap.Float64("aigateway_quota", aigatewayTotalQuota),
			zap.Float64("database_quota", totalValidQuota))

		// Use RefreshQuota for full quota setting
		if err := s.aiGatewayClient.RefreshQuota(userID, totalValidQuota); err != nil {
			return fmt.Errorf("failed to refresh AiGateway quota: %w", err)
		}

		logger.Info("Quota sync completed",
			zap.String("user_id", userID),
			zap.Float64("original_quota", aigatewayTotalQuota),
			zap.Float64("new_quota", totalValidQuota))
	} else {
		logger.Info("Quota is consistent, no sync needed",
			zap.String("user_id", userID),
			zap.Float64("quota_value", aigatewayTotalQuota))
	}

	return nil
}

// recordMonthlyUsedQuota records monthly used quota for all users
func (s *QuotaService) recordMonthlyUsedQuota(now time.Time) error {
	logger.Info("Starting to record monthly used quota")

	// Get last month's year-month in YYYY-MM format
	lastMonth := now.AddDate(0, -1, 0)
	yearMonth := lastMonth.Format("2006-01")

	// Get all users with valid quota
	userIDs, err := s.GetUsersWithValidQuota()
	if err != nil {
		return fmt.Errorf("failed to get users with valid quota: %w", err)
	}

	logger.Info("Found users with valid quota", zap.Int("count", len(userIDs)))

	// Process users in batch
	for _, userID := range userIDs {
		err := s.recordUserMonthlyUsedQuota(userID, yearMonth)
		if err != nil {
			logger.Error("Failed to record monthly used quota for user",
				zap.String("user_id", userID),
				zap.Error(err))
			// Continue processing other users without interrupting the entire process
		}
	}

	logger.Info("Monthly quota usage recording completed",
		zap.Int("total_users", len(userIDs)))

	return nil
}

// DeductQuota deducts quota from a user's account
func (s *QuotaService) DeductQuota(userID string, amount float64, reason, referenceID, model string) error {
	// Validate amount
	if amount <= 0 {
		return fmt.Errorf("amount must be positive")
	}

	// Get used quota from AiGateway
	usedQuota, err := s.aiGatewayClient.QueryUsedQuotaValue(userID)
	if err != nil {
		return fmt.Errorf("failed to get used quota: %w", err)
	}

	// Start database transaction
	tx := s.db.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Get all valid quotas for the user, ordered by expiry date
	var quotas []models.Quota
	if err := tx.Where("user_id = ? AND status = ?", userID, models.StatusValid).
		Order("expiry_date ASC").Find(&quotas).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to get quota list: %w", err)
	}

	// Calculate total available quota
	totalDatabaseQuota := 0.0
	for _, quota := range quotas {
		totalDatabaseQuota += quota.Amount
	}

	availableQuota := totalDatabaseQuota - usedQuota
	if availableQuota < amount {
		tx.Rollback()
		return fmt.Errorf("insufficient quota: available %g, needed %g", availableQuota, amount)
	}

	// Deduct the amount from quotas, starting from earliest expiry date
	remainingDeduct := amount
	updatedQuotas := make([]*models.Quota, 0)
	deletedQuotaIDs := make([]int, 0)

	for _, quota := range quotas {
		if remainingDeduct <= 0 {
			break
		}

		// Calculate how much to deduct from this quota
		deductFromThis := min(remainingDeduct, quota.Amount)
		quota.Amount -= deductFromThis
		remainingDeduct -= deductFromThis

		if quota.Amount > 0 {
			// Update the quota record
			updatedQuotas = append(updatedQuotas, &quota)
		} else {
			// Mark for deletion if amount becomes zero or negative
			deletedQuotaIDs = append(deletedQuotaIDs, quota.ID)
		}
	}

	// Update quota records
	for _, quota := range updatedQuotas {
		if err := tx.Model(&models.Quota{}).Where("id = ?", quota.ID).
			Update("amount", quota.Amount).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to update quota: %w", err)
		}
	}

	// Delete zero amount quotas
	if len(deletedQuotaIDs) > 0 {
		if err := tx.Where("id IN ?", deletedQuotaIDs).Delete(&models.Quota{}).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to delete zero quota records: %w", err)
		}
	}

	// Find earliest expiry date for audit record
	var earliestExpiryDate time.Time
	if len(quotas) > 0 {
		earliestExpiryDate = quotas[0].ExpiryDate
		for _, quota := range quotas {
			if quota.ExpiryDate.Before(earliestExpiryDate) {
				earliestExpiryDate = quota.ExpiryDate
			}
		}
	} else {
		earliestExpiryDate = time.Now().AddDate(0, 0, 30) // Default to 30 days if no quotas
	}

	// Record audit log
	auditRecord := &models.QuotaAudit{
		UserID:     userID,
		Amount:     -amount, // Negative amount for deduction
		Operation:  models.OperationDeduct,
		ExpiryDate: earliestExpiryDate,
	}

	// Add reason, referenceID, and model to audit record if provided
	if reason != "" {
		auditRecord.StrategyName = reason // Use StrategyName field to store reason
	}
	if referenceID != "" {
		// Store referenceID in RelatedUser field
		auditRecord.RelatedUser = referenceID
	}

	// Prepare audit details
	auditDetails := &models.QuotaAuditDetails{
		Operation: models.OperationDeduct,
		Summary: models.QuotaAuditSummary{
			TotalAmount:        amount,
			TotalItems:         1,
			SuccessfulItems:    1,
			EarliestExpiryDate: earliestExpiryDate.Format(time.RFC3339),
		},
		Items: []models.QuotaAuditDetailItem{
			{
				Amount:     amount,
				ExpiryDate: earliestExpiryDate.Format(time.RFC3339),
				Status:     models.AuditStatusSuccess,
			},
		},
	}
	if err := auditRecord.MarshalDetails(auditDetails); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to marshal audit details: %w", err)
	}
	if err := tx.Create(auditRecord).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create audit record: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Update AiGateway quota
	if err := s.aiGatewayClient.DeltaQuota(userID, -amount); err != nil {
		// Log the error but don't fail the operation since database is already updated
		logger.Error("Failed to update AiGateway quota after deduction", zap.Error(err), zap.String("user_id", userID))
	}

	return nil
}

// MergeUserQuota merges all quota from other user to main user
func (s *QuotaService) MergeUserQuota(req *MergeQuotaRequest) (*MergeQuotaResponse, error) {
	// Validate request
	if req.MainUserID == req.OtherUserID {
		return &MergeQuotaResponse{
			MainUserID:  req.MainUserID,
			OtherUserID: req.OtherUserID,
			Amount:      0,
			Operation:   "MERGE_QUOTA",
			Status:      "FAILED",
			Message:     "Main user and other user cannot be the same",
		}, nil
	}

	// Start transaction
	tx := s.db.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Get all valid quotas from other user
	var otherUserQuotas []models.Quota
	if err := tx.Where("user_id = ? AND status = ? AND amount > 0", req.OtherUserID, models.StatusValid).
		Order("expiry_date ASC").Find(&otherUserQuotas).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to get other user quotas: %w", err)
	}

	// If no quotas found, return success with empty result
	if len(otherUserQuotas) == 0 {
		return &MergeQuotaResponse{
			MainUserID:  req.MainUserID,
			OtherUserID: req.OtherUserID,
			Amount:      0,
			Operation:   "MERGE_QUOTA",
			Status:      "SUCCESS",
			Message:     "No quotas found to merge",
		}, nil
	}

	var totalAmount float64

	// Calculate results from original quotas for audit and response
	for _, quota := range otherUserQuotas {
		totalAmount += quota.Amount
	}

	// Process each quota individually to handle potential conflicts
	for _, quota := range otherUserQuotas {
		// Check if main user already has a quota with same expiry_date and status
		var existingQuota models.Quota
		err := tx.Where("user_id = ? AND expiry_date = ? AND status = ?",
			req.MainUserID, quota.ExpiryDate, quota.Status).First(&existingQuota).Error

		if err == nil {
			// Main user already has a quota with same expiry_date and status
			// Merge the amounts by adding to existing quota
			newAmount := existingQuota.Amount + quota.Amount
			if err := tx.Model(&existingQuota).Update("amount", newAmount).Error; err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("failed to merge quota amount for user %s, expiry %s: %w",
					req.MainUserID, quota.ExpiryDate.Format("2006-01-02"), err)
			}

			// Delete the original quota from other user to avoid duplication
			if err := tx.Delete(&quota).Error; err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("failed to delete original quota from user %s: %w", req.OtherUserID, err)
			}
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			// No conflict found, safely update user_id
			if err := tx.Model(&quota).Update("user_id", req.MainUserID).Error; err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("failed to transfer quota from user %s to user %s: %w",
					req.OtherUserID, req.MainUserID, err)
			}
		} else {
			// Unexpected error occurred during checking
			tx.Rollback()
			return nil, fmt.Errorf("failed to check existing quota for user %s: %w", req.MainUserID, err)
		}
	}

	tx.Commit()

	return &MergeQuotaResponse{
		MainUserID:  req.MainUserID,
		OtherUserID: req.OtherUserID,
		Amount:      totalAmount,
		Operation:   "MERGE_QUOTA",
		Status:      "SUCCESS",
		Message:     "Quota merged successfully",
	}, nil
}

// min returns the minimum of two float64 values
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
