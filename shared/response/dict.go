package response

import (
	"fmt"
	"net/http"
)

// detail holds the resolved message and HTTP status for a given code.
type detail struct {
	Code       string
	Message    string
	HTTPStatus int
}

// ErrorDictionary maps each ErrorCode to its detail.
var ErrorDictionary = map[ErrorCode]detail{}

// SuccessDictionary maps each SuccessCode to its detail.
var SuccessDictionary = map[SuccessCode]detail{}

// init registers all error and success codes into their respective dictionaries.
func init() {
	registerErrors([]detail{
		// Client errors
		{string(ErrInvalidRequest), "Invalid request", http.StatusBadRequest},
		{string(ErrMissingRequiredField), "%s is required", http.StatusBadRequest},
		{string(ErrInvalidVideoFormat), "Invalid video format", http.StatusBadRequest},
		{string(ErrFileTooLarge), "File size exceeds the allowed limit", http.StatusBadRequest},

		// Server errors
		{string(ErrInternalServerError), "An internal server error occurred", http.StatusInternalServerError},
		{string(ErrFFmpegFailed), "FFmpeg failed to process the video", http.StatusInternalServerError},
		{string(ErrWorkerUnavailable), "Worker service is unreachable", http.StatusServiceUnavailable},
		{string(ErrTranscodingFailed), "Transcoding completed with errors", http.StatusInternalServerError},
	})

	registerSuccesses([]detail{
		{string(SuccessOK), "Request processed successfully", http.StatusOK},
		{string(SuccessCreated), "Resource created successfully", http.StatusCreated},
	})
}

// registerErrors adds a list of error details into ErrorDictionary.
func registerErrors(list []detail) {
	for _, d := range list {
		ErrorDictionary[ErrorCode(d.Code)] = d
	}
}

// registerSuccesses adds a list of success details into SuccessDictionary.
func registerSuccesses(list []detail) {
	for _, d := range list {
		SuccessDictionary[SuccessCode(d.Code)] = d
	}
}

// getErrorDetail looks up an ErrorCode in the dictionary and formats its message with optional args.
func getErrorDetail(code ErrorCode, args ...interface{}) (*detail, bool) {
	return getDetail(ErrorDictionary, code, args...)
}

// getSuccessDetail looks up a SuccessCode in the dictionary and formats its message with optional args.
func getSuccessDetail(code SuccessCode, args ...interface{}) (*detail, bool) {
	return getDetail(SuccessDictionary, code, args...)
}

// getDetail is a generic lookup helper for both error and success dictionaries.
func getDetail[T comparable](dict map[T]detail, code T, args ...interface{}) (*detail, bool) {
	d, ok := dict[code]
	if !ok {
		return nil, false
	}
	if len(args) > 0 {
		d.Message = fmt.Sprintf(d.Message, args...)
	}
	return &d, true
}
