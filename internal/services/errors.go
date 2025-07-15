package services

import "fmt"

// ServiceError represents custom error types for service operations
type ServiceError struct {
	Code    string
	Message string
}

func (e *ServiceError) Error() string {
	return e.Message
}

// Error constants for service operations
const (
	ErrorUserNotFound     = "user_not_found"
	ErrorDeptNotFound     = "department_not_found"
	ErrorDatabaseError    = "database_error"
	ErrorValidationFailed = "validation_failed"
	ErrorResourceNotFound = "resource_not_found"
	ErrorConflict         = "conflict"
)

// NewUserNotFoundError creates a new user not found error
func NewUserNotFoundError(employeeNumber string) *ServiceError {
	return &ServiceError{
		Code:    ErrorUserNotFound,
		Message: fmt.Sprintf("user not found: employee number '%s' does not exist", employeeNumber),
	}
}

// NewDepartmentNotFoundError creates a new department not found error
func NewDepartmentNotFoundError(departmentName string) *ServiceError {
	return &ServiceError{
		Code:    ErrorDeptNotFound,
		Message: fmt.Sprintf("department not found: no employees belong to department '%s'", departmentName),
	}
}

// NewDatabaseError creates a new database error
func NewDatabaseError(operation string, err error) *ServiceError {
	return &ServiceError{
		Code:    ErrorDatabaseError,
		Message: fmt.Sprintf("database error during %s: %v", operation, err),
	}
}

// NewValidationFailedError creates a new validation failed error
func NewValidationFailedError(message string) *ServiceError {
	return &ServiceError{
		Code:    ErrorValidationFailed,
		Message: message,
	}
}

// NewResourceNotFoundError creates a new resource not found error
func NewResourceNotFoundError(resourceType, identifier string) *ServiceError {
	return &ServiceError{
		Code:    ErrorResourceNotFound,
		Message: fmt.Sprintf("%s not found: %s", resourceType, identifier),
	}
}

// NewConflictError creates a new conflict error
func NewConflictError(message string) *ServiceError {
	return &ServiceError{
		Code:    ErrorConflict,
		Message: message,
	}
}
