// Package response provides standardized response and error handling utilities.
package response

// ErrorCode represents a unique application error identifier.
type ErrorCode string

// SuccessCode represents a unique application success identifier.
type SuccessCode string

// Success codes.
const (
	SuccessOK      SuccessCode = "000001" // request processed successfully
	SuccessCreated SuccessCode = "000002" // resource created successfully
)

// Client error codes — input and request validation failures.
const (
	ErrInvalidRequest       ErrorCode = "100001" // request body or params are invalid
	ErrMissingRequiredField ErrorCode = "100002" // required field is absent
	ErrInvalidVideoFormat   ErrorCode = "100003" // uploaded file is not a supported video format
	ErrFileTooLarge         ErrorCode = "100004" // file size exceeds the allowed limit
)

// Server error codes — internal processing failures.
const (
	ErrInternalServerError ErrorCode = "200001" // unexpected server-side error
	ErrFFmpegFailed        ErrorCode = "200002" // ffmpeg failed to process the video
	ErrWorkerUnavailable   ErrorCode = "200003" // worker service is unreachable
	ErrTranscodingFailed   ErrorCode = "200004" // transcoding completed with errors
)
