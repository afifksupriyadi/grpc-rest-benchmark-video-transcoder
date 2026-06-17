package constant

// Communication segment identifiers used for metrics labeling.
const (
	SegmentClientToGateway = "client_to_gateway" // t1 → t2: client upload to gateway
	SegmentGatewayToWorker = "gateway_to_worker" // t3 → t4: gateway forward to worker
	SegmentWorkerToGateway = "worker_to_gateway" // t5 → t6: worker result to gateway
	SegmentGatewayToClient = "gateway_to_client" // t7 → t8: gateway result to client
)
