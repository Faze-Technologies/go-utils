package request

import "net/http"

type ServiceError struct {
	error
	HttpStatus int
	Message    string
	ErrorCode  ErrorCode
	Data       interface{}
}

func CreateBadRequestError(err error, message string) *ServiceError {
	sErr := ServiceError{}
	statusCode := http.StatusBadRequest
	errorCode := BadRequestError
	sErr.generateCustomError(statusCode, errorCode, message, err, nil)
	return &sErr
}

func CreateInternalServerError(err error) *ServiceError {
	sErr := ServiceError{}
	statusCode := http.StatusInternalServerError
	errorCode := InternalServerError
	sErr.generateCustomError(statusCode, errorCode, "Internal Server Error", err, nil)
	return &sErr
}

func CreateConflictError(err error, message string) *ServiceError {
	sErr := ServiceError{}
	statusCode := http.StatusConflict
	errorCode := AlreadyExistsError
	sErr.generateCustomError(statusCode, errorCode, message, err, nil)
	return &sErr
}

func CreateNotFoundError(err error, message string) *ServiceError {
	sErr := ServiceError{}
	statusCode := http.StatusNotFound
	errorCode := NotFoundError
	sErr.generateCustomError(statusCode, errorCode, message, err, nil)
	return &sErr
}

func CreateTooManyRequestsError(err error, message string) *ServiceError {
	sErr := ServiceError{}
	statusCode := http.StatusNotFound
	errorCode := ResourceExhaustedError
	sErr.generateCustomError(statusCode, errorCode, message, err, nil)
	return &sErr
}

func CreateUnauthorizedError(err error, message string) *ServiceError {
	sErr := ServiceError{}
	statusCode := http.StatusUnauthorized
	errorCode := UnauthorizedError
	sErr.generateCustomError(statusCode, errorCode, message, err, nil)
	return &sErr
}

func (e *ServiceError) generateCustomError(statusCode int, errorCode ErrorCode, message string, err error, data interface{}) {
	e.HttpStatus = statusCode
	if e.error != nil && e.error.Error() != "" {
		e.Message = e.Error()
	}
	if message != "" {
		e.Message = message
	}
	if data != nil {
		e.Data = data
	}
	e.ErrorCode = errorCode
	e.error = err
}

func (e *ServiceError) GetError() error {
	return e.error
}

func (e *ServiceError) Error() string {
	return e.error.Error()
}
