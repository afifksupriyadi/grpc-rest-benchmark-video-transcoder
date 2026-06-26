// Package rest provides a REST implementation of the WorkerClient interface.
package rest

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/internal/model"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/internal/service"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/lib/httpclient"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/shared/timing"
)

// RestWorkerClient implements service.WorkerClient using HTTP REST.
type RestWorkerClient struct {
	httpClient httpclient.Client
	workerURL  string
}

// NewRestWorkerClient creates a new RestWorkerClient with the given HTTP client and worker address.
func NewRestWorkerClient(httpClient httpclient.Client, workerURL string) service.WorkerClient {
	return &RestWorkerClient{
		httpClient: httpClient,
		workerURL:  workerURL,
	}
}

// ProcessVideo sends the video payload to the worker via HTTP POST, carrying t3 in the request header.
// It reads t5 back from the response header before parsing the chunked body.
func (c *RestWorkerClient) ProcessVideo(ctx context.Context, payload model.VideoPayload, t3 time.Time) (*model.TranscodeResult, time.Time, error) {
	req := &httpclient.Request{
		Method: http.MethodPost,
		URL:    fmt.Sprintf("http://%s/v1/process", c.workerURL),
		Headers: map[string]string{
			"Content-Type":         "application/octet-stream",
			"X-Video-Filename":     payload.Filename,
			timing.HeaderTimestamp: timing.EncodeTimestamp(t3),
		},
		Body: payload.Data,
	}

	resp, err := c.httpClient.Do(ctx, req)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("worker request failed: %w", err)
	}

	t5Str, ok := resp.Headers[timing.HeaderTimestamp]
	if !ok {
		return nil, time.Time{}, fmt.Errorf("missing timestamp header in worker response")
	}
	t5, err := timing.DecodeTimestamp(t5Str)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("invalid timestamp header in worker response: %w", err)
	}

	result, err := parseChunkedResponse(resp.Body)
	if err != nil {
		return nil, time.Time{}, err
	}

	return result, t5, nil
}

// parseChunkedResponse reads the chunked streaming response from the worker.
// It uses bufio.Reader to safely alternate between JSON header lines and binary data reads.
func parseChunkedResponse(body []byte) (*model.TranscodeResult, error) {
	reader := bufio.NewReader(bytes.NewReader(body))
	result := &model.TranscodeResult{}

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to read chunk header line: %w", err)
		}

		var header model.ChunkHeader
		if err := json.Unmarshal(line, &header); err != nil {
			return nil, fmt.Errorf("failed to decode chunk header: %w", err)
		}

		if header.Done || header.Size == 0 {
			continue
		}

		data := make([]byte, header.Size)
		if _, err := io.ReadFull(reader, data); err != nil {
			return nil, fmt.Errorf("failed to read chunk data for %s: %w", header.Resolution, err)
		}

		result.Outputs = append(result.Outputs, model.ResolutionOutput{
			Resolution: header.Resolution,
			Data:       data,
		})
	}

	return result, nil
}
