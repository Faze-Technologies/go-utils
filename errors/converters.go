package errors

import (
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ToHTTPStatus converts a ServiceError to HTTP status code
func (e *ServiceError) ToHTTPStatus() int {
	switch e.Code {
	case CodeOK:
		return http.StatusOK
	case CodeCanceled:
		return http.StatusRequestTimeout
	case CodeUnknown:
		return http.StatusInternalServerError
	case CodeInvalidArgument:
		return http.StatusBadRequest
	case CodeDeadlineExceeded:
		return http.StatusRequestTimeout
	case CodeNotFound:
		return http.StatusNotFound
	case CodeAlreadyExists:
		return http.StatusConflict
	case CodePermissionDenied:
		return http.StatusForbidden
	case CodeResourceExhausted:
		return http.StatusTooManyRequests
	case CodeFailedPrecondition:
		return http.StatusPreconditionFailed
	case CodeAborted:
		return http.StatusConflict
	case CodeOutOfRange:
		return http.StatusBadRequest
	case CodeUnimplemented:
		return http.StatusNotImplemented
	case CodeInternal:
		return http.StatusInternalServerError
	case CodeUnavailable:
		return http.StatusServiceUnavailable
	case CodeDataLoss:
		return http.StatusInternalServerError
	case CodeUnauthenticated:
		return http.StatusUnauthorized
	case CodeValidationFailed:
		return http.StatusBadRequest
	case CodeBusinessRuleViolation:
		return http.StatusUnprocessableEntity
	case CodeMFARequired:
		return http.StatusPreconditionRequired
	case CodeVerificationRequired:
		return http.StatusPreconditionRequired
	default:
		return http.StatusInternalServerError
	}
}

// ToGRPCStatus converts a ServiceError to gRPC status
func (e *ServiceError) ToGRPCStatus() *status.Status {
	grpcCode := e.toGRPCCode()
	return status.New(grpcCode, e.Message)
}

// ToGRPCError converts a ServiceError to gRPC error
func (e *ServiceError) ToGRPCError() error {
	return e.ToGRPCStatus().Err()
}

// toGRPCCode maps ServiceError codes to gRPC codes
func (e *ServiceError) toGRPCCode() codes.Code {
	switch e.Code {
	case CodeOK:
		return codes.OK
	case CodeCanceled:
		return codes.Canceled
	case CodeUnknown:
		return codes.Unknown
	case CodeInvalidArgument:
		return codes.InvalidArgument
	case CodeDeadlineExceeded:
		return codes.DeadlineExceeded
	case CodeNotFound:
		return codes.NotFound
	case CodeAlreadyExists:
		return codes.AlreadyExists
	case CodePermissionDenied:
		return codes.PermissionDenied
	case CodeResourceExhausted:
		return codes.ResourceExhausted
	case CodeFailedPrecondition:
		return codes.FailedPrecondition
	case CodeAborted:
		return codes.Aborted
	case CodeOutOfRange:
		return codes.OutOfRange
	case CodeUnimplemented:
		return codes.Unimplemented
	case CodeInternal:
		return codes.Internal
	case CodeUnavailable:
		return codes.Unavailable
	case CodeDataLoss:
		return codes.DataLoss
	case CodeUnauthenticated:
		return codes.Unauthenticated
	case CodeValidationFailed:
		return codes.InvalidArgument
	case CodeBusinessRuleViolation:
		return codes.FailedPrecondition
	case CodeMFARequired:
		return codes.FailedPrecondition
	case CodeVerificationRequired:
		return codes.FailedPrecondition
	default:
		return codes.Unknown
	}
}

// HTTPResponse represents the JSON structure for HTTP error responses
type HTTPResponse struct {
	Success bool                   `json:"success"`
	Error   *HTTPErrorDetails      `json:"error,omitempty"`
	Data    interface{}            `json:"data,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// HTTPErrorDetails represents error details in HTTP responses
type HTTPErrorDetails struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

// ToHTTPResponse converts a ServiceError to HTTP response structure
func (e *ServiceError) ToHTTPResponse() *HTTPResponse {
	response := &HTTPResponse{
		Success: false,
		Error: &HTTPErrorDetails{
			Code:    e.Code,
			Message: e.Message,
		},
	}

	if len(e.Details) > 0 {
		response.Details = e.Details
	}

	return response
}
