package models

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"quota-manager/internal/config"
	"quota-manager/internal/utils"
)

// AuthUser struct for parsing user info from JWT
type AuthUser struct {
	ID      string `json:"universal_id"`
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
	if id, ok := claims["universal_id"].(string); ok {
		userInfo.ID = id
	} else {
		return nil, fmt.Errorf("user ID not found in JWT token")
	}

	return &userInfo, nil
}

// QuotaStrategy strategy table structure
type QuotaStrategy struct {
	ID           int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Name         string    `gorm:"uniqueIndex;not null" json:"name" validate:"required,min=1,max=100"`
	Title        string    `gorm:"not null" json:"title" validate:"required,min=1,max=200"`
	Type         string    `gorm:"not null" json:"type" validate:"required,oneof=single periodic"` // periodic/single
	Amount       float64   `gorm:"not null" json:"amount"`
	Model        string    `json:"model" validate:"omitempty,min=1,max=100"`
	PeriodicExpr string    `gorm:"column:periodic_expr" json:"periodic_expr" validate:"omitempty,cron"`
	Condition    string    `json:"condition" validate:"omitempty"`
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
	ID               string    `gorm:"primaryKey;type:uuid" json:"id"`
	CreatedAt        time.Time `gorm:"column:created_at;type:timestamptz(0)" json:"created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at;type:timestamptz(0)" json:"updated_at"`
	AccessTime       time.Time `gorm:"column:access_time;type:timestamptz(0)" json:"access_time"`
	Name             string    `gorm:"size:100;not null" json:"name"`
	GithubID         string    `gorm:"column:github_id;size:100" json:"github_id"`
	GithubName       string    `gorm:"column:github_name;size:100" json:"github_name"`
	VIP              int       `gorm:"default:0" json:"vip"`
	Phone            string    `gorm:"size:20" json:"phone"`
	Email            string    `gorm:"size:100" json:"email"`
	Password         string    `gorm:"size:100" json:"password"`
	Company          string    `gorm:"size:100" json:"company"`
	Location         string    `gorm:"size:100" json:"location"`
	UserCode         string    `gorm:"column:user_code;size:100" json:"user_code"`
	ExternalAccounts string    `gorm:"column:external_accounts;size:100" json:"external_accounts"`
	EmployeeNumber   string    `gorm:"column:employee_number;size:100" json:"employee_number"`
	GithubStar       string    `gorm:"column:github_star;type:text" json:"github_star"`
	Devices          string    `gorm:"type:jsonb" json:"devices"`
}

// Quota user quota table with expiry time
type Quota struct {
	ID         int       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     string    `gorm:"not null;index;size:255" json:"user_id"`
	Amount     float64   `gorm:"not null" json:"amount"`
	ExpiryDate time.Time `gorm:"not null;index" json:"expiry_date"`
	Status     string    `gorm:"not null;default:VALID;index;size:20" json:"status"` // VALID/EXPIRED
	CreateTime time.Time `gorm:"autoCreateTime" json:"create_time"`
	UpdateTime time.Time `gorm:"autoUpdateTime" json:"update_time"`
}

// QuotaAudit quota change audit log
type QuotaAudit struct {
	ID           int       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       string    `gorm:"not null;index;size:255" json:"user_id"`
	Amount       float64   `gorm:"not null" json:"amount"`                  // positive or negative
	Operation    string    `gorm:"not null;index;size:50" json:"operation"` // RECHARGE/TRANSFER_IN/TRANSFER_OUT
	VoucherCode  string    `gorm:"index;size:1000" json:"voucher_code,omitempty"`
	RelatedUser  string    `gorm:"size:255" json:"related_user,omitempty"`
	StrategyID   *int      `gorm:"index" json:"strategy_id,omitempty"`            // Strategy ID for RECHARGE operations
	StrategyName string    `gorm:"index;size:100" json:"strategy_name,omitempty"` // Strategy name for RECHARGE operations
	ExpiryDate   time.Time `gorm:"not null" json:"expiry_date"`
	Details      string    `gorm:"type:text" json:"details,omitempty"` // JSON string with detailed operation info
	CreateTime   time.Time `gorm:"autoCreateTime;index" json:"create_time"`
}

