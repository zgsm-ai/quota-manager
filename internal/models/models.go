package models

import (
	"time"

	"gorm.io/gorm"
)

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
	ID          int        `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      string     `gorm:"not null;index;size:255" json:"user_id"`
	Amount      int        `gorm:"not null" json:"amount"`                  // positive or negative
	Operation   string     `gorm:"not null;index;size:50" json:"operation"` // RECHARGE/TRANSFER_IN/TRANSFER_OUT
	Description string     `gorm:"type:text" json:"description"`
	VoucherCode string     `gorm:"uniqueIndex;size:255" json:"voucher_code,omitempty"`
	RelatedUser string     `gorm:"size:255" json:"related_user,omitempty"`
	ExpiryDate  *time.Time `json:"expiry_date,omitempty"`
	CreateTime  time.Time  `gorm:"autoCreateTime;index" json:"create_time"`
}

// VoucherRedemption track redeemed vouchers to prevent duplicate redemption
type VoucherRedemption struct {
	ID          int       `gorm:"primaryKey;autoIncrement" json:"id"`
	VoucherCode string    `gorm:"uniqueIndex;not null;size:255" json:"voucher_code"`
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
