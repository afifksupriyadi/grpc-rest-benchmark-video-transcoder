// Package rest provides a REST implementation of the WorkerClient interface.
package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/internal/model"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/internal/service"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/gateway/lib/httpclient"
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

// ProcessVideo sends the video payload to the worker via HTTP POST and reads the chunked response.
// It reads ChunkHeader JSON before each binary chunk to identify the resolution and size.
func (c *RestWorkerClient) ProcessVideo(ctx context.Context, payload model.VideoPayload) (*model.TranscodeResult, error) {
	req := &httpclient.Request{
		Method: http.MethodPost,
		URL:    fmt.Sprintf("%s/process", c.workerURL),
		Headers: map[string]string{
			"Content-Type":     "application/octet-stream",
			"X-Video-Filename": payload.Filename,
		},
		Body: payload.Data,
	}

	resp, err := c.httpClient.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("worker request failed: %w", err)
	}

	return parseChunkedResponse(resp.Body)
}

// parseChunkedResponse reads the chunked streaming response from the worker.
// It reads alternating ChunkHeader (JSON) and binary data until all resolutions are received.
func parseChunkedResponse(body []byte) (*model.TranscodeResult, error) {
	reader := bytes.NewReader(body)
	result := &model.TranscodeResult{}

	for {
		// read chunk header
		var header model.ChunkHeader
		decoder := json.NewDecoder(reader)
		if err := decoder.Decode(&header); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to decode chunk header: %w", err)
		}

		if header.Done {
			continue
		}

		// read binary data
		data := make([]byte, header.Size)
		if _, err := io.ReadFull(reader, data); err != nil {
			return nil, fmt.Errorf("failed to read chunk data: %w", err)
		}

		result.Outputs = append(result.Outputs, model.ResolutionOutput{
			Resolution: header.Resolution,
			Data:       data,
		})
	}

	return result, nil
}