// QuotaAuditDetails contains detailed information about quota operations
type QuotaAuditDetails struct {
	Operation string                 `json:"operation"`
	Summary   QuotaAuditSummary      `json:"summary"`
	Items     []QuotaAuditDetailItem `json:"items,omitempty"`
}

// QuotaAuditSummary contains summary information
type QuotaAuditSummary struct {
	TotalAmount        float64 `json:"total_amount"`
	TotalItems         int     `json:"total_items"`
	SuccessfulItems    int     `json:"successful_items,omitempty"`
	FailedItems        int     `json:"failed_items,omitempty"`
	ExpiredItems       int     `json:"expired_items,omitempty"`
	EarliestExpiryDate string  `json:"earliest_expiry_date"`
}

// QuotaAuditDetailItem represents individual quota item in audit
type QuotaAuditDetailItem struct {
	Amount        float64 `json:"amount"`
	ExpiryDate    string  `json:"expiry_date"`
	Status        string  `json:"status"` // SUCCESS/FAILED/EXPIRED
	FailureReason string  `json:"failure_reason,omitempty"`
	OriginalQuota float64 `json:"original_quota,omitempty"` // For TRANSFER_IN: existing quota before transfer
	NewQuota      float64 `json:"new_quota,omitempty"`      // For TRANSFER_IN: quota after transfer
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
	return "auth_users"
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

// EmployeeDepartment represents the employee department mapping
type EmployeeDepartment struct {
	ID                 int       `gorm:"primaryKey;autoIncrement" json:"id"`
	EmployeeNumber     string    `gorm:"uniqueIndex;not null;size:100" json:"employee_number"`
	Username           string    `gorm:"not null;size:100" json:"username"`
	DeptFullLevelNames string    `gorm:"type:text;not null" json:"dept_full_level_names"` // Store as comma-separated string
	CreateTime         time.Time `gorm:"autoCreateTime" json:"create_time"`
	UpdateTime         time.Time `gorm:"autoUpdateTime" json:"update_time"`
}

// ModelWhitelist represents the model whitelist for users and departments
type ModelWhitelist struct {
	ID               int       `gorm:"primaryKey;autoIncrement" json:"id"`
	TargetType       string    `gorm:"not null;size:20;index" json:"target_type"`        // 'user' or 'department'
	TargetIdentifier string    `gorm:"not null;size:500;index" json:"target_identifier"` // employee_number for user, department name for department
	AllowedModels    string    `gorm:"type:text;not null" json:"allowed_models"`         // Store as comma-separated string
	CreateTime       time.Time `gorm:"autoCreateTime" json:"create_time"`
	UpdateTime       time.Time `gorm:"autoUpdateTime" json:"update_time"`
}

// EffectivePermission represents the effective permissions for each employee
type EffectivePermission struct {
	ID              int       `gorm:"primaryKey;autoIncrement" json:"id"`
	EmployeeNumber  string    `gorm:"uniqueIndex;not null;size:100" json:"employee_number"`
	EffectiveModels string    `gorm:"type:text;not null" json:"effective_models"` // Store as comma-separated string
	WhitelistID     *int      `gorm:"index" json:"whitelist_id"`
	CreateTime      time.Time `gorm:"autoCreateTime" json:"create_time"`
	UpdateTime      time.Time `gorm:"autoUpdateTime" json:"update_time"`
}

// PermissionAudit represents the audit log for permission operations
type PermissionAudit struct {
	ID               int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Operation        string    `gorm:"not null;size:50;index" json:"operation"`
	TargetType       string    `gorm:"size:20;index" json:"target_type"`
	TargetIdentifier string    `gorm:"size:500;index" json:"target_identifier"`
	Details          string    `gorm:"type:text" json:"details"`
	CreateTime       time.Time `gorm:"autoCreateTime;index" json:"create_time"`
}

// StarCheckSetting represents star check settings for users/departments
type StarCheckSetting struct {
	ID               int       `gorm:"primaryKey;autoIncrement" json:"id"`
	TargetType       string    `gorm:"not null;size:20;index" json:"target_type"`        // 'user' or 'department'
	TargetIdentifier string    `gorm:"not null;size:500;index" json:"target_identifier"` // employee_number for user, department name for department
	Enabled          bool      `gorm:"not null" json:"enabled"`                          // star check enabled/disabled
	CreateTime       time.Time `gorm:"autoCreateTime" json:"create_time"`
	UpdateTime       time.Time `gorm:"autoUpdateTime" json:"update_time"`
}

// EffectiveStarCheckSetting represents the effective star check settings for each employee
type EffectiveStarCheckSetting struct {
	ID             int       `gorm:"primaryKey;autoIncrement" json:"id"`
	EmployeeNumber string    `gorm:"uniqueIndex;not null;size:100" json:"employee_number"`
	Enabled        bool      `gorm:"not null" json:"enabled"` // effective star check setting
	SettingID      *int      `gorm:"index" json:"setting_id"`
	CreateTime     time.Time `gorm:"autoCreateTime" json:"create_time"`
	UpdateTime     time.Time `gorm:"autoUpdateTime" json:"update_time"`
}

// QuotaCheckSetting represents quota check settings for users/departments
type QuotaCheckSetting struct {
	ID               int       `gorm:"primaryKey;autoIncrement" json:"id"`
	TargetType       string    `gorm:"not null;size:20;index" json:"target_type"`        // 'user' or 'department'
	TargetIdentifier string    `gorm:"not null;size:500;index" json:"target_identifier"` // employee_number for user, department name for department
	Enabled          bool      `gorm:"not null;default:false" json:"enabled"`            // quota check enabled/disabled
	CreateTime       time.Time `gorm:"autoCreateTime" json:"create_time"`
	UpdateTime       time.Time `gorm:"autoUpdateTime" json:"update_time"`
}

// EffectiveQuotaCheckSetting represents the effective quota check settings for each employee
type EffectiveQuotaCheckSetting struct {
	ID             int       `gorm:"primaryKey;autoIncrement" json:"id"`
	EmployeeNumber string    `gorm:"uniqueIndex;not null;size:100" json:"employee_number"`
	Enabled        bool      `gorm:"not null;default:false" json:"enabled"` // effective quota check setting
	SettingID      *int      `gorm:"index" json:"setting_id"`
	CreateTime     time.Time `gorm:"autoCreateTime" json:"create_time"`
	UpdateTime     time.Time `gorm:"autoUpdateTime" json:"update_time"`
}

// GetDeptFullLevelNamesAsSlice returns the department full level names as a slice
func (e *EmployeeDepartment) GetDeptFullLevelNamesAsSlice() []string {
	if e.DeptFullLevelNames == "" {
		return []string{}
	}
	return strings.Split(e.DeptFullLevelNames, ",")
}

// SetDeptFullLevelNamesFromSlice sets the department full level names from a slice
func (e *EmployeeDepartment) SetDeptFullLevelNamesFromSlice(names []string) {
	e.DeptFullLevelNames = strings.Join(names, ",")
}

// TableName sets the table name for EmployeeDepartment
func (EmployeeDepartment) TableName() string {
	return "employee_department"
}

// GetAllowedModelsAsSlice returns the allowed models as a slice
func (m *ModelWhitelist) GetAllowedModelsAsSlice() []string {
	if m.AllowedModels == "" {
		return []string{}
	}
	return strings.Split(m.AllowedModels, ",")
}

// SetAllowedModelsFromSlice sets the allowed models from a slice
func (m *ModelWhitelist) SetAllowedModelsFromSlice(models []string) {
	m.AllowedModels = strings.Join(models, ",")
}

// TableName sets the table name for ModelWhitelist
func (ModelWhitelist) TableName() string {
	return "model_whitelist"
}

// GetEffectiveModelsAsSlice returns the effective models as a slice
func (e *EffectivePermission) GetEffectiveModelsAsSlice() []string {
	if e.EffectiveModels == "" {
		return []string{}
	}
	return strings.Split(e.EffectiveModels, ",")
}

// SetEffectiveModelsFromSlice sets the effective models from a slice
func (e *EffectivePermission) SetEffectiveModelsFromSlice(models []string) {
	e.EffectiveModels = strings.Join(models, ",")
}

// TableName sets the table name for EffectivePermission
func (EffectivePermission) TableName() string {
	return "effective_permissions"
}

// TableName sets the table name for PermissionAudit
func (PermissionAudit) TableName() string {
	return "permission_audit"
}

// TableName sets the table name for StarCheckSetting
func (StarCheckSetting) TableName() string {
	return "star_check_settings"
}

// TableName sets the table name for EffectiveStarCheckSetting
func (EffectiveStarCheckSetting) TableName() string {
	return "effective_star_check_settings"
}

// TableName sets the table name for QuotaCheckSetting
func (QuotaCheckSetting) TableName() string {
	return "quota_check_settings"
}

// TableName sets the table name for EffectiveQuotaCheckSetting
func (EffectiveQuotaCheckSetting) TableName() string {
	return "effective_quota_check_settings"
}

// Constants for target types
const (
	TargetTypeUser       = "user"
	TargetTypeDepartment = "department"
)

// Constants for permission operations
const (
	OperationEmployeeSync            = "employee_sync"
	OperationWhitelistSet            = "whitelist_set"
	OperationPermissionUpdate        = "permission_updated"
	OperationStarCheckSet            = "star_check_set"
	OperationStarCheckSettingUpdate  = "star_check_setting_update"
	OperationQuotaCheckSet           = "quota_check_set"
	OperationQuotaCheckSettingUpdate = "quota_check_setting_update"
)

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
	// For model methods, we need to get config from the global config manager
	// This is a temporary solution - ideally we should pass config as parameter
	cfg := config.GetGlobalConfig()
	if cfg == nil {
		// Fallback to system timezone if config is not available
		return time.Now().Truncate(time.Second).After(q.ExpiryDate.Truncate(time.Second))
	}

	now := utils.NowInConfigTimezone(cfg).Truncate(time.Second)
	return now.After(q.ExpiryDate.Truncate(time.Second))
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

// Status constants for quota audit detail items
const (
	AuditStatusSuccess = "SUCCESS"
	AuditStatusFailed  = "FAILED"
	AuditStatusExpired = "EXPIRED"
)

// MarshalDetails converts QuotaAuditDetails to JSON string
func (q *QuotaAudit) MarshalDetails(details *QuotaAuditDetails) error {
	if details == nil {
		q.Details = ""
		return nil
	}

	jsonBytes, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("failed to marshal audit details: %w", err)
	}

	q.Details = string(jsonBytes)
	return nil
}

