package response

import (
	"context"
	"fmt"
	"net/http"
	"runtime"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// WrapAppError creates an AppError with the given code, original error, and optional configurations.
// It captures the caller location automatically for debugging purposes.
func WrapAppError(ctx context.Context, originalErr error, code ErrorCode, msg string, opts ...AppErrorOption) *AppError {
	_, file, line, _ := runtime.Caller(1)
	appErr := &AppError{
		OriginalError: originalErr,
		ErrCode:       code,
		Location:      fmt.Sprintf("%s:%d", file, line),
	}
	for _, opt := range opts {
		opt(appErr)
	}
	return appErr
}

// ParseErrorWithHTTP converts an AppError into a ParsedError for REST response building.
func ParseErrorWithHTTP(err error) *ParsedError {
	if appErr, ok := err.(*AppError); ok {
		if d, found := getErrorDetail(appErr.ErrCode, appErr.Args...); found {
			return &ParsedError{
				HTTPCode:   d.HTTPStatus,
				ErrCode:    ErrorCode(d.Code),
				ErrMessage: d.Message,
				ErrPayload: appErr.ErrPayload,
			}
		}
		return &ParsedError{
			HTTPCode:   http.StatusInternalServerError,
			ErrCode:    appErr.ErrCode,
			ErrMessage: "unknown error",
			ErrPayload: appErr.ErrPayload,
		}
	}
	return &ParsedError{
		HTTPCode:   http.StatusInternalServerError,
		ErrCode:    ErrInternalServerError,
		ErrMessage: "unknown error",
		ErrPayload: struct{}{},
	}
}

// ParseErrorWithGRPC converts an AppError into a gRPC status error for gRPC response building.
func ParseErrorWithGRPC(err error) error {
	parsed := ParseErrorWithHTTP(err)
	return status.Error(httpStatusToGRPCCode(parsed.HTTPCode), parsed.ErrMessage)
}

// httpStatusToGRPCCode maps an HTTP status code to the nearest gRPC code equivalent.
func httpStatusToGRPCCode(httpStatus int) codes.Code {
	switch httpStatus {
	case http.StatusBadRequest:
		return codes.InvalidArgument
	case http.StatusNotFound:
		return codes.NotFound
	case http.StatusServiceUnavailable:
		return codes.Unavailable
	default:
		return codes.Internal
	}
}
