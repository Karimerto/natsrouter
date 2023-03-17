package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Karimerto/natsrouter"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func runServer(opts *server.Options) (*server.Server, error) {
	s, err := server.NewServer(opts)
	if err != nil || s == nil {
		return nil, err
	}

	// Run server in Go routine.
	go s.Start()

	// Wait for accept loop(s) to be started
	if !s.ReadyForConnections(10 * time.Second) {
		return nil, errors.New("Unable to start NATS Server in Go Routine")
	}

	return s, nil
}

func getServer(t *testing.T) *server.Server {
	// Create test server
	opts := &server.Options{Host: "localhost", Port: server.RANDOM_PORT, NoSigs: true}
	s, err := runServer(opts)
	if err != nil {
		t.Fatalf("Could not start NATS server: %v", err)
	}

	return s
}

func TestAuthMiddleware(t *testing.T) {
	// Create test server
	s := getServer(t)
	defer s.Shutdown()

	t.Run("accept login", func(t *testing.T) {
		// Create router and connect to test server
		nr, err := natsrouter.Connect(s.Addr().String())
		am := NewAuthMiddleware(func(token string) bool { return true })
		nr = nr.Use(am.Auth)
		if err != nil {
			t.Fatalf("Could not connect to NATS server: %v", err)
		}

		nc := nr.Conn()
		defer nr.Close()

		authToken := "token-1"

		sub := nr.Subject("foo")
		// _, err := sub.Subscribe(emptyHandler)
		_, err = sub.Subscribe(func(msg *natsrouter.NatsMsg) error {
			if TokenFromContext(msg.Context()) != authToken {
				t.Errorf("auth token does not match")
			}

			// Send response with same content
			if err := msg.Respond(msg.Data); err != nil {
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
		msg.Header.Add("authorization", authToken)

		reply, err := nc.RequestMsg(msg, 1*time.Second)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !bytes.Equal(msg.Data, reply.Data) {
			t.Errorf("responses do not match, expected %s, received %s", string(msg.Data), string(reply.Data))
		}
	})

	t.Run("reject login", func(t *testing.T) {
		// Create router and connect to test server
		nr, err := natsrouter.Connect(s.Addr().String())
		am := NewAuthMiddleware(func(token string) bool { return false })
		nr = nr.Use(am.Auth)
		if err != nil {
			t.Fatalf("Could not connect to NATS server: %v", err)
		}

		nc := nr.Conn()
		defer nr.Close()

		authToken := "token-1"

		sub := nr.Subject("foo")
		// _, err := sub.Subscribe(emptyHandler)
		_, err = sub.Subscribe(func(msg *natsrouter.NatsMsg) error {
			// Nothing to do, should not be reached
			t.Errorf("handler should not be reached")
			return nil
		})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Create message and send a request
		msg := nats.NewMsg("foo")
		msg.Data = []byte("data")
		msg.Header.Add("authorization", authToken)

		reply, err := nc.RequestMsg(msg, 1*time.Second)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		// Error header should have been returned
		if reply.Header.Get("error") != "json" {
			t.Errorf("error header was not found")
		}
		handlerErr := natsrouter.HandlerError{}
		err = json.Unmarshal(reply.Data, &handlerErr)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if handlerErr.Message != ErrNotAuthorized.Error() {
			t.Errorf("authorization error does not match: %s", handlerErr.Message)
		}
	})

	t.Run("reject login with custom error header", func(t *testing.T) {
		// Create router and connect to test server
		tag := "err"
		format := "proto"

		nr, err := natsrouter.Connect(s.Addr().String(), natsrouter.WithErrorConfigString(tag, format))
		am := NewAuthMiddleware(func(token string) bool { return false })
		nr = nr.Use(am.Auth)
		if err != nil {
			t.Fatalf("Could not connect to NATS server: %v", err)
		}

		nc := nr.Conn()
		defer nr.Close()

		authToken := "token-1"

		sub := nr.Subject("foo")
		// _, err := sub.Subscribe(emptyHandler)
		_, err = sub.Subscribe(func(msg *natsrouter.NatsMsg) error {
			// Nothing to do, should not be reached
			t.Errorf("handler should not be reached")
			return nil
		})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Create message and send a request
		msg := nats.NewMsg("foo")
		msg.Data = []byte("data")
		msg.Header.Add("authorization", authToken)

		reply, err := nc.RequestMsg(msg, 1*time.Second)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		// Error header should have been returned
		if reply.Header.Get(tag) != format {
			t.Errorf("error header was not found")
		}
	})

	t.Run("missing login", func(t *testing.T) {
		// Create router and connect to test server
		nr, err := natsrouter.Connect(s.Addr().String())
		am := NewAuthMiddleware(func(token string) bool { return false })
		nr = nr.Use(am.Auth)
		if err != nil {
			t.Fatalf("Could not connect to NATS server: %v", err)
		}

		nc := nr.Conn()
		defer nr.Close()

		sub := nr.Subject("foo")
		// _, err := sub.Subscribe(emptyHandler)
		_, err = sub.Subscribe(func(msg *natsrouter.NatsMsg) error {
			// Nothing to do, should not be reached
			t.Errorf("handler should not be reached")
			return nil
		})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Create message without authorization header and send a request
		msg := nats.NewMsg("foo")
		msg.Data = []byte("data")

		reply, err := nc.RequestMsg(msg, 1*time.Second)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		// Error header should have been returned
		if reply.Header.Get("error") != "json" {
			t.Errorf("error header was not found")
		}
		handlerErr := natsrouter.HandlerError{}
		err = json.Unmarshal(reply.Data, &handlerErr)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if handlerErr.Message != ErrTokenMissing.Error() {
			t.Errorf("authorization error does not match: %s", handlerErr.Message)
		}
	})
}
