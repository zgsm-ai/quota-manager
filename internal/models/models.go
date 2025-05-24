package models

import (
	"time"

	"gorm.io/gorm"
)

// QuotaStrategy 策略表结构
type QuotaStrategy struct {
	ID           int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Name         string    `gorm:"uniqueIndex;not null" json:"name"`
	Title        string    `gorm:"not null" json:"title"`
	Type         string    `gorm:"not null" json:"type"` // periodic/single
	Amount       int       `gorm:"not null" json:"amount"`
	Model        string    `gorm:"not null" json:"model"`
	PeriodicExpr string    `gorm:"column:periodic_expr" json:"periodic_expr"`
	Condition    string    `json:"condition"`
	CreateTime   time.Time `gorm:"autoCreateTime" json:"create_time"`
	UpdateTime   time.Time `gorm:"autoUpdateTime" json:"update_time"`
}

// QuotaExecute 执行状态表
type QuotaExecute struct {
	ID          int       `gorm:"primaryKey;autoIncrement" json:"id"`
	StrategyID  int       `gorm:"not null;index" json:"strategy_id"`
	User        string    `gorm:"not null;index" json:"user"`
	BatchNumber string    `gorm:"not null;index" json:"batch_number"`
	Status      string    `gorm:"not null" json:"status"`
	CreateTime  time.Time `gorm:"autoCreateTime" json:"create_time"`
	UpdateTime  time.Time `gorm:"autoUpdateTime" json:"update_time"`
}

// UserInfo 用户信息表
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

// TableName 设置表名
func (QuotaStrategy) TableName() string {
	return "quota_strategy"
}

func (QuotaExecute) TableName() string {
	return "quota_execute"
}

func (UserInfo) TableName() string {
	return "user_info"
}

// AutoMigrate 自动迁移数据库表
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&QuotaStrategy{}, &QuotaExecute{}, &UserInfo{})
}