package httpclient

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

type RestyClient struct {
	client *resty.Client
}

func NewRestyClient(timeout time.Duration) *RestyClient {
	return &RestyClient{
		client: resty.New().SetTimeout(timeout),
	}
}

func (c *RestyClient) Do(ctx context.Context, req *Request) (*Response, error) {
	r := c.client.R().
		SetContext(ctx).
		SetHeaders(req.Headers).
		SetBody(req.Body)

	res, err := r.Execute(req.Method, req.URL)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}

	// normalize header keys to lowercase so lookups via plain map indexing
	// (e.g. resp.Headers[timing.HeaderTimestamp]) work regardless of how
	// Go's net/http canonicalizes the header name on the wire
	headers := make(map[string]string, len(res.Header()))
	for k, v := range res.Header() {
		if len(v) > 0 {
			headers[strings.ToLower(k)] = v[0]
		}
	}

	return &Response{
		StatusCode: res.StatusCode(),
		Headers:    headers,
		Body:       res.Body(),
	}, nil
}
