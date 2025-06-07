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
	Amount      int        `json:"amount"`
	Operation   string     `json:"operation"`
	Description string     `json:"description"`
	VoucherCode string     `json:"voucher_code,omitempty"`
	ExpiryDate  *time.Time `json:"expiry_date,omitempty"`
	CreateTime  time.Time  `json:"create_time"`
}

// TransferOutRequest represents transfer out request
type TransferOutRequest struct {
	GiverID     string              `json:"giver_id"`
	GiverName   string              `json:"giver_name"`
	GiverPhone  string              `json:"giver_phone"`
	GiverGithub string              `json:"giver_github"`
	ReceiverID  string              `json:"receiver_id"`
	QuotaList   []TransferQuotaItem `json:"quota_list"`
}

// TransferQuotaItem represents quota item for transfer
type TransferQuotaItem struct {
	Amount     int       `json:"amount"`
	ExpiryDate time.Time `json:"expiry_date"`
}

// TransferOutResponse represents transfer out response
type TransferOutResponse struct {
	VoucherCode string `json:"voucher_code"`
	Description string `json:"description"`
}

// TransferInRequest represents transfer in request
type TransferInRequest struct {
	ReceiverID  string `json:"receiver_id"`
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
	Description string                `json:"description"`
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
			Description: record.Description,
			VoucherCode: record.VoucherCode,
			ExpiryDate:  record.ExpiryDate,
			CreateTime:  record.CreateTime,
		}
	}

	return result, total, nil
}

// TransferOut handles quota transfer out
func (s *QuotaService) TransferOut(req *TransferOutRequest) (*TransferOutResponse, error) {
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
			req.GiverID, quotaItem.ExpiryDate, models.StatusValid).First(&quota).Error; err != nil {
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
	totalAmount := 0
	for i, item := range req.QuotaList {
		voucherQuotaList[i] = VoucherQuotaItem{
			Amount:     item.Amount,
			ExpiryDate: item.ExpiryDate,
		}
		totalAmount += item.Amount
	}

	voucherData := &VoucherData{
		GiverID:     req.GiverID,
		GiverName:   req.GiverName,
		GiverPhone:  req.GiverPhone,
		GiverGithub: req.GiverGithub,
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
				req.GiverID, quotaItem.ExpiryDate, models.StatusValid).
			Update("amount", gorm.Expr("amount - ?", quotaItem.Amount)).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to update quota: %w", err)
		}
	}

	// Generate description
	var quotaDetails []string
	for _, item := range req.QuotaList {
		quotaDetails = append(quotaDetails, fmt.Sprintf("%d Credit将在%s到期",
			item.Amount, item.ExpiryDate.Format("2006年1月2日")))
	}
	description := fmt.Sprintf("您给用户%s转出总共%d Credit，兑换码:%s，其中%s",
		req.ReceiverID, totalAmount, voucherCode, strings.Join(quotaDetails, "，"))

	// Record audit log
	auditRecord := &models.QuotaAudit{
		UserID:      req.GiverID,
		Amount:      -totalAmount,
		Operation:   models.OperationTransferOut,
		Description: description,
		VoucherCode: voucherCode,
		RelatedUser: req.ReceiverID,
	}
	if err := tx.Create(auditRecord).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create audit record: %w", err)
	}

	// Update AiGateway quota
	if err := s.deltaQuotaInAiGateway(req.GiverID, -totalAmount); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update AiGateway quota: %w", err)
	}

	tx.Commit()

	return &TransferOutResponse{
		VoucherCode: voucherCode,
		Description: description,
	}, nil
}

