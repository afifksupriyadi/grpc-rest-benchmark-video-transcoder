package constant

// Communication segment identifiers used for metrics labeling.
const (
	SegmentGatewayToWorker = "gateway_to_worker" // t3 → t4: gateway forward to worker
	SegmentWorkerToGateway = "worker_to_gateway" // t5 → t6: worker result to gateway
)
