package validation

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

// Custom validator instance
var schemaValidator *validator.Validate

func init() {
	// Always create our own validator instance to ensure proper configuration
	schemaValidator = validator.New()

	// Register custom validators first
	registerCustomValidators()

	// Set our configured validator as Gin's default validator
	binding.Validator = &defaultValidator{validate: schemaValidator}
}

// defaultValidator implements binding.StructValidator interface
type defaultValidator struct {
	validate *validator.Validate
}

func (v *defaultValidator) ValidateStruct(obj interface{}) error {
	return v.validate.Struct(obj)
}

func (v *defaultValidator) Engine() interface{} {
	return v.validate
}

// registerCustomValidators registers all custom validation functions
func registerCustomValidators() {
	// Register cron expression validator
	schemaValidator.RegisterValidation("cron", validateCron)

	// Register custom validators for permission management
	schemaValidator.RegisterValidation("employee_number", validateEmployeeNumber)
	schemaValidator.RegisterValidation("department_name", validateDepartmentName)
}

// validateCron validates cron expression using our existing function
func validateCron(fl validator.FieldLevel) bool {
	return IsValidCronExpr(fl.Field().String()) == nil
}

// validateEmployeeNumber validates employee number format
func validateEmployeeNumber(fl validator.FieldLevel) bool {
	employeeNumber := fl.Field().String()
	if len(employeeNumber) < 2 || len(employeeNumber) > 100 {
		return false
	}

	// Check if it contains only alphanumeric characters
	for _, char := range employeeNumber {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' || char == '-') {
			return false
		}
	}
	return true
}

// validateDepartmentName validates department name format
func validateDepartmentName(fl validator.FieldLevel) bool {
	departmentName := fl.Field().String()
	if len(departmentName) < 2 || len(departmentName) > 100 {
		return false
	}

	// Allow Chinese characters, English letters, digits, underscores, and hyphens
	for _, char := range departmentName {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '_' || char == '-' ||
			(char >= 0x4e00 && char <= 0x9fff)) { // Chinese characters range
			return false
		}
	}
	return true
}

// ValidateStruct validates a struct using schema tags
func ValidateStruct(s interface{}) error {
	if err := schemaValidator.Struct(s); err != nil {
		return formatValidationErrors(err)
	}
	return nil
}

// formatValidationErrors converts validator errors to our ValidationErrors format
func formatValidationErrors(err error) error {
	var validationErrors ValidationErrors

	if validatorErrors, ok := err.(validator.ValidationErrors); ok {
		for _, e := range validatorErrors {
			field := getJSONFieldName(e)
			message := getErrorMessage(e)
			validationErrors = append(validationErrors, ValidationError{
				Field:   field,
				Message: message,
			})
		}
	}

	return validationErrors
}

// getJSONFieldName extracts the JSON field name from validation error
func getJSONFieldName(err validator.FieldError) string {
	field := err.Field()

	// Try to get the struct field to extract JSON tag
	structField := err.StructField()
	if structField != "" {
		// Get the type of the struct containing this field
		if structType := err.StructNamespace(); structType != "" {
			// Parse the namespace to get the actual struct field
			parts := strings.Split(structType, ".")
			if len(parts) > 1 {
				fieldName := parts[len(parts)-1]

				// Try to find the field in the struct
				if jsonTag := getJSONTag(err); jsonTag != "" {
					return jsonTag
				}
				return strings.ToLower(fieldName)
			}
		}
	}

	return strings.ToLower(field)
}

// getJSONTag tries to extract JSON tag from struct field
func getJSONTag(err validator.FieldError) string {
	// This is a simplified approach. In real implementation, you might need
	// to use reflection to get the actual struct field and its JSON tag
	structField := err.StructField()
	if structField != "" {
		// For now, return snake_case version of field name
		return toSnakeCase(structField)
	}
	return ""
}

// toSnakeCase converts camelCase to snake_case
func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

// getErrorMessage generates user-friendly error messages for validation errors
func getErrorMessage(err validator.FieldError) string {
	field := err.Field()
	tag := err.Tag()
	param := err.Param()

	switch tag {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "uuid":
		return fmt.Sprintf("%s must be a valid UUID format", field)
	case "cron":
		return fmt.Sprintf("%s must be a valid cron expression", field)
	case "strategy_type":
		return fmt.Sprintf("%s must be 'single' or 'periodic'", field)
	case "positive":
		return fmt.Sprintf("%s must be a positive number", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", field, param)
	case "max":
		return fmt.Sprintf("%s must not exceed %s characters", field, param)
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", field, param)
	case "employee_number":
		return fmt.Sprintf("%s must be 2-20 characters long and contain only alphanumeric characters", field)
	case "department_name":
		return fmt.Sprintf("%s must be 2-100 characters long and contain only letters, digits, underscores, and hyphens", field)
	case "dive":
		return fmt.Sprintf("Invalid item in %s", field)
	default:
		return fmt.Sprintf("%s is invalid", field)
	}
}

// GetValidator returns the validator instance for advanced usage
func GetValidator() *validator.Validate {
	return schemaValidator
}
