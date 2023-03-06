// Example request id middleware for natsrouter

package middleware

import (
	"context"

	"github.com/Karimerto/natsrouter"

	// For generating request ID
	"github.com/rs/xid"
)

type requestIdContextKey struct{}

// An example request id middleware
func RequestIdMiddleware(tags ...string) func(next natsrouter.NatsCtxHandler) natsrouter.NatsCtxHandler {
	return func(next natsrouter.NatsCtxHandler) natsrouter.NatsCtxHandler {
		// If no tags are defined, then assume "request_id"
		// Try a few variants since NATS headers are case-sensitive
		if len(tags) == 0 {
			tags = []string{"request_id", "Request_id", "Request_Id", "REQUEST_ID"}
		}

		return func(msg *natsrouter.NatsMsg) error {
			var requestId string
			// Try all possible tags until something is found
			for _, tag := range tags {
				requestId = msg.Header.Get(tag)
				if requestId != "" {
					break
				}
			}

			// If nothing is found, generate one
			if len(requestId) == 0 {
				requestId = xid.New().String()
			}

			ctx := context.WithValue(msg.Context(), requestIdContextKey{}, requestId)
			return next(msg.WithContext(ctx))
		}
	}
}

// Get current request Id from the context
func RequestIdFromContext(ctx context.Context) string {
	requestId, ok := ctx.Value(requestIdContextKey{}).(string)
	if !ok {
		return ""
	}
	return requestId
}
