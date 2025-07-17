package request

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func SendServiceError(c *gin.Context, err *ServiceError) {
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
