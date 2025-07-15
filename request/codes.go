package request

type ErrorCode string

const (
	KeyNotFoundError        ErrorCode = "keyNotFoundError"
	BadRequestError         ErrorCode = "badRequestError"
	InternalServerError     ErrorCode = "internalServerError"
	NoError                 ErrorCode = "noError"
	CanceledError           ErrorCode = "canceledError"
	UnknownError            ErrorCode = "unknownError"
	InvalidArgumentError    ErrorCode = "invalidArgumentError"
	DeadlineExceededError   ErrorCode = "deadlineExceededError"
	UnauthorizedError       ErrorCode = "unauthorizedError"
	NotFoundError           ErrorCode = "notFoundError"
	AlreadyExistsError      ErrorCode = "alreadyExistsError"
	PermissionDeniedError   ErrorCode = "permissionDeniedError"
	ResourceExhaustedError  ErrorCode = "resourceExhaustedError"
	FailedPreconditionError ErrorCode = "failedPreconditionError"
	AbortedError            ErrorCode = "abortedError"
	OutOfRangeError         ErrorCode = "outOfRangeError"
	UnimplementedError      ErrorCode = "unimplementedError"
	ServiceUnavailableError ErrorCode = "serviceUnavailableError"
	DataLossError           ErrorCode = "dataLossError"
	MFARequiredError        ErrorCode = "mfaRequiredError"
	VerificationError       ErrorCode = "verificationRequiredError"
)
