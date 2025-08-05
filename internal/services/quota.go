package services

import (
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
func (s *QuotaService) AddQuotaForStrategy(userID string, amount float64, strategyID int, strategyName string) error {
	// Calculate expiry date (end of this/next month)
	now := utils.NowInConfigTimezone(s.configManager.GetDirect()).Truncate(time.Second)
	var expiryDate time.Time

	// Always set to end of current month
	expiryDate = time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, now.Location())

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

	// 步骤 1: 记录每月已使用配额（在 Find expired but still valid quotas 之前）
	logger.Info("Step 1: Recording monthly used quota before expiry processing")
	if err := s.recordMonthlyUsedQuota(now); err != nil {
		logger.Error("Failed to record monthly used quota", zap.Error(err))
		// 注意：这里不直接返回错误，继续执行过期处理，但记录错误日志
		// 因为月度配额记录失败不应该影响配额过期的主要功能
	}

	// 步骤 2: Find expired but still valid quotas（原有逻辑）
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
			UserID:     userID,
			Amount:     -expiredAmount, // Negative amount for expiry
			Operation:  "EXPIRE",
			ExpiryDate: now, // Use current time as expiry time
			CreateTime: utils.NowInConfigTimezone(s.configManager.GetDirect()),
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

// GetUsersWithValidQuota 获取存在有效配额的所有用户
func (s *QuotaService) GetUsersWithValidQuota() ([]string, error) {
	var userIDs []string

	// 使用 DISTINCT 确保每个用户只出现一次
	if err := s.db.DB.Model(&models.Quota{}).
		Where("status = ?", models.StatusValid).
		Distinct("user_id").
		Find(&userIDs).Error; err != nil {
		return nil, fmt.Errorf("failed to get users with valid quota: %w", err)
	}

	return userIDs, nil
}

// recordUserMonthlyUsedQuota 记录单个用户的月度已使用配额
func (s *QuotaService) recordUserMonthlyUsedQuota(userID string, yearMonth string) error {
	// 从 aigateway 获取用户已使用配额
	usedQuota, err := s.aiGatewayClient.QueryUsedQuotaValue(userID)
	if err != nil {
		logger.Warn("Failed to get used quota from aigateway",
			zap.String("user_id", userID),
			zap.Error(err))
		return fmt.Errorf("failed to get used quota for user %s: %w", userID, err)
	}

	// 如果已使用配额为0或不存在，则不记录
	if usedQuota <= 0 {
		logger.Info("Skip recording zero or negative used quota",
			zap.String("user_id", userID),
			zap.Float64("used_quota", usedQuota))
		return nil
	}

	// 创建记录
	record := &models.MonthlyQuotaUsage{
		UserID:     userID,
		YearMonth:  yearMonth,
		UsedQuota:  usedQuota,
		RecordTime: utils.NowInConfigTimezone(s.configManager.GetDirect()),
	}

	// 使用 ON CONFLICT 处理重复记录
	if err := s.db.DB.Create(record).Error; err != nil {
		// 如果是唯一约束冲突，更新现有记录
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

// recordMonthlyUsedQuota 记录所有用户的月度已使用配额
func (s *QuotaService) recordMonthlyUsedQuota(now time.Time) error {
	logger.Info("Starting to record monthly used quota")

	// 获取上个月年月，格式为 YYYY-MM
	lastMonth := now.AddDate(0, -1, 0)
	yearMonth := lastMonth.Format("2006-01")

	// 获取存在有效配额的所有用户
	userIDs, err := s.GetUsersWithValidQuota()
	if err != nil {
		return fmt.Errorf("failed to get users with valid quota: %w", err)
	}

	logger.Info("Found users with valid quota", zap.Int("count", len(userIDs)))

	// 批量处理用户
	for _, userID := range userIDs {
		err := s.recordUserMonthlyUsedQuota(userID, yearMonth)
		if err != nil {
			logger.Error("Failed to record monthly used quota for user",
				zap.String("user_id", userID),
				zap.Error(err))
			// 继续处理其他用户，不中断整个流程
		}
	}

	logger.Info("Monthly quota usage recording completed",
		zap.Int("total_users", len(userIDs)))

	return nil
}
