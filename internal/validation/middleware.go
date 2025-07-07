package validation

import (
	"net/http"
	"quota-manager/internal/response"

	"github.com/gin-gonic/gin"
)

// ValidationMiddleware creates a middleware function for request validation
func ValidationMiddleware() gin.HandlerFunc {
	return gin.HandlerFunc(func(c *gin.Context) {
		c.Next()
	})
}

// ValidateJSON validates JSON request body and binds it to the provided struct
func ValidateJSON(c *gin.Context, obj interface{}) error {
	// First bind the JSON
	if err := c.ShouldBindJSON(obj); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "Invalid request body: "+err.Error()))
		return err
	}

	// Then validate using schema
	if err := ValidateStruct(obj); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, err.Error()))
		return err
	}

	return nil
}

// ValidateQuery validates query parameters and binds them to the provided struct
func ValidateQuery(c *gin.Context, obj interface{}) error {
	// First bind the query parameters
	if err := c.ShouldBindQuery(obj); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "Invalid query parameters: "+err.Error()))
		return err
	}

	// Then validate using schema
	if err := ValidateStruct(obj); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, err.Error()))
		return err
	}

	return nil
}

// ValidateURI validates URI parameters and binds them to the provided struct
func ValidateURI(c *gin.Context, obj interface{}) error {
	// First bind the URI parameters
	if err := c.ShouldBindUri(obj); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, "Invalid URI parameters: "+err.Error()))
		return err
	}

	// Then validate using schema
	if err := ValidateStruct(obj); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, err.Error()))
		return err
	}

	return nil
}

// ValidationHelper provides helper methods for manual validation
type ValidationHelper struct{}

// NewValidationHelper creates a new validation helper
func NewValidationHelper() *ValidationHelper {
	return &ValidationHelper{}
}

// ValidateAndRespond validates a struct and sends error response if validation fails
func (vh *ValidationHelper) ValidateAndRespond(c *gin.Context, obj interface{}) bool {
	if err := ValidateStruct(obj); err != nil {
		c.JSON(http.StatusBadRequest, response.NewErrorResponse(response.BadRequestCode, err.Error()))
		return false
	}
	return true
}

// Custom validation functions for specific business logic

// ValidateStrategyForType validates strategy fields based on type
func ValidateStrategyForType(strategy interface{}) error {
	// This can be used for custom cross-field validation
	// For example, ensuring periodic strategies have periodic_expr
	return ValidateStruct(strategy)
}
