// Package transport contains client implementations for calling the gateway service.
package transport

import (
	"context"
	"time"

	"github.com/afifksupriyadi/grpc-rest-benchmark-video-transcoder/client/internal/model"
)

// GatewayClient defines the contract for sending a video to gateway and receiving
// the transcoded result back. t1 is passed in so it can be propagated to gateway.
// t7 is returned, read from gateway's response, so the client can compute t8-t7
// after it finishes receiving and saving the result.
type GatewayClient interface {
	Transcode(ctx context.Context, filename string, data []byte, t1 time.Time) (result *model.TranscodeResult, t7 time.Time, err error)
}
