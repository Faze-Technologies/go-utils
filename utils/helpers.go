package utils

import (
	"errors"
	"fmt"
	"github.com/Faze-Technologies/go-utils/data"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"net/http"
)

func SendServiceError(c *gin.Context, err *data.ServiceError) {
	c.JSON(err.HttpStatus, gin.H{
		"status":  "error",
		"error":   err.ErrorCode,
		"message": err.Message,
		"data":    err.Data,
	})
}

func SendSuccessResponse(c *gin.Context, res interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"data":   res,
	})
}

func FormatValidateErrors(validationErrors *validator.ValidationErrors) string {
	errRes := make([]error, 0)
	for _, e := range *validationErrors {
		errRes = append(errRes, fmt.Errorf("invalid value field %s", e.Field()))
	}
	return errors.Join(errRes...).Error()
}
