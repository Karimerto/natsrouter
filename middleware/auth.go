// Example authentication middleware for natsrouter

package middleware

import (
	"context"
	"errors"

	"github.com/Karimerto/natsrouter"
)

var ErrTokenMissing = errors.New("authorization token not found")
var ErrNotAuthorized = errors.New("not authorized")

type tokenContextKey struct{}

// Callback that must return true if authorized, false otherwise
type AuthCallback func(token string) bool

type AuthMiddleware struct {
	cb   AuthCallback
	tags []string
}

// Create a new authentication middleware, with optional header tag(s)
// If header tag is not defined, it defaults to `authorization`
func NewAuthMiddleware(cb AuthCallback, tags ...string) *AuthMiddleware {
	return &AuthMiddleware{
		cb:   cb,
		tags: tags,
	}
}

// Authentication middleware function
func (a *AuthMiddleware) Auth(next natsrouter.NatsCtxHandler) natsrouter.NatsCtxHandler {
	return func(msg *natsrouter.NatsMsg) error {
		var token string
		var tags []string
		// If no tags are defined, then assume "authorization"
		// Try a few variants since NATS headers are case-sensitive
		if len(a.tags) == 0 {
			tags = []string{"authorization", "Authorization", "AUTHORIZATION"}
		} else {
			tags = a.tags
		}

		// Try all possible tags until something is found
		for _, tag := range tags {
			token = msg.Header.Get(tag)
			if token != "" {
				break
			}
		}

		// If no token is found, return error
		if len(token) == 0 {
			return ErrTokenMissing
		}

		// If callback returns false, token is not authorized
		if !a.cb(token) {
			return ErrNotAuthorized
		}

		// Store token in context, in case other middleware wants to use it
		ctx := context.WithValue(msg.Context(), tokenContextKey{}, token)
		return next(msg.WithContext(ctx))
	}
}

// Get auth token from the context
func TokenFromContext(ctx context.Context) string {
	return ctx.Value(tokenContextKey{}).(string)
}
