package utils

import (
	"time"
)

// CalculateExpiryDate calculates quota expiry date based on strategy configuration and base time
// Parameters:
//   - now: base time (usually current time)
//   - expiryDays: strategy configured expiry days, nil or 0 means use default end-of-month expiry
//
// Returns:
//   - calculated expiry time with time part fixed to 23:59:59
func CalculateExpiryDate(now time.Time, expiryDays *int) time.Time {
	if expiryDays != nil && *expiryDays > 0 {
		// Use strategy specified expiry days, time fixed to 23:59:59
		targetDate := now.AddDate(0, 0, *expiryDays)
		return time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 23, 59, 59, 0, now.Location())
	} else {
		// Use default end-of-month expiry
		return time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, now.Location())
	}
}
