package natsrouter

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"strings"

	"github.com/nats-io/nats.go"
)

const compressMinimum = 1000

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

func (n *NatsMsg) respondCompressed(reply *nats.Msg, data []byte) error {
	// Check size, and if too small, do not compress
	if len(data) < compressMinimum {
		reply.Data = data
		return n.RespondMsg(reply)
	}

	// First check if request supports compression
	enc := n.Header.Get("accept-encoding")
	var buf bytes.Buffer
	if strings.Contains(enc, "gzip") {
		// Compress with gzip
		zw := gzip.NewWriter(&buf)
		_, err := zw.Write(data)
		if err != nil {
			return err
		}
		zw.Close()
		reply.Header.Set("encoding", "gzip")
	} else if strings.Contains(enc, "deflate") {
		// Compress with deflate
		fw, _ := flate.NewWriter(&buf, flate.DefaultCompression)
		_, err := fw.Write(data)
		if err != nil {
			return err
		}
		fw.Close()
		reply.Header.Set("encoding", "deflate")
	} else {
		// No compression
		buf.Write(data)
	}

	reply.Data = buf.Bytes()

	return n.RespondMsg(reply)
}

// Send a response back to requester
func (n *NatsMsg) Respond(data []byte) error {
	reply := nats.NewMsg(n.Reply)
	return n.respondCompressed(reply, data)
}

// Send a response with given headers
func (n *NatsMsg) RespondWithHeaders(data []byte, headers map[string]string) error {
	reply := nats.NewMsg(n.Reply)
	for k, v := range headers {
		reply.Header.Add(k, v)
	}

	return n.respondCompressed(reply, data)
}

// Send a response and copy given original headers, or all if nothing defined
func (n *NatsMsg) RespondWithOriginalHeaders(data []byte, headers ...string) error {
	reply := nats.NewMsg(n.Reply)
	if len(headers) > 0 {
		for _, header := range headers {
			reply.Header[header] = n.Header[header]
		}
	} else {
		reply.Header = n.Header
	}

	return n.respondCompressed(reply, data)
}
