package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"quota-manager/internal/config"
	"quota-manager/internal/models"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// QuotaService handles quota-related operations
type QuotaService struct {
	db            *gorm.DB
	aiGatewayConf *config.AiGatewayConfig
	voucherSvc    *VoucherService
}

// NewQuotaService creates a new quota service
func NewQuotaService(db *gorm.DB, aiGatewayConf *config.AiGatewayConfig, voucherSvc *VoucherService) *QuotaService {
	return &QuotaService{
		db:            db,
		aiGatewayConf: aiGatewayConf,
		voucherSvc:    voucherSvc,
	}
}

// QuotaInfo represents user quota information
type QuotaInfo struct {
	TotalQuota int               `json:"total_quota"`
	UsedQuota  int               `json:"used_quota"`
	QuotaList  []QuotaDetailItem `json:"quota_list"`
}

// QuotaDetailItem represents quota detail item
type QuotaDetailItem struct {
	Amount     int       `json:"amount"`
	ExpiryDate time.Time `json:"expiry_date"`
}

// QuotaAuditRecord represents quota audit record
type QuotaAuditRecord struct {
	Amount      int       `json:"amount"`
	Operation   string    `json:"operation"`
	VoucherCode string    `json:"voucher_code,omitempty"`
	RelatedUser string    `json:"related_user,omitempty"`
	ExpiryDate  time.Time `json:"expiry_date"`
	CreateTime  time.Time `json:"create_time"`
}

// TransferOutRequest represents transfer out request
type TransferOutRequest struct {
	ReceiverID string              `json:"receiver_id"`
	QuotaList  []TransferQuotaItem `json:"quota_list"`
}

// TransferQuotaItem represents quota item for transfer
type TransferQuotaItem struct {
	Amount     int       `json:"amount"`
	ExpiryDate time.Time `json:"expiry_date"`
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
	VoucherCode string `json:"voucher_code"`
}

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
	Amount      int                   `json:"amount"`
}

// TransferQuotaResult represents transfer quota result
type TransferQuotaResult struct {
	Amount     int       `json:"amount"`
	ExpiryDate time.Time `json:"expiry_date"`
	IsExpired  bool      `json:"is_expired"`
}

// GetUserQuota retrieves user quota information
func (s *QuotaService) GetUserQuota(userID string) (*QuotaInfo, error) {
	// Get total quota from AiGateway
	totalQuota, err := s.getQuotaFromAiGateway(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get total quota: %w", err)
	}

	// Get used quota from AiGateway
	usedQuota, err := s.getUsedQuotaFromAiGateway(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get used quota: %w", err)
	}

	// Get quota list from database
	var quotas []models.Quota
	if err := s.db.Where("user_id = ? AND status = ?", userID, models.StatusValid).Find(&quotas).Error; err != nil {
		return nil, fmt.Errorf("failed to get quota list: %w", err)
	}

	// Group quotas by expiry date
	quotaMap := make(map[string]int)
	for _, quota := range quotas {
		key := quota.ExpiryDate.Format("2006-01-02T15:04:05Z")
		quotaMap[key] += quota.Amount
	}

	quotaList := make([]QuotaDetailItem, 0, len(quotaMap))
	for dateStr, amount := range quotaMap {
		expiryDate, _ := time.Parse("2006-01-02T15:04:05Z", dateStr)
		quotaList = append(quotaList, QuotaDetailItem{
			Amount:     amount,
			ExpiryDate: expiryDate,
		})
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
	if err := s.db.Model(&models.QuotaAudit{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count audit records: %w", err)
	}

	// Get records with pagination
	if err := s.db.Where("user_id = ?", userID).
		Order("create_time DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&records).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get audit records: %w", err)
	}

	result := make([]QuotaAuditRecord, len(records))
	for i, record := range records {
		result[i] = QuotaAuditRecord{
			Amount:      record.Amount,
			Operation:   record.Operation,
			VoucherCode: record.VoucherCode,
			RelatedUser: record.RelatedUser,
			ExpiryDate:  record.ExpiryDate,
			CreateTime:  record.CreateTime,
		}
	}

	return result, total, nil
}

// TransferOut handles quota transfer out
func (s *QuotaService) TransferOut(giver *models.AuthUser, req *TransferOutRequest) (*TransferOutResponse, error) {
	// Start transaction
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Validate quota availability
	for _, quotaItem := range req.QuotaList {
		var quota models.Quota
		if err := tx.Where("user_id = ? AND expiry_date = ? AND status = ?",
			giver.ID, quotaItem.ExpiryDate, models.StatusValid).First(&quota).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("quota not found for expiry date %v", quotaItem.ExpiryDate)
		}

		if quota.Amount < quotaItem.Amount {
			tx.Rollback()
			return nil, fmt.Errorf("insufficient quota for expiry date %v: have %d, need %d",
				quotaItem.ExpiryDate, quota.Amount, quotaItem.Amount)
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

	voucherData := &VoucherData{
		GiverID:     giver.ID,
		GiverName:   giver.Name,
		GiverPhone:  giver.Phone,
		GiverGithub: giver.Github,
		ReceiverID:  req.ReceiverID,
		QuotaList:   voucherQuotaList,
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
	}

	// Calculate total amount for audit record
	totalAmount := 0
	for _, item := range req.QuotaList {
		totalAmount += item.Amount
	}

	// Record audit log
	auditRecord := &models.QuotaAudit{
		UserID:      giver.ID,
		Amount:      -totalAmount,
		Operation:   models.OperationTransferOut,
		VoucherCode: voucherCode,
		RelatedUser: req.ReceiverID,
		ExpiryDate:  req.QuotaList[0].ExpiryDate, // Use first expiry date for audit
	}
	if err := tx.Create(auditRecord).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create audit record: %w", err)
	}

	// Update AiGateway quota
	if err := s.deltaQuotaInAiGateway(giver.ID, -totalAmount); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update AiGateway quota: %w", err)
	}

	tx.Commit()

	return &TransferOutResponse{
		VoucherCode: voucherCode,
		RelatedUser: req.ReceiverID,
		Operation:   models.OperationTransferOut,
		QuotaList:   req.QuotaList,
	}, nil
}

