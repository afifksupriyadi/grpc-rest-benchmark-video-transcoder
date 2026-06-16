package response

// Response wraps the HTTP status code and response body for all API responses.
type Response struct {
	Status int         `json:"status"`
	Body   interface{} `json:"body"`
}

// Body represents the standard structure for all response bodies.
type Body struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// AppError represents an application-level error with context for logging and response building.
// It is populated by the service layer and passed up to the handler layer.
// - ErrCode identifies the error type via the code dictionary
// - Location captures where the error originated for debugging
type AppError struct {
	OriginalError error
	ErrCode       ErrorCode
	Location      string
	Args          []interface{}
	ErrPayload    interface{}
}

// Error implements the error interface for AppError.
func (e *AppError) Error() string {
	if e.OriginalError != nil {
		return e.OriginalError.Error()
	}
	return "unknown error"
}

// ParsedError represents an error that has been resolved against the dictionary.
// It holds the final HTTP code and message ready for response building.
type ParsedError struct {
	HTTPCode   int
	ErrCode    ErrorCode
	ErrMessage string
	ErrPayload interface{}
}

// AppErrorOption defines a functional option for configuring AppError.
type AppErrorOption func(*AppError)

// WithPayload attaches a structured payload to the error response body.
func WithPayload(payload interface{}) AppErrorOption {
	return func(a *AppError) {
		a.ErrPayload = payload
	}
}

// WithArgs injects dynamic values into the error message template defined in the dictionary.
func WithArgs(args ...interface{}) AppErrorOption {
	return func(a *AppError) {
		a.Args = args
	}
}
