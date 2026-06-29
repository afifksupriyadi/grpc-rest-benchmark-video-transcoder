package constant

// SegmentGatewayToClient identifies Segment 4 when reporting back to gateway.
// Defined locally because client cannot import gateway's internal constant
// package across module boundaries.
const SegmentGatewayToClient = "gateway_to_client"
