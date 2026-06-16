package response

import "context"

// BuildSuccess constructs a standardized success Response from the given data and success code.
func BuildSuccess(ctx context.Context, data interface{}, code SuccessCode) *Response {
	d, found := getSuccessDetail(code)
	if !found {
		return &Response{
			Status: 200,
			Body:   buildBody(string(code), "success", data),
		}
	}
	return &Response{
		Status: d.HTTPStatus,
		Body:   buildBody(d.Code, d.Message, data),
	}
}

// BuildError constructs a standardized error Response from the given error.
func BuildError(ctx context.Context, err error) *Response {
	parsed := ParseErrorWithHTTP(err)
	payload := parsed.ErrPayload
	if payload == nil {
		payload = struct{}{}
	}
	return &Response{
		Status: parsed.HTTPCode,
		Body:   buildBody(string(parsed.ErrCode), parsed.ErrMessage, payload),
	}
}

// buildBody assembles the standard Body struct used in all responses.
func buildBody(code, message string, data interface{}) Body {
	return Body{
		Code:    code,
		Message: message,
		Data:    data,
	}
}
