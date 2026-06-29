package model

// ChunkHeader is written to the response stream before each binary chunk.
// It tells the client which resolution the following binary data belongs to.
type ChunkHeader struct {
	Resolution string `json:"resolution"` // e.g. "720p", "480p", "360p"
	Size       int64  `json:"size"`       // size of the following binary data in bytes
	Done       bool   `json:"done"`       // true if this is the last chunk for this resolution
}

// ReportMetricRequest represents a client-reported metric for a segment that
// gateway cannot measure on its own — currently only used for SegmentGatewayToClient
// (t8-t7), since gateway never knows the client's true t8.
type ReportMetricRequest struct {
	Segment         string  `json:"segment"`
	Protocol        string  `json:"protocol"`
	DurationSeconds float64 `json:"durationSeconds"`
	Bytes           int64   `json:"bytes"`
}
