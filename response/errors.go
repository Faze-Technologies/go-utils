package response

import (
	"fmt"
)

// ErrorCode represents standardized error codes
type ErrorCode string

const (
	// Standard error codes that map to both HTTP and gRPC
	CodeOK                 ErrorCode = "OK"
	CodeCanceled           ErrorCode = "CANCELED"
	CodeUnknown            ErrorCode = "UNKNOWN"
	CodeInvalidArgument    ErrorCode = "INVALID_ARGUMENT"
	CodeDeadlineExceeded   ErrorCode = "DEADLINE_EXCEEDED"
	CodeNotFound           ErrorCode = "NOT_FOUND"
	CodeAlreadyExists      ErrorCode = "ALREADY_EXISTS"
	CodePermissionDenied   ErrorCode = "PERMISSION_DENIED"
	CodeResourceExhausted  ErrorCode = "RESOURCE_EXHAUSTED"
	CodeFailedPrecondition ErrorCode = "FAILED_PRECONDITION"
	CodeAborted            ErrorCode = "ABORTED"
	CodeOutOfRange         ErrorCode = "OUT_OF_RANGE"
	CodeUnimplemented      ErrorCode = "UNIMPLEMENTED"
	CodeInternal           ErrorCode = "INTERNAL"
	CodeUnavailable        ErrorCode = "UNAVAILABLE"
	CodeDataLoss           ErrorCode = "DATA_LOSS"
	CodeUnauthenticated    ErrorCode = "UNAUTHENTICATED"

	// Custom business logic errors
	CodeValidationFailed      ErrorCode = "VALIDATION_FAILED"
	CodeBusinessRuleViolation ErrorCode = "BUSINESS_RULE_VIOLATION"
	CodeMFARequired           ErrorCode = "MFA_REQUIRED"
	CodeVerificationRequired  ErrorCode = "VERIFICATION_REQUIRED"
)

// ServiceError represents a standardized error that can be converted to HTTP or gRPC
type ServiceError struct {
	Code        ErrorCode              `json:"code"`
	Message     string                 `json:"message"`
	ErrorCode   int                    `json:"errorCode,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Cause       error                  `json:"-"` // Original error, not serialized
}

// Error implements the error interface
func (e *ServiceError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// WithDetails adds additional context to the error
func (e *ServiceError) WithDetails(key string, value interface{}) *ServiceError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// WithErrorCode sets a numeric error code for frontend identification
func (e *ServiceError) WithErrorCode(code int) *ServiceError {
	e.ErrorCode = code
	return e
}

// WithCause sets the underlying cause of the error
func (e *ServiceError) WithCause(err error) *ServiceError {
	e.Cause = err
	return e
}

// New creates a new ServiceError with an optional numeric error code
func New(code ErrorCode, message string, errorCode ...int) *ServiceError {
	e := &ServiceError{
		Code:    code,
		Message: message,
		Details: make(map[string]interface{}),
	}
	if len(errorCode) > 0 {
		e.ErrorCode = errorCode[0]
	}
	return e
}

// Newf creates a new ServiceError with formatted message
func Newf(code ErrorCode, format string, args ...interface{}) *ServiceError {
	return &ServiceError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
		Details: make(map[string]interface{}),
	}
}

// Wrap creates a ServiceError wrapping an existing error
func Wrap(code ErrorCode, message string, err error) *ServiceError {
	return &ServiceError{
		Code:    code,
		Message: message,
		Details: make(map[string]interface{}),
		Cause:   err,
	}
}

// Wrapf creates a ServiceError wrapping an existing error with formatted message
func Wrapf(code ErrorCode, err error, format string, args ...interface{}) *ServiceError {
	return &ServiceError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
		Details: make(map[string]interface{}),
		Cause:   err,
	}
}

// Common error constructors for frequently used errors

// NotFound creates a not found error
func NotFound(message string) *ServiceError {
	return New(CodeNotFound, message)
}

// NotFoundf creates a not found error with formatted message
func NotFoundf(format string, args ...interface{}) *ServiceError {
	return Newf(CodeNotFound, format, args...)
}

// InvalidArgument creates an invalid argument error
func InvalidArgument(message string) *ServiceError {
	return New(CodeInvalidArgument, message)
}

// InvalidArgumentf creates an invalid argument error with formatted message
func InvalidArgumentf(format string, args ...interface{}) *ServiceError {
	return Newf(CodeInvalidArgument, format, args...)
}

// Internal creates an internal error
func Internal(message string) *ServiceError {
	return New(CodeInternal, message)
}

// Internalf creates an internal error with formatted message
func Internalf(format string, args ...interface{}) *ServiceError {
	return Newf(CodeInternal, format, args...)
}

// InternalWrap creates an internal error wrapping another error
func InternalWrap(err error, message string) *ServiceError {
	return Wrap(CodeInternal, message, err)
}

// InternalWrapf creates an internal error wrapping another error with formatted message
func InternalWrapf(err error, format string, args ...interface{}) *ServiceError {
	return Wrapf(CodeInternal, err, format, args...)
}

// AlreadyExists creates an already exists error
func AlreadyExists(message string) *ServiceError {
	return New(CodeAlreadyExists, message)
}

// AlreadyExistsf creates an already exists error with formatted message
func AlreadyExistsf(format string, args ...interface{}) *ServiceError {
	return Newf(CodeAlreadyExists, format, args...)
}

// PermissionDenied creates a permission denied error
func PermissionDenied(message string) *ServiceError {
	return New(CodePermissionDenied, message)
}

// PermissionDeniedf creates a permission denied error with formatted message
func PermissionDeniedf(format string, args ...interface{}) *ServiceError {
	return Newf(CodePermissionDenied, format, args...)
}

// Unauthenticated creates an unauthenticated error
func Unauthenticated(message string) *ServiceError {
	return New(CodeUnauthenticated, message)
}

// Unauthenticatedf creates an unauthenticated error with formatted message
func Unauthenticatedf(format string, args ...interface{}) *ServiceError {
	return Newf(CodeUnauthenticated, format, args...)
}

// ValidationFailed creates a validation failed error
func ValidationFailed(message string) *ServiceError {
	return New(CodeValidationFailed, message)
}

// ValidationFailedf creates a validation failed error with formatted message
func ValidationFailedf(format string, args ...interface{}) *ServiceError {
	return Newf(CodeValidationFailed, format, args...)
}

// BusinessRuleViolation creates a business rule violation error
func BusinessRuleViolation(message string) *ServiceError {
	return New(CodeBusinessRuleViolation, message)
}

// BusinessRuleViolationf creates a business rule violation error with formatted message
func BusinessRuleViolationf(format string, args ...interface{}) *ServiceError {
	return Newf(CodeBusinessRuleViolation, format, args...)
}

// MFARequired creates an MFA required error
func MFARequired(message string) *ServiceError {
	return New(CodeMFARequired, message)
}

// VerificationRequired creates a verification required error
func VerificationRequired(message string) *ServiceError {
	return New(CodeVerificationRequired, message)
}
