package utils

import (
	"time"

	"quota-manager/internal/config"
)

// DefaultTimezone 默认时区
const DefaultTimezone = "Asia/Shanghai"

// GetTimezone 获取配置的时区，如果配置无效则使用默认时区
func GetTimezone(cfg *config.Config) *time.Location {
	tz := cfg.Timezone
	if tz == "" {
		tz = DefaultTimezone
	}

	location, err := time.LoadLocation(tz)
	if err != nil {
		// 如果加载时区失败，使用默认时区
		location, err = time.LoadLocation(DefaultTimezone)
		if err != nil {
			// 如果默认时区也失败，使用UTC
			return time.UTC
		}
	}

	return location
}

// NowInConfigTimezone 获取配置时区的当前时间
func NowInConfigTimezone(cfg *config.Config) time.Time {
	return time.Now().In(GetTimezone(cfg))
}

// NowInDefaultTimezone 获取默认时区的当前时间
func NowInDefaultTimezone() time.Time {
	location, err := time.LoadLocation(DefaultTimezone)
	if err != nil {
		return time.Now().In(time.UTC)
	}
	return time.Now().In(location)
}

// ParseInConfigTimezone 在配置时区中解析时间字符串
func ParseInConfigTimezone(cfg *config.Config, layout, value string) (time.Time, error) {
	location := GetTimezone(cfg)
	return time.ParseInLocation(layout, value, location)
}

// FormatInConfigTimezone 在配置时区中格式化时间
func FormatInConfigTimezone(cfg *config.Config, t time.Time, layout string) string {
	return t.In(GetTimezone(cfg)).Format(layout)
}

// ToConfigTimezone 将时间转换到配置时区
func ToConfigTimezone(cfg *config.Config, t time.Time) time.Time {
	return t.In(GetTimezone(cfg))
}