// TransferIn handles quota transfer in
func (s *QuotaService) TransferIn(req *TransferInRequest) (*TransferInResponse, error) {
	// Validate and decode voucher
	voucherData, err := s.voucherSvc.ValidateAndDecodeVoucher(req.VoucherCode)
	if err != nil {
		return nil, fmt.Errorf("invalid voucher code: %w", err)
	}

	// Check if receiver ID matches
	if voucherData.ReceiverID != req.ReceiverID {
		return nil, fmt.Errorf("receiver ID mismatch")
	}

	// Check if voucher has been redeemed
	var redemption models.VoucherRedemption
	if err := s.db.Where("voucher_code = ?", req.VoucherCode).First(&redemption).Error; err == nil {
		return nil, fmt.Errorf("voucher code has already been redeemed")
	}

	// Start transaction
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Filter valid quotas and calculate amounts
	now := time.Now()
	validQuotas := make([]TransferQuotaResult, 0)
	expiredQuotas := make([]TransferQuotaResult, 0)
	totalValidAmount := 0
	totalExpiredAmount := 0

	for _, quotaItem := range voucherData.QuotaList {
		isExpired := now.After(quotaItem.ExpiryDate)
		result := TransferQuotaResult{
			Amount:     quotaItem.Amount,
			ExpiryDate: quotaItem.ExpiryDate,
			IsExpired:  isExpired,
		}

		if isExpired {
			expiredQuotas = append(expiredQuotas, result)
			totalExpiredAmount += quotaItem.Amount
		} else {
			validQuotas = append(validQuotas, result)
			totalValidAmount += quotaItem.Amount

			// Add quota to receiver
			quota := &models.Quota{
				UserID:     req.ReceiverID,
				Amount:     quotaItem.Amount,
				ExpiryDate: quotaItem.ExpiryDate,
				Status:     models.StatusValid,
			}
			if err := tx.Create(quota).Error; err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("failed to create quota: %w", err)
			}
		}
	}

	allQuotas := append(validQuotas, expiredQuotas...)

	// Generate description
	var validDetails []string
	var expiredDetails []string

	for _, quota := range validQuotas {
		validDetails = append(validDetails, fmt.Sprintf("%d Credit将在%s到期",
			quota.Amount, quota.ExpiryDate.Format("2006年1月2日")))
	}

	for _, quota := range expiredQuotas {
		expiredDetails = append(expiredDetails, fmt.Sprintf("%d Credit已在%s过期",
			quota.Amount, quota.ExpiryDate.Format("2006年1月2日")))
	}

	descParts := []string{
		fmt.Sprintf("由用户%s给您转入总共%d Credit", voucherData.GiverName, totalValidAmount+totalExpiredAmount),
		fmt.Sprintf("兑换码:%s", req.VoucherCode),
	}

	if len(validDetails) > 0 {
		descParts = append(descParts, fmt.Sprintf("其中有效Credit:%d(%s)", totalValidAmount, strings.Join(validDetails, "，")))
	}

	if len(expiredDetails) > 0 {
		descParts = append(descParts, fmt.Sprintf("失效Credit:%d(%s)", totalExpiredAmount, strings.Join(expiredDetails, "，")))
	}

	description := strings.Join(descParts, "，")

	// Record audit log
	auditRecord := &models.QuotaAudit{
		UserID:      req.ReceiverID,
		Amount:      totalValidAmount,
		Operation:   models.OperationTransferIn,
		Description: description,
		VoucherCode: req.VoucherCode,
		RelatedUser: voucherData.GiverID,
	}
	if err := tx.Create(auditRecord).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create audit record: %w", err)
	}

	// Record voucher redemption
	redemptionRecord := &models.VoucherRedemption{
		VoucherCode: req.VoucherCode,
		ReceiverID:  req.ReceiverID,
	}
	if err := tx.Create(redemptionRecord).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create redemption record: %w", err)
	}

	// Update AiGateway quota
	if totalValidAmount > 0 {
		if err := s.deltaQuotaInAiGateway(req.ReceiverID, totalValidAmount); err != nil {
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
		ReceiverID:  voucherData.ReceiverID,
		QuotaList:   allQuotas,
		Description: description,
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

	// Record audit log
	description := fmt.Sprintf("策略执行充值: %s", strategyName)
	auditRecord := &models.QuotaAudit{
		UserID:      userID,
		Amount:      amount,
		Operation:   models.OperationRecharge,
		Description: description,
		ExpiryDate:  &expiryDate,
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
