package httpclient

import (
	"context"
	"fmt"
	"time"

	"github.com/go-resty/resty/v2"
)

// RestyClient implements Client using the go-resty library.
type RestyClient struct {
	client *resty.Client
}

// NewRestyClient creates a new RestyClient with the given timeout.
func NewRestyClient(timeout time.Duration) *RestyClient {
	return &RestyClient{
		client: resty.New().SetTimeout(timeout),
	}
}

// Do executes an HTTP request and returns the response.
func (c *RestyClient) Do(ctx context.Context, req *Request) (*Response, error) {
	r := c.client.R().
		SetContext(ctx).
		SetHeaders(req.Headers).
		SetBody(req.Body)

	res, err := r.Execute(req.Method, req.URL)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}

	headers := make(map[string]string, len(res.Header()))
	for k, v := range res.Header() {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	return &Response{
		StatusCode: res.StatusCode(),
		Headers:    headers,
		Body:       res.Body(),
	}, nil
}
