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
	"strings"
	"sync"

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
	options RouterOptions
	quit    chan struct{}
	chanWg  sync.WaitGroup
	closed  chan struct{}
}

// Defines a struct for the router options, which currently contains error
// config, default request id tag (for error reporting) and optional list of
// NATS connection options.
type RouterOptions struct {
	ErrorConfig  *ErrorConfig
	RequestIdTag string
	NatsOptions  nats.Options
}

// Defines a function type that will be used to define options for the router.
type RouterOption func(*RouterOptions)

// Define error config in the router options.
func WithErrorConfig(ec *ErrorConfig) RouterOption {
	return func(o *RouterOptions) {
		o.ErrorConfig = ec
	}
}

// Define error config as strings in the router options.
func WithErrorConfigString(tag, format string) RouterOption {
	return func(o *RouterOptions) {
		o.ErrorConfig = &ErrorConfig{
			Tag:    tag,
			Format: format,
		}
	}
}

// Define new request id header tag
func WithRequestIdTag(tag string) RouterOption {
	return func(o *RouterOptions) {
		o.RequestIdTag = tag
	}
}

// Append one or more nats.Option to the connection, before connecting
func WithNatsOptions(nopts nats.Options) RouterOption {
	return func(o *RouterOptions) {
		o.NatsOptions = nopts
	}
}

func GetDefaultRouterOptions() RouterOptions {
	return RouterOptions{
		&ErrorConfig{"error", "json"},
		"request_id",
		nats.GetDefaultOptions(),
	}
}

// Create a new NatsRouter with a *nats.Conn and an optional list of
// RouterOptions functions. It sets the default RouterOptions to use a default
// ErrorConfig, and then iterates through each option function, calling it with
// the RouterOptions struct pointer to set any additional options.
//
// Deprecated: Use Connect instead. This does not support properly draining
// publications and subscriptions.
func NewRouter(nc *nats.Conn, options ...RouterOption) *NatsRouter {
	router := &NatsRouter{
		nc: nc,
		options: RouterOptions{
			&ErrorConfig{"error", "json"},
			"request_id",
			nats.GetDefaultOptions(),
		},
		quit:   make(chan struct{}),
		closed: make(chan struct{}),
	}

	for _, opt := range options {
		opt(&router.options)
	}

	return router
}

// Creates a new NatsRouter with a string address and an optional list of
// RouterOptions functions. It connects to the NATS server using the provided
// address, and then calls NewRouter to create a new NatsRouter with the
// resulting *nats.Conn and optional RouterOptions. If there was an error
// connecting to the server, it returns nil and the error.
//
// Deprecated: Use Connect instead. This does not support properly draining
// publications and subscriptions.
func NewRouterWithAddress(addr string, options ...RouterOption) (*NatsRouter, error) {
	nc, err := nats.Connect(addr)
	if err != nil {
		return nil, err
	}

	return NewRouter(nc, options...), nil
}

// Process the url string argument to Connect.
// Return an array of urls, even if only one.
func processUrlString(url string) []string {
	urls := strings.Split(url, ",")
	var j int
	for _, s := range urls {
		u := strings.TrimSpace(s)
		if len(u) > 0 {
			urls[j] = u
			j++
		}
	}
	return urls[:j]
}

func Connect(url string, options ...RouterOption) (*NatsRouter, error) {
	opts := GetDefaultRouterOptions()
	opts.NatsOptions.Servers = processUrlString(url)
	for _, opt := range options {
		if opt != nil {
			opt(&opts)
		}
	}
	return opts.Connect()
}

// Connect will attempt to connect to a NATS server with multiple options.
func (r RouterOptions) Connect() (*NatsRouter, error) {
	// Check options, set defaults if necessary
	if r.ErrorConfig == nil {
		r.ErrorConfig = &ErrorConfig{"error", "json"}
	}
	if r.RequestIdTag == "" {
		r.RequestIdTag = "request_id"
	}

	// Create router instance
	router := &NatsRouter{
		options: r,
		quit:   make(chan struct{}),
		closed: make(chan struct{}),
	}

	// Set custom closed callback
	if r.NatsOptions.ClosedCB != nil {
		// Preserve original CB as well (via a closure)
		r.NatsOptions.ClosedCB = func(orig nats.ConnHandler) nats.ConnHandler {
			original := orig
			return func(nc *nats.Conn) {
				// First notify own channel
				close(router.closed)
				// And then call the original handler
				original(nc)
			}
		}(r.NatsOptions.ClosedCB)
	} else {
		r.NatsOptions.ClosedCB = func(_ *nats.Conn) {
			close(router.closed)
		}
	}

	// Perform actual connection
	nc, err := r.NatsOptions.Connect()
	if err != nil {
		return nil, err
	}
	router.nc = nc
	return router, nil
}

