// Package contextutil provides helpers for storing and retrieving request-scoped values from context.
package contextutil

import "context"

// ctxKey is a private type for context keys to avoid collisions.
type ctxKey string

const (
	CtxKeyProtocol ctxKey = "protocol" // active protocol for the current request
)

// SetProtocol stores the protocol identifier into the context.
func SetProtocol(ctx context.Context, protocol string) context.Context {
	return context.WithValue(ctx, CtxKeyProtocol, protocol)
}

// GetProtocol retrieves the protocol identifier from the context.
func GetProtocol(ctx context.Context) string {
	protocol, _ := ctx.Value(CtxKeyProtocol).(string)
	return protocol
}
