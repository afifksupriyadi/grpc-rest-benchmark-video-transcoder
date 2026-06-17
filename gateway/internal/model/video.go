// Package model defines domain structs and DTOs for the gateway service.
package model

// VideoPayload holds the raw video data received from the client.
type VideoPayload struct {
	Filename string
	Data     []byte // raw video file bytes received from client upload
}

// ResolutionOutput holds the transcoded result for a single resolution.
type ResolutionOutput struct {
	Resolution string
	Data       []byte // raw transcoded video bytes for this resolution
}

// TranscodeResult holds all resolution outputs from a transcoding job.
type TranscodeResult struct {
	Outputs []ResolutionOutput
}
