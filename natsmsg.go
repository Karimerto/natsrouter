package natsrouter

import (
	"context"

	"github.com/nats-io/nats.go"
)

// Extend nats.Msg struct to include context
type NatsMsg struct {
	*nats.Msg
	ctx context.Context
}

// Get current attached message context
func (n *NatsMsg) Context() context.Context {
	return n.ctx
}

// Set new message context
func (n *NatsMsg) WithContext(ctx context.Context) *NatsMsg {
	return &NatsMsg{
		n.Msg,
		ctx,
	}
}

// Send a response back to requester
func (n *NatsMsg) Respond(data []byte) error {
	reply := nats.NewMsg(n.Reply)
	reply.Data = data
	return n.RespondMsg(reply)
}

// Send a response with given headers
func (n *NatsMsg) RespondWithHeaders(data []byte, headers map[string]string) error {
	reply := nats.NewMsg(n.Reply)
	reply.Data = data
	for k, v := range headers {
		reply.Header.Add(k, v)
	}

	return n.RespondMsg(reply)
}

// Send a response and copy given original headers, or all if nothing defined
func (n *NatsMsg) RespondWithOriginalHeaders(data []byte, headers ...string) error {
	reply := nats.NewMsg(n.Reply)
	reply.Data = data
	if len(headers) > 0 {
		for _, header := range headers {
			reply.Header[header] = n.Header[header]
		}
	} else {
		reply.Header = n.Header
	}

	return n.RespondMsg(reply)
}
