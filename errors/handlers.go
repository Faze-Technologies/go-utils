package errors

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

// SendHTTPError sends an HTTP error response using Gin
func SendHTTPError(c *gin.Context, err *ServiceError) {
	statusCode := err.ToHTTPStatus()
	response := err.ToHTTPResponse()
	c.JSON(statusCode, response)
}

// SendHTTPSuccess sends an HTTP success response using Gin
func SendHTTPSuccess(c *gin.Context, data interface{}) {
	response := &HTTPResponse{
		Success: true,
		Data:    data,
	}
	c.JSON(http.StatusOK, response)
}

// SendHTTPCreated sends an HTTP created response using Gin
func SendHTTPCreated(c *gin.Context, data interface{}) {
	response := &HTTPResponse{
		Success: true,
		Data:    data,
	}
	c.JSON(http.StatusCreated, response)
}

// WriteHTTPError writes an HTTP error response using standard http.ResponseWriter
func WriteHTTPError(w http.ResponseWriter, err *ServiceError) {
	statusCode := err.ToHTTPStatus()
	response := err.ToHTTPResponse()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if jsonErr := json.NewEncoder(w).Encode(response); jsonErr != nil {
		// Fallback to plain text if JSON encoding fails
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}
}

// WriteHTTPSuccess writes an HTTP success response using standard http.ResponseWriter
func WriteHTTPSuccess(w http.ResponseWriter, data interface{}) {
	response := &HTTPResponse{
		Success: true,
		Data:    data,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if jsonErr := json.NewEncoder(w).Encode(response); jsonErr != nil {
		// Fallback to plain text if JSON encoding fails
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}
}

// ToGRPCError is a convenience function to convert ServiceError to gRPC error
// This is the function that should be used in gRPC handlers
func ToGRPCError(err *ServiceError) error {
	if err == nil {
		return nil
	}
	return err.ToGRPCError()
}