// TransferIn handles quota transfer in
func (s *QuotaService) TransferIn(receiver *models.AuthUser, req *TransferInRequest) (*TransferInResponse, error) {
	// Validate voucher
	voucherData, err := s.voucherSvc.ValidateAndDecodeVoucher(req.VoucherCode)
	if err != nil {
		return nil, fmt.Errorf("invalid voucher code: %w", err)
	}

	// Check if voucher is for the correct receiver
	if voucherData.ReceiverID != receiver.ID {
		return nil, fmt.Errorf("voucher is not for this user")
	}

	// Check if voucher has already been redeemed
	var existingRedemption models.VoucherRedemption
	if err := s.db.Where("voucher_code = ?", req.VoucherCode).First(&existingRedemption).Error; err == nil {
		return nil, fmt.Errorf("voucher has already been redeemed")
	}

	// Start transaction
	tx := s.db.Begin()
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
		return nil, fmt.Errorf("failed to record voucher redemption: %w", err)
	}

	totalAmount := 0
	quotaResults := make([]TransferQuotaResult, len(voucherData.QuotaList))

	// Process quota transfer
	for i, quotaItem := range voucherData.QuotaList {
		isExpired := time.Now().After(quotaItem.ExpiryDate)
		quotaResults[i] = TransferQuotaResult{
			Amount:     quotaItem.Amount,
			ExpiryDate: quotaItem.ExpiryDate,
			IsExpired:  isExpired,
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
					tx.Rollback()
					return nil, fmt.Errorf("failed to create quota: %w", err)
				}
			} else {
				// Update existing quota
				if err := tx.Model(&existingQuota).Update("amount", existingQuota.Amount+quotaItem.Amount).Error; err != nil {
					tx.Rollback()
					return nil, fmt.Errorf("failed to update quota: %w", err)
				}
			}

			// Record audit log for valid quota
			auditRecord := &models.QuotaAudit{
				UserID:      receiver.ID,
				Amount:      quotaItem.Amount,
				Operation:   models.OperationTransferIn,
				VoucherCode: req.VoucherCode,
				RelatedUser: voucherData.GiverID,
				ExpiryDate:  quotaItem.ExpiryDate,
			}
			if err := tx.Create(auditRecord).Error; err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("failed to create audit record: %w", err)
			}

			totalAmount += quotaItem.Amount
		}
	}

	// Update AiGateway quota only for valid quota
	if totalAmount > 0 {
		if err := s.deltaQuotaInAiGateway(receiver.ID, totalAmount); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to update AiGateway quota: %w", err)
		}
	}

	tx.Commit()

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
	}, nil
}

// AddQuotaForStrategy adds quota for strategy execution
func (s *QuotaService) AddQuotaForStrategy(userID string, amount int, strategyName string) error {
	// Calculate expiry date (end of this/next month)
	now := time.Now()
	var expiryDate time.Time

	// If less than 30 days until end of month, set expiry to end of next month
	endOfMonth := time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, now.Location())
	if endOfMonth.Sub(now).Hours() < 24*30 {
		expiryDate = time.Date(now.Year(), now.Month()+2, 0, 23, 59, 59, 0, now.Location())
	} else {
		expiryDate = endOfMonth
	}

	// Start transaction
	tx := s.db.Begin()
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

	// Record audit log only if it's not expired yet
	auditRecord := &models.QuotaAudit{
		UserID:     userID,
		Amount:     amount,
		Operation:  models.OperationRecharge,
		ExpiryDate: expiryDate,
	}
	if err := tx.Create(auditRecord).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create audit record: %w", err)
	}

	// Update AiGateway quota
	if err := s.deltaQuotaInAiGateway(userID, amount); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update AiGateway quota: %w", err)
	}

	tx.Commit()
	return nil
}