// Drain pubs/subs and close connection to NATS server
func (n *NatsRouter) Drain() {
	// First close any channel-based subscriptions
	close(n.quit)
	n.chanWg.Wait()

	// Then start draining the connection
	n.nc.Drain()

	// Wait until it is done
	<- n.closed
}

// Close connection to NATS server
func (n *NatsRouter) Close() {
	close(n.quit)
	n.chanWg.Wait()
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
		quit:    make(chan struct{}),
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
func (n *NatsRouter) QueueSubscribe(subject, queue string, handler NatsCtxHandler) (*nats.Subscription, error) {
	return n.nc.QueueSubscribe(subject, queue, n.msgHandler(handler))
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
			if len(n.options.RequestIdTag) > 0 {
				if reqId, ok := msg.Header[n.options.RequestIdTag]; ok {
					reply.Header.Add(n.options.RequestIdTag, reqId[0])
				}
			}
			reply.Header.Add(n.options.ErrorConfig.Tag, n.options.ErrorConfig.Format)
			reply.Data = errData

			msg.RespondMsg(reply)
		}
	}
}

// Same as Subscribe, except uses channels. Note that error handling is
// available only for middleware, since the message is processed first by
// middleware and then inserted into the *NatsMsg channel.
func (n *NatsRouter) ChanSubscribe(subject string, ch chan *NatsMsg) (*nats.Subscription, error) {
	intCh := make(chan *nats.Msg, 64)
	n.chanWg.Add(1)
	go n.chanMsgHandler(ch, intCh)
	return n.nc.ChanSubscribe(subject, intCh)
}

// Same as QueueSubscribe, except uses channels. Note that error handling is
// available only for middleware, since the message is processed first by
// middleware and then inserted into the *NatsMsg channel.
func (n *NatsRouter) ChanQueueSubscribe(subject, queue string, ch chan *NatsMsg) (*nats.Subscription, error) {
	intCh := make(chan *nats.Msg, 64)
	n.chanWg.Add(1)
	go n.chanMsgHandler(ch, intCh)
	return n.nc.ChanQueueSubscribe(subject, queue, intCh)
}

// Handler that wraps function call with any registered middleware functions in
// reverse order. On any error, an error message is automatically sent as a
// response to the request.
func (n *NatsRouter) chanMsgHandler(ch chan *NatsMsg, intCh chan *nats.Msg) {
	defer n.chanWg.Done()

	handler := func(natsMsg *NatsMsg) error {
		ch <- natsMsg
		return nil
	}

chanLoop:
	for {
		select {
		case msg := <-intCh:
			natsMsg := &NatsMsg{
				msg,
				context.Background(),
			}

			var wrappedHandler NatsCtxHandler = handler
			for i := len(n.mw) - 1; i >= 0; i-- {
				wrappedHandler = n.mw[i](wrappedHandler)
			}
			// Errors are only handled for the middleware
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
				if len(n.options.RequestIdTag) > 0 {
					if reqId, ok := msg.Header[n.options.RequestIdTag]; ok {
						reply.Header.Add(n.options.RequestIdTag, reqId[0])
					}
				}
				reply.Header.Add(n.options.ErrorConfig.Tag, n.options.ErrorConfig.Format)
				reply.Data = errData

				msg.RespondMsg(reply)
			}
		case <-n.quit:
			break chanLoop
		}
	}
}

// Publish is a passthrough function for the `nats` Publish function
func (n *NatsRouter) Publish(subject string, data []byte) error {
	return n.nc.Publish(subject, data)
}

// Returns a new `Queue` object with the given queue name
func (n *NatsRouter) Queue(queue string) *Queue {
	return &Queue{
		n:     n,
		queue: queue,
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
