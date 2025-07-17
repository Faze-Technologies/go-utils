package request

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func NewValidator() {
	validate = validator.New()
}

func formatValidateErrors(validationErrors *validator.ValidationErrors) string {
	errRes := make([]error, 0)
	for _, e := range *validationErrors {
		errRes = append(errRes, fmt.Errorf("invalid value field %s", e.Field()))
	}
	return errors.Join(errRes...).Error()
}

func ValidateRequest(c *gin.Context, req interface{}) *ServiceError {
	err := c.ShouldBindJSON(req)
	if err != nil {
		return CreateBadRequestError(err, "Invalid Request Body")
	}
	if err = validate.Struct(req); err != nil {
		var validationErrors validator.ValidationErrors
		errors.As(err, &validationErrors)
		errRes := formatValidateErrors(&validationErrors)
		return CreateBadRequestError(nil, errRes)
	}
	return nil
}