// UnmarshalDetails converts JSON string back to QuotaAuditDetails
func (q *QuotaAudit) UnmarshalDetails() (*QuotaAuditDetails, error) {
	if q.Details == "" {
		return nil, nil
	}

	var details QuotaAuditDetails
	if err := json.Unmarshal([]byte(q.Details), &details); err != nil {
		return nil, fmt.Errorf("failed to unmarshal audit details: %w", err)
	}

	return &details, nil
}

// Status constants
const (
	StatusValid   = "VALID"
	StatusExpired = "EXPIRED"
)

// MonthlyQuotaUsage 月度配额使用量记录表
type MonthlyQuotaUsage struct {
	ID         int       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID     string    `gorm:"column:user_id;not null;index" json:"user_id"`
	YearMonth  string    `gorm:"column:year_month;not null;index" json:"year_month"` // 格式: YYYY-MM
	UsedQuota  float64   `gorm:"column:used_quota;not null" json:"used_quota"`
	RecordTime time.Time `gorm:"column:record_time;type:timestamptz(0)" json:"record_time"`
	CreateTime time.Time `gorm:"column:create_time;type:timestamptz(0);autoCreateTime" json:"create_time"`
}

// TableName 设置表名
func (MonthlyQuotaUsage) TableName() string {
	return "monthly_quota_usage"
}
