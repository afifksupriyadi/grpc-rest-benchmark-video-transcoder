// Package constant defines fixed values used across the worker service.
package constant

// Communication segment identifiers used for metrics labeling.
const (
	SegmentGatewayToWorker = "gateway_to_worker" // t4 - t3: gateway forward to worker
	SegmentWorkerToGateway = "worker_to_gateway" // t6 - t5: worker result to gateway
)
