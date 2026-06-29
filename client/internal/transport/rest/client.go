// Package rest provides a REST implementation of the GatewayClient interface.
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

	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/client/internal/model"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/client/internal/transport"
	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/shared/timing"
)

// RestGatewayClient implements transport.GatewayClient using HTTP REST.
type RestGatewayClient struct {
	httpClient *http.Client
	gatewayURL string
}

// NewRestGatewayClient creates a new RestGatewayClient for the given gateway address.
func NewRestGatewayClient(gatewayAddr string, timeout time.Duration) transport.GatewayClient {
	return &RestGatewayClient{
		httpClient: &http.Client{Timeout: timeout},
		gatewayURL: fmt.Sprintf("http://%s", gatewayAddr),
	}
}

// Transcode sends the video to gateway via HTTP POST, carrying t1 in the request header.
// It reads t7 back from the response header before parsing the chunked body.
func (c *RestGatewayClient) Transcode(ctx context.Context, filename string, data []byte, t1 time.Time) (*model.TranscodeResult, time.Time, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.gatewayURL+"/v1/transcode", bytes.NewReader(data))
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("failed to build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Video-Filename", filename)
	req.Header.Set(timing.HeaderTimestamp, timing.EncodeTimestamp(t1))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("gateway request failed: %w", err)
	}
	defer resp.Body.Close()

	t7Str := resp.Header.Get(timing.HeaderTimestamp)
	t7, err := timing.DecodeTimestamp(t7Str)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("missing or invalid timestamp header in response: %w", err)
	}

	result, err := parseChunkedResponse(resp.Body)
	if err != nil {
		return nil, time.Time{}, err
	}

	return result, t7, nil
}

// parseChunkedResponse reads the chunked streaming response from gateway.
// It alternates between reading a ChunkHeader JSON line and the binary data that follows.
func parseChunkedResponse(body io.Reader) (*model.TranscodeResult, error) {
	reader := bufio.NewReader(body)
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
