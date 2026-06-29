// Package model defines domain structs and DTOs for the client.
package model

// ChunkHeader is read from the response stream before each binary chunk.
// It identifies which resolution the following binary data belongs to.
// This mirrors the same struct used by gateway and worker, since the client
// parses the same wire format when reading the chunked REST response.
type ChunkHeader struct {
	Resolution string `json:"resolution"`
	Size       int64  `json:"size"`
	Done       bool   `json:"done"`
}

// ResolutionOutput holds the transcoded result for a single resolution.
type ResolutionOutput struct {
	Resolution string
	Data       []byte
}

// TranscodeResult holds all resolution outputs received from gateway.
type TranscodeResult struct {
	Outputs []ResolutionOutput
}
