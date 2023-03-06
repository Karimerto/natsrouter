package middleware

import (
	"bytes"
	"testing"
	"time"

	"github.com/Karimerto/natsrouter"

	"github.com/nats-io/nats.go"
)

func TestRequestIdMiddleware(t *testing.T) {
	// Create test server
	s := getServer(t)
	defer s.Shutdown()

	t.Run("default request_id header", func(t *testing.T) {
		// Create router and connect to test server
		nr, err := natsrouter.NewRouterWithAddress(s.Addr().String())
		nr = nr.Use(RequestIdMiddleware())
		if err != nil {
			t.Fatalf("Could not connect to NATS server: %v", err)
		}

		nc := nr.Conn()
		defer nr.Close()

		reqId := "req-1"

		sub := nr.Subject("foo")
		// _, err := sub.Subscribe(emptyHandler)
		_, err = sub.Subscribe(func(msg *natsrouter.NatsMsg) error {
			if RequestIdFromContext(msg.Context()) != reqId {
				t.Errorf("request id does not match/not found")
			}

			// Send response with same content
			if err := msg.RespondWithOriginalHeaders(msg.Data); err != nil {
				t.Errorf("Failed to publish reply: %v", err)
			}

			return nil
		})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Create message and send a request
		msg := nats.NewMsg("foo")
		msg.Data = []byte("data")
		msg.Header.Add("request_id", reqId)

		reply, err := nc.RequestMsg(msg, 1*time.Second)
		// Verify contents
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !bytes.Equal(msg.Data, reply.Data) {
			t.Errorf("responses do not match, expected %s, received %s", string(msg.Data), string(reply.Data))
		}
		if msg.Header.Get("request_id") != reply.Header.Get("request_id") {
			t.Errorf("request_id does not match")
		}
	})

	t.Run("custom request_id header", func(t *testing.T) {
		headerTag := "reqid"

		// Create router and connect to test server
		nr, err := natsrouter.NewRouterWithAddress(s.Addr().String())
		nr = nr.Use(RequestIdMiddleware(headerTag))
		if err != nil {
			t.Fatalf("Could not connect to NATS server: %v", err)
		}

		nc := nr.Conn()
		defer nr.Close()

		reqId := "req-1"

		sub := nr.Subject("foo")
		// _, err := sub.Subscribe(emptyHandler)
		_, err = sub.Subscribe(func(msg *natsrouter.NatsMsg) error {
			if RequestIdFromContext(msg.Context()) != reqId {
				t.Errorf("request id does not match/not found")
			}

			// Send response with same content
			if err := msg.RespondWithOriginalHeaders(msg.Data); err != nil {
				t.Errorf("Failed to publish reply: %v", err)
			}

			return nil
		})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Create message and send a request
		msg := nats.NewMsg("foo")
		msg.Data = []byte("data")
		msg.Header.Add(headerTag, reqId)

		reply, err := nc.RequestMsg(msg, 1*time.Second)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !bytes.Equal(msg.Data, reply.Data) {
			t.Errorf("responses do not match, expected %s, received %s", string(msg.Data), string(reply.Data))
		}
		if msg.Header.Get(headerTag) != reply.Header.Get(headerTag) {
			t.Errorf("request_id does not match")
		}
	})

	t.Run("missing request_id header", func(t *testing.T) {
		// Create router and connect to test server
		nr, err := natsrouter.NewRouterWithAddress(s.Addr().String())
		nr = nr.Use(RequestIdMiddleware())
		if err != nil {
			t.Fatalf("Could not connect to NATS server: %v", err)
		}

		nc := nr.Conn()
		defer nr.Close()

		sub := nr.Subject("foo")
		// _, err := sub.Subscribe(emptyHandler)
		_, err = sub.Subscribe(func(msg *natsrouter.NatsMsg) error {
			reqId := RequestIdFromContext(msg.Context())
			if len(reqId) == 0 {
				t.Errorf("no request id found")
			}

			// Send response with same content
			headers := make(map[string]string)
			headers["request_id"] = reqId
			if err := msg.RespondWithHeaders(msg.Data, headers); err != nil {
				t.Errorf("Failed to publish reply: %v", err)
			}

			return nil
		})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Create message without request_id and send a request
		msg := nats.NewMsg("foo")
		msg.Data = []byte("data")

		reply, err := nc.RequestMsg(msg, 1*time.Second)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !bytes.Equal(msg.Data, reply.Data) {
			t.Errorf("responses do not match, expected %s, received %s", string(msg.Data), string(reply.Data))
		}
		if len(reply.Header.Get("request_id")) == 0 {
			t.Errorf("request_id not found")
		}
	})
}
