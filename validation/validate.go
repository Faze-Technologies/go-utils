package validation

import (
	"errors"
	"fmt"

	appErrors "github.com/Faze-Technologies/go-utils/response"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func NewValidator() {
	validate = validator.New()
}

func formatValidationErrors(validationErrors validator.ValidationErrors) string {
	errMessages := make([]string, 0, len(validationErrors))
	for _, e := range validationErrors {
		errMessages = append(errMessages, fmt.Sprintf("field '%s' validation failed", e.Field()))
	}
	return fmt.Sprintf("validation failed: %v", errMessages)
}

// ValidateRequest validates request body and returns standardized error
func ValidateRequest(c *gin.Context, req interface{}) *appErrors.ServiceError {
	err := c.ShouldBindJSON(req)
	if err != nil {
		return appErrors.InvalidArgumentf("Invalid request body: %v", err)
	}

	if err = validate.Struct(req); err != nil {
		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) {
			errMsg := formatValidationErrors(validationErrors)
			return appErrors.ValidationFailed(errMsg)
		}
		return appErrors.InvalidArgumentf("Validation error: %v", err)
	}

	return nil
}

// ValidateQuery validates query parameters and returns standardized error
func ValidateQuery(c *gin.Context, req interface{}) *appErrors.ServiceError {
	err := c.ShouldBindQuery(req)
	if err != nil {
		return appErrors.InvalidArgumentf("Invalid query parameters: %v", err)
	}

	if err = validate.Struct(req); err != nil {
		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) {
			errMsg := formatValidationErrors(validationErrors)
			return appErrors.ValidationFailed(errMsg)
		}
		return appErrors.InvalidArgumentf("Query validation error: %v", err)
	}

	return nil
}

// ValidateURI validates URI parameters and returns standardized error
func ValidateURI(c *gin.Context, req interface{}) *appErrors.ServiceError {
	err := c.ShouldBindUri(req)
	if err != nil {
		return appErrors.InvalidArgumentf("Invalid URI parameters: %v", err)
	}

	if err = validate.Struct(req); err != nil {
		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) {
			errMsg := formatValidationErrors(validationErrors)
			return appErrors.ValidationFailed(errMsg)
		}
		return appErrors.InvalidArgumentf("URI validation error: %v", err)
	}

	return nil
}
