package validation

import (
	"fmt"
	"strings"

	"github.com/robfig/cron/v3"
)

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

// ValidatePageParams validates pagination parameters
func ValidatePageParams(page, pageSize int) (int, int, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	// if pageSize > 100 {
	// 	return 0, 0, fmt.Errorf("page size cannot exceed 100")
	// }
	return page, pageSize, nil
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
