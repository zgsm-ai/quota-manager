package utils

import (
	"time"

	"quota-manager/internal/config"
)

// DefaultTimezone default timezone
const DefaultTimezone = "Asia/Shanghai"

// GetTimezone gets the configured timezone, uses default timezone if configuration is invalid
func GetTimezone(cfg *config.Config) *time.Location {
	tz := cfg.Timezone
	if tz == "" {
		tz = DefaultTimezone
	}

	location, err := time.LoadLocation(tz)
	if err != nil {
		// If loading timezone fails, use default timezone
		location, err = time.LoadLocation(DefaultTimezone)
		if err != nil {
			// If default timezone also fails, use UTC
			return time.UTC
		}
	}

	return location
}

// NowInConfigTimezone gets current time in configured timezone
func NowInConfigTimezone(cfg *config.Config) time.Time {
	return time.Now().In(GetTimezone(cfg))
}

// NowInDefaultTimezone gets current time in default timezone
func NowInDefaultTimezone() time.Time {
	location, err := time.LoadLocation(DefaultTimezone)
	if err != nil {
		return time.Now().In(time.UTC)
	}
	return time.Now().In(location)
}

// ParseInConfigTimezone parses time string in configured timezone
func ParseInConfigTimezone(cfg *config.Config, layout, value string) (time.Time, error) {
	location := GetTimezone(cfg)
	return time.ParseInLocation(layout, value, location)
}

// FormatInConfigTimezone formats time in configured timezone
func FormatInConfigTimezone(cfg *config.Config, t time.Time, layout string) string {
	return t.In(GetTimezone(cfg)).Format(layout)
}

// ToConfigTimezone converts time to configured timezone
func ToConfigTimezone(cfg *config.Config, t time.Time) time.Time {
	return t.In(GetTimezone(cfg))
}
