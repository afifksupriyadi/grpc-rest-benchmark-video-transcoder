// Package model defines domain structs and DTOs for the worker service.
package model

// VideoData holds the raw video bytes received from the gateway.
type VideoData struct {
	Filename string
	Data     []byte // raw video file bytes received from gateway
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
