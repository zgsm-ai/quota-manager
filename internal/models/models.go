package models

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// AuthUser struct for parsing user info from JWT
type AuthUser struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	StaffID string `json:"staffID"`
	Github  string `json:"github"`
	Phone   string `json:"phone"`
}

// parseUserInfoFromToken parses user info from JWT token
func ParseUserInfoFromToken(accessToken string) (*AuthUser, error) {
	// Remove "Bearer " prefix if present
	if strings.HasPrefix(accessToken, "Bearer ") {
		accessToken = accessToken[7:]
	}

	// Split JWT token into parts
	parts := strings.Split(accessToken, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT token format")
	}

	// Decode payload (second part)
	payload := parts[1]
	// Add padding if needed
	if m := len(payload) % 4; m != 0 {
		payload += strings.Repeat("=", 4-m)
	}

	payloadBytes, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	// Parse claims
	var claims map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JWT claims: %w", err)
	}

	// Extract user ID
	var userInfo AuthUser
	if id, ok := claims["id"].(string); ok {
		userInfo.ID = id
	} else {
		return nil, fmt.Errorf("user ID not found in JWT token")
	}

	return &userInfo, nil
}

// QuotaStrategy strategy table structure
type QuotaStrategy struct {
	ID           int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Name         string    `gorm:"uniqueIndex;not null" json:"name"`
	Title        string    `gorm:"not null" json:"title"`
	Type         string    `gorm:"not null" json:"type"` // periodic/single
	Amount       int       `gorm:"not null" json:"amount"`
	Model        string    `gorm:"not null" json:"model"`
	PeriodicExpr string    `gorm:"column:periodic_expr" json:"periodic_expr"`
	Condition    string    `json:"condition"`
	Status       bool      `gorm:"not null;default:true" json:"status"` // true=enabled, false=disabled
	CreateTime   time.Time `gorm:"autoCreateTime" json:"create_time"`
	UpdateTime   time.Time `gorm:"autoUpdateTime" json:"update_time"`
}

// QuotaExecute execution status table
type QuotaExecute struct {
	ID          int       `gorm:"primaryKey;autoIncrement" json:"id"`
	StrategyID  int       `gorm:"not null;index" json:"strategy_id"`
	User        string    `gorm:"column:user_id;not null;index" json:"user"`
	BatchNumber string    `gorm:"not null;index" json:"batch_number"`
	Status      string    `gorm:"not null" json:"status"`
	ExpiryDate  time.Time `gorm:"not null" json:"expiry_date"`
	CreateTime  time.Time `gorm:"autoCreateTime" json:"create_time"`
	UpdateTime  time.Time `gorm:"autoUpdateTime" json:"update_time"`
}

// UserInfo user information table
type UserInfo struct {
	ID             string    `gorm:"primaryKey" json:"id"`
	Name           string    `json:"name"`
	GithubUsername string    `json:"github_username"`
	Email          string    `json:"email"`
	Phone          string    `json:"phone"`
	GithubStar     string    `json:"github_star"`
	VIP            int       `gorm:"default:0" json:"vip"`
	Org            string    `json:"org"`
	RegisterTime   time.Time `json:"register_time"`
	AccessTime     time.Time `json:"access_time"`
	CreateTime     time.Time `gorm:"autoCreateTime" json:"create_time"`
	UpdateTime     time.Time `gorm:"autoUpdateTime" json:"update_time"`
}

// Quota user quota table with expiry time
type Quota struct {
	ID         int       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     string    `gorm:"not null;index;size:255" json:"user_id"`
	Amount     int       `gorm:"not null" json:"amount"`
	ExpiryDate time.Time `gorm:"not null;index" json:"expiry_date"`
	Status     string    `gorm:"not null;default:VALID;index;size:20" json:"status"` // VALID/EXPIRED
	CreateTime time.Time `gorm:"autoCreateTime" json:"create_time"`
	UpdateTime time.Time `gorm:"autoUpdateTime" json:"update_time"`
}

// QuotaAudit quota change audit log
type QuotaAudit struct {
	ID          int       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      string    `gorm:"not null;index;size:255" json:"user_id"`
	Amount      int       `gorm:"not null" json:"amount"`                  // positive or negative
	Operation   string    `gorm:"not null;index;size:50" json:"operation"` // RECHARGE/TRANSFER_IN/TRANSFER_OUT
	VoucherCode string    `gorm:"index;size:1000" json:"voucher_code,omitempty"`
	RelatedUser string    `gorm:"size:255" json:"related_user,omitempty"`
	ExpiryDate  time.Time `gorm:"not null" json:"expiry_date"`
	CreateTime  time.Time `gorm:"autoCreateTime;index" json:"create_time"`
}

// VoucherRedemption track redeemed vouchers to prevent duplicate redemption
type VoucherRedemption struct {
	ID          int       `gorm:"primaryKey;autoIncrement" json:"id"`
	VoucherCode string    `gorm:"uniqueIndex;not null;size:1000" json:"voucher_code"`
	ReceiverID  string    `gorm:"not null;size:255" json:"receiver_id"`
	CreateTime  time.Time `gorm:"autoCreateTime" json:"create_time"`
}

// TableName sets the table name
func (QuotaStrategy) TableName() string {
	return "quota_strategy"
}

func (QuotaExecute) TableName() string {
	return "quota_execute"
}

func (UserInfo) TableName() string {
	return "user_info"
}

func (Quota) TableName() string {
	return "quota"
}

func (QuotaAudit) TableName() string {
	return "quota_audit"
}

func (VoucherRedemption) TableName() string {
	return "voucher_redemption"
}

// AutoMigrate automatically migrates database tables
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&QuotaStrategy{}, &QuotaExecute{}, &UserInfo{}, &Quota{}, &QuotaAudit{}, &VoucherRedemption{})
}

// IsEnabled checks if the strategy is enabled
func (s *QuotaStrategy) IsEnabled() bool {
	return s.Status
}

// Enable enables the strategy
func (s *QuotaStrategy) Enable() {
	s.Status = true
}

// Disable disables the strategy
func (s *QuotaStrategy) Disable() {
	s.Status = false
}

// IsValid checks if quota is valid
func (q *Quota) IsValid() bool {
	return q.Status == "VALID"
}

// IsExpired checks if quota is expired
func (q *Quota) IsExpired() bool {
	return time.Now().After(q.ExpiryDate)
}

// Expire sets quota status to expired
func (q *Quota) Expire() {
	q.Status = "EXPIRED"
}

// Operation constants
const (
	OperationRecharge    = "RECHARGE"
	OperationTransferIn  = "TRANSFER_IN"
	OperationTransferOut = "TRANSFER_OUT"
)

// Status constants
const (
	StatusValid   = "VALID"
	StatusExpired = "EXPIRED"
)
