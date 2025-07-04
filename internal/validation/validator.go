package validation

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/robfig/cron/v3"
)

// UUID validation regex pattern
var uuidRegex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// IsValidUUID validates UUID format
func IsValidUUID(uuid string) bool {
	if uuid == "" {
		return false
	}
	return uuidRegex.MatchString(strings.ToLower(uuid))
}

// IsValidCronExpr validates cron expression using robfig/cron parser
func IsValidCronExpr(expr string) error {
	if expr == "" {
		return fmt.Errorf("cron expression cannot be empty")
	}

	// Use the same cron parser as the application (with seconds support)
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	_, err := parser.Parse(expr)
	if err != nil {
		return fmt.Errorf("invalid cron expression: %v", err)
	}

	return nil
}

// IsPositiveInteger validates if value is a positive integer
func IsPositiveInteger(value interface{}) bool {
	switch v := value.(type) {
	case int:
		return v > 0
	case int32:
		return v > 0
	case int64:
		return v > 0
	case float64:
		// Check if it's a whole number and positive
		return v > 0 && v == float64(int(v))
	case string:
		if num, err := strconv.Atoi(v); err == nil {
			return num > 0
		}
		return false
	default:
		return false
	}
}

// IsValidStrategyType validates strategy type
func IsValidStrategyType(strategyType string) bool {
	return strategyType == "single" || strategyType == "periodic"
}

// ValidatePageParams validates pagination parameters
func ValidatePageParams(page, pageSize int) (int, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		return 0, 0, fmt.Errorf("page size cannot exceed 100")
	}
	return page, pageSize, nil
}

// ValidateRequiredString validates required string field
func ValidateRequiredString(value, fieldName string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", fieldName)
	}
	return nil
}

// ValidateStringLength validates string length constraints
func ValidateStringLength(value, fieldName string, minLen, maxLen int) error {
	length := len(strings.TrimSpace(value))
	if length < minLen {
		return fmt.Errorf("%s must be at least %d characters", fieldName, minLen)
	}
	if maxLen > 0 && length > maxLen {
		return fmt.Errorf("%s must not exceed %d characters", fieldName, maxLen)
	}
	return nil
}

// ValidationError represents a validation error with field details
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors represents multiple validation errors
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	var messages []string
	for _, err := range e {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "; ")
}

// NewValidationError creates a new validation error
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{Field: field, Message: message}
}
