// Package label provides constants for propagating scenario metadata across processes.
package label

// HTTP header / gRPC metadata keys used to carry scenario metadata, separate
// from X-Timestamp since these carry different kinds of information.
const (
	HeaderScenario         = "x-scenario"
	HeaderPayloadSize      = "x-payload-size"
	HeaderConcurrencyLevel = "x-concurrency-level"
)

// Labels bundles the three scenario-related label values together, so they
// can be passed as one unit through the call chain instead of three separate
// parameters.
// - Scenario identifies which research scenario this request belongs to (e.g. "scenario_a", "scenario_b")
// - PayloadSize is the rounded input file size label (e.g. "10mb", "500mb"), only relevant for Scenario A
// - ConcurrencyLevel is the concurrent request count label (e.g. "20"), only relevant for Scenario B
type Labels struct {
	Scenario         string
	PayloadSize      string
	ConcurrencyLevel string
}
