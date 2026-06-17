package model

// ChunkHeader is written to the response stream before each binary chunk.
// It tells the client which resolution the following binary data belongs to.
type ChunkHeader struct {
	Resolution string `json:"resolution"` // e.g. "720p", "480p", "360p"
	Size       int64  `json:"size"`       // size of the following binary data in bytes
	Done       bool   `json:"done"`       // true if this is the last chunk for this resolution
}
