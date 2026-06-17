// Package httpclient provides an HTTP client interface and its implementations.
package httpclient

import "context"

// Request holds the data needed to perform an HTTP request.
type Request struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    []byte
}

// Response holds the data returned from an HTTP request.
type Response struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}

// Client defines the interface for making HTTP requests.
type Client interface {
	Do(ctx context.Context, req *Request) (*Response, error)
}
