/*
This is a Go package called `natsrouter`, which provides middleware-supported
NATS message routing. It defines `NatsRouter` struct that holds the NATS
connection and an array of middleware functions. New function creates a new
instance of `NatsRouter`, `WithMiddleware` adds middleware functions to the
router, and `Subscribe` and `QueueSubscribe` register handlers for NATS
messages. `Queue` and `Subject` structs provide additional routing capabilities
by allowing to group subscriptions and create more specific subscription
subjects. `getSubject` method of `Subject` is used to create the final
subscription subject by joining the subjects with dots and verifying that ">"
is the last in the chain.
*/

package natsrouter

import (
	"context"
	"encoding/json"

	"github.com/nats-io/nats.go"
)

// Handler function that adds a `context.Context` to a `*nats.Msg`
type NatsCtxHandler func(*NatsMsg) error

// Middleware function that takes a `NatsCtxHandler` and returns a new `NatsCtxHandler`
type NatsMiddlewareFunc func(NatsCtxHandler) NatsCtxHandler

// NatsRouter is a middleware-supported NATS router that provides a fluent API
// for subscribing to subjects and chaining middleware functions.
type NatsRouter struct {
	nc      *nats.Conn
	mw      []NatsMiddlewareFunc
	options *RouterOptions
}

// Defines a struct for the router options, which currently only contains
// error config.
type RouterOptions struct {
	ec           *ErrorConfig
	requestIdTag string
}

// Defines a function type that will be used to define options for the router.
type RouterOption func(options *RouterOptions)

// Define error config in the router options.
func WithErrorConfig(ec *ErrorConfig) RouterOption {
	return func(options *RouterOptions) {
		options.ec = ec
	}
}

// Define error config as strings in the router options.
func WithErrorConfigString(tag, format string) RouterOption {
	return func(options *RouterOptions) {
		options.ec = &ErrorConfig{
			Tag:    tag,
			Format: format,
		}
	}
}

// Define new request id header tag
func WithRequestIdTag(tag string) RouterOption {
	return func(options *RouterOptions) {
		options.requestIdTag = tag
	}
}

// Create a new NatsRouter with a *nats.Conn and an optional list of
// RouterOptions functions. It sets the default RouterOptions to use a default
// ErrorConfig, and then iterates through each option function, calling it with
// the RouterOptions struct pointer to set any additional options.
func NewRouter(nc *nats.Conn, options ...RouterOption) *NatsRouter {
	router := &NatsRouter{
		nc: nc,
		options: &RouterOptions{
			&ErrorConfig{"error", "json"},
			"request_id",
		},
	}

	for _, opt := range options {
		opt(router.options)
	}

	return router
}

// Creates a new NatsRouter with a string address and an optional list of
// RouterOptions functions. It connects to the NATS server using the provided
// address, and then calls NewRouter to create a new NatsRouter with the
// resulting *nats.Conn and optional RouterOptions. If there was an error
// connecting to the server, it returns nil and the error.
func NewRouterWithAddress(addr string, options ...RouterOption) (*NatsRouter, error) {
	nc, err := nats.Connect(addr)
	if err != nil {
		return nil, err
	}

	return NewRouter(nc, options...), nil
}

// Close connection to NATS server
func (n *NatsRouter) Close() {
	n.nc.Close()
}

// Get current underlying NATS connection
func (n *NatsRouter) Conn() *nats.Conn {
	return n.nc
}

// Returns a new `NatsRouter` with additional middleware functions
func (n *NatsRouter) WithMiddleware(fns ...NatsMiddlewareFunc) *NatsRouter {
	return &NatsRouter{
		nc:      n.nc,
		mw:      append(n.mw, fns...),
		options: n.options,
	}
}

// Alias for `WithMiddleware`
func (n *NatsRouter) Use(fns ...NatsMiddlewareFunc) *NatsRouter {
	return n.WithMiddleware(fns...)
}

// Subscribe registers a handler function for the specified subject and returns
// a *nats.Subscription. The handler function is wrapped with any registered
// middleware functions in reverse order.
func (n *NatsRouter) Subscribe(subject string, handler NatsCtxHandler) (*nats.Subscription, error) {
	return n.nc.Subscribe(subject, n.msgHandler(handler))
}

// QueueSubscribe registers a handler function for the specified subject and
// queue group and returns a *nats.Subscription. The handler function is wrapped
// with any registered middleware functions in reverse order.
func (n *NatsRouter) QueueSubscribe(subject, group string, handler NatsCtxHandler) (*nats.Subscription, error) {
	return n.nc.QueueSubscribe(subject, group, n.msgHandler(handler))
}

// Handler that wraps function call with any registered middleware functions in
// reverse order. On any error, an error message is automatically sent as a
// response to the request.
func (n *NatsRouter) msgHandler(handler NatsCtxHandler) func(*nats.Msg) {
	return func(msg *nats.Msg) {
		natsMsg := &NatsMsg{
			msg,
			context.Background(),
		}

		var wrappedHandler NatsCtxHandler = handler
		for i := len(n.mw) - 1; i >= 0; i-- {
			wrappedHandler = n.mw[i](wrappedHandler)
		}
		err := wrappedHandler(natsMsg)

		if err != nil {
			handlerErr, ok := err.(*HandlerError)
			if !ok {
				handlerErr = &HandlerError{
					Message: err.Error(),
					Code:    500,
				}
			}
			errData, _ := json.Marshal(handlerErr)

			reply := nats.NewMsg(msg.Reply)
			if len(n.options.requestIdTag) > 0 {
				if reqId, ok := msg.Header[n.options.requestIdTag]; ok {
					reply.Header.Add(n.options.requestIdTag, reqId[0])
				}
			}
			reply.Header.Add(n.options.ec.Tag, n.options.ec.Format)
			reply.Data = errData

			msg.RespondMsg(reply)
		}
	}
}

// Publish is a passthrough function for the `nats` Publish function
func (n *NatsRouter) Publish(subject string, data []byte) error {
	return n.nc.Publish(subject, data)
}

// Returns a new `Queue` object with the given group name
func (n *NatsRouter) Queue(group string) *Queue {
	return &Queue{
		n:     n,
		group: group,
	}
}

// Returns a new `Subject` object with the given subject name(s)
func (n *NatsRouter) Subject(subjects ...string) *Subject {
	return &Subject{
		n:        n,
		subjects: subjects,
	}
}

// Returns a new `Subject` object with the wildcard ALL_SUBJECT. This is also knows as a "wiretap" mode, listening on all requests.
func (n *NatsRouter) Wiretap() *Subject {
	return &Subject{
		n:        n,
		subjects: []string{ALL_SUBJECT},
	}
}