// ExpireQuotas expires quotas and synchronizes with AiGateway
func (s *QuotaService) ExpireQuotas() error {
	now := time.Now()

	// Find expired but still valid quotas
	var expiredQuotas []models.Quota
	if err := s.db.Where("status = ? AND expiry_date < ?", models.StatusValid, now).Find(&expiredQuotas).Error; err != nil {
		return fmt.Errorf("failed to find expired quotas: %w", err)
	}

	if len(expiredQuotas) == 0 {
		return nil
	}

	// Group by user
	userQuotaMap := make(map[string]int)
	for _, quota := range expiredQuotas {
		userQuotaMap[quota.UserID] += quota.Amount
	}

	// Start transaction
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Mark quotas as expired
	if err := tx.Model(&models.Quota{}).
		Where("status = ? AND expiry_date < ?", models.StatusValid, now).
		Update("status", models.StatusExpired).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to expire quotas: %w", err)
	}

	// Process each user
	for userID := range userQuotaMap {
		// Get user's remaining valid quota
		var validQuotaSum int64
		if err := tx.Model(&models.Quota{}).
			Where("user_id = ? AND status = ?", userID, models.StatusValid).
			Select("COALESCE(SUM(amount), 0)").Scan(&validQuotaSum).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to calculate valid quota for user %s: %w", userID, err)
		}

		// Get current quota info from AiGateway
		totalQuota, err := s.getQuotaFromAiGateway(userID)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to get total quota from AiGateway for user %s: %w", userID, err)
		}

		usedQuota, err := s.getUsedQuotaFromAiGateway(userID)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to get used quota from AiGateway for user %s: %w", userID, err)
		}

		remainingQuota := totalQuota - usedQuota

		// Reset used quota first
		if err := s.deltaUsedQuotaInAiGateway(userID, -usedQuota); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to reset used quota for user %s: %w", userID, err)
		}

		// Adjust total quota
		validQuota := int(validQuotaSum)
		var newTotalQuota int
		if validQuota >= remainingQuota {
			newTotalQuota = validQuota
		} else {
			newTotalQuota = validQuota
		}

		deltaQuota := newTotalQuota - totalQuota
		if deltaQuota != 0 {
			if err := s.deltaQuotaInAiGateway(userID, deltaQuota); err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to adjust total quota for user %s: %w", userID, err)
			}
		}
	}

	tx.Commit()
	return nil
}

// Helper methods for AiGateway communication

func (s *QuotaService) getQuotaFromAiGateway(userID string) (int, error) {
	url := fmt.Sprintf("%s%s/quota?consumer=%s", s.aiGatewayConf.BaseURL(), s.aiGatewayConf.AdminPath, userID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}

	req.Header.Set("Authorization", "Bearer "+s.aiGatewayConf.Credential)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("AiGateway returned status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	quota, ok := result["quota"].(float64)
	if !ok {
		return 0, fmt.Errorf("invalid quota response format")
	}

	return int(quota), nil
}

func (s *QuotaService) getUsedQuotaFromAiGateway(userID string) (int, error) {
	url := fmt.Sprintf("%s%s/quota/used?consumer=%s", s.aiGatewayConf.BaseURL(), s.aiGatewayConf.AdminPath, userID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}

	req.Header.Set("Authorization", "Bearer "+s.aiGatewayConf.Credential)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("AiGateway returned status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	quota, ok := result["quota"].(float64)
	if !ok {
		return 0, fmt.Errorf("invalid used quota response format")
	}

	return int(quota), nil
}

func (s *QuotaService) deltaQuotaInAiGateway(userID string, delta int) error {
	reqURL := fmt.Sprintf("%s%s/quota/delta", s.aiGatewayConf.BaseURL(), s.aiGatewayConf.AdminPath)

	data := url.Values{}
	data.Set("consumer", userID)
	data.Set("value", strconv.Itoa(delta))

	req, err := http.NewRequest("POST", reqURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+s.aiGatewayConf.Credential)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("AiGateway returned status %d", resp.StatusCode)
	}

	return nil
}

func (s *QuotaService) deltaUsedQuotaInAiGateway(userID string, delta int) error {
	reqURL := fmt.Sprintf("%s%s/quota/used/delta", s.aiGatewayConf.BaseURL(), s.aiGatewayConf.AdminPath)

	data := url.Values{}
	data.Set("consumer", userID)
	data.Set("value", strconv.Itoa(delta))

	req, err := http.NewRequest("POST", reqURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+s.aiGatewayConf.Credential)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("AiGateway returned status %d", resp.StatusCode)
	}

	return nil
}
