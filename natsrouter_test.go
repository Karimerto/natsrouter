package natsrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func emptyHandler(msg *NatsMsg) error {
	return nil
}

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

func TestRunServer(t *testing.T) {
	opts := &server.Options{Host: "localhost", Port: server.RANDOM_PORT, NoSigs: true}
	s, err := runServer(opts)
	if err != nil {
		t.Fatalf("Could not start NATS server: %v", err)
	}
	defer s.Shutdown()

	// nc, err := nats.Connect(test.DefaultURL)
	nc, err := nats.Connect(s.Addr().String())
	if err != nil {
		t.Fatalf("Could not connect to NATS server: %v", err)
	}
	defer nc.Close()
}

func TestConnect(t *testing.T) {
	// Create test server
	opts := &server.Options{Host: "localhost", Port: server.RANDOM_PORT, NoSigs: true}
	s, err := runServer(opts)
	if err != nil {
		t.Fatalf("Could not start NATS server: %v", err)
	}
	defer s.Shutdown()

	// Create router and connect to test server
	nr, err := Connect(s.Addr().String())
	if err != nil {
		t.Fatalf("Could not connect to NATS server: %v", err)
	}
	defer nr.Close()
}

func TestOptionsConnect(t *testing.T){
	// Create test server
	opts := &server.Options{Host: "localhost", Port: server.RANDOM_PORT, NoSigs: true}
	s, err := runServer(opts)
	if err != nil {
		t.Fatalf("Could not start NATS server: %v", err)
	}
	defer s.Shutdown()

	// Create router and connect to test server
	rOpts := GetDefaultRouterOptions()
	rOpts.NatsOptions.Url = s.Addr().String()
	nr, err := rOpts.Connect()
	if err != nil {
		t.Fatalf("Could not connect to NATS server: %v", err)
	}
	defer nr.Close()
}

func TestDrain(t *testing.T){
	// Create test server
	opts := &server.Options{Host: "localhost", Port: server.RANDOM_PORT, NoSigs: true}
	s, err := runServer(opts)
	if err != nil {
		t.Fatalf("Could not start NATS server: %v", err)
	}
	defer s.Shutdown()

	// Create router and connect to test server
	ch := make(chan struct{})
	rOpts := RouterOptions{
		NatsOptions: nats.Options{
			Url: s.Addr().String(),
			ClosedCB: func(_ *nats.Conn) {
				close (ch)
			},
		},
	}
	nr, err := rOpts.Connect()
	if err != nil {
		t.Fatalf("Could not connect to NATS server: %v", err)
	}
	nr.Drain()

	select {
	case <-ch:
	default:
		t.Error("Channel is not closed")
	}
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

func getServerAndRouter(t *testing.T) (*server.Server, *NatsRouter) {
	s := getServer(t)

	// Create router and connect to test server
	nr, err := Connect(s.Addr().String())
	if err != nil {
		t.Fatalf("Could not connect to NATS server: %v", err)
	}
	return s, nr
}

func TestSubjectSubscribe(t *testing.T) {
	// Create test server and router
	s, nr := getServerAndRouter(t)
	defer s.Shutdown()
	defer nr.Close()

	t.Run("subscribe with single subject", func(t *testing.T) {
		sub := nr.Subject("foo")
		_, err := sub.Subscribe(emptyHandler)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("subscribe with any subject", func(t *testing.T) {
		sub := nr.Subject("foo").Any()
		_, err := sub.Subscribe(emptyHandler)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("subscribe with all subject", func(t *testing.T) {
		sub := nr.Subject("foo").All()
		_, err := sub.Subscribe(emptyHandler)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("subscribe with multiple subjects", func(t *testing.T) {
		sub := nr.Subject("a").Subject("b").Subject("c")
		_, err := sub.Subscribe(emptyHandler)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		got, err := sub.getSubject()
		if got != "a.b.c" {
			t.Errorf("subject string does not match, expected a.b.c, got %s", got)
		}
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("subscribe with invalid subject", func(t *testing.T) {
		sub := nr.Subject("foo").All().Subject("bar")
		_, err := sub.Subscribe(emptyHandler)
		if !errors.Is(err, ErrNonLastAllSubject) {
			t.Errorf("expected error '%v', but got '%v'", ErrNonLastAllSubject, err)
		}
	})
}

func TestQueueSubscribe(t *testing.T) {
	// Create test server and router
	s, nr := getServerAndRouter(t)
	defer s.Shutdown()
	defer nr.Close()

	sub := nr.Queue("group").Subject("foo")

	t.Run("subscribe", func(t *testing.T) {
		_, err := sub.Subscribe(emptyHandler)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("unsubscribe", func(t *testing.T) {
		// Subscribe and then unsubscribe to test that the subscription is successfully removed.
		subscription, err := sub.Subscribe(emptyHandler)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		err = subscription.Unsubscribe()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestNatsRouterPublish(t *testing.T) {
	// Create test server and router
	s, nr := getServerAndRouter(t)
	defer s.Shutdown()
	defer nr.Close()

	t.Run("publish to subject", func(t *testing.T) {
		subject := "foo.bar"
		err := nr.Publish(subject, []byte("hello, world!"))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestMiddlewareChain(t *testing.T) {
	// define some middleware functions
	middleware1 := func(next NatsCtxHandler) NatsCtxHandler {
		return func(msg *NatsMsg) error {
			ctx := context.WithValue(msg.Context(), "key1", "value1")
			return next(msg.WithContext(ctx))
		}
	}

	middleware2 := func(next NatsCtxHandler) NatsCtxHandler {
		return func(msg *NatsMsg) error {
			ctx := context.WithValue(msg.Context(), "key2", "value2")
			return next(msg.WithContext(ctx))
		}
	}

	// define a final handler function
	handler := func(msg *NatsMsg) error {
		if msg.Context().Value("key1") != "value1" {
			t.Errorf("Expected key1 to be value1")
		}

		if msg.Context().Value("key2") != "value2" {
			t.Errorf("Expected key2 to be value2")
		}

		// Send response with same content
		if err := msg.Respond(msg.Data); err != nil {
			t.Errorf("Failed to publish reply: %v", err)
		}

		return nil
	}

	// Create test server and router
	s, nr := getServerAndRouter(t)
	defer s.Shutdown()

	// Add middleware
	nr = nr.Use(middleware1, middleware2)
	defer nr.Close()

	// Get underlying connection
	nc := nr.Conn()

	t.Run("subscribe with middleware to single subject", func(t *testing.T) {
		sub := nr.Subject("foo1")
		// _, err := sub.Subscribe(emptyHandler)
		_, err := sub.Subscribe(handler)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Create message and send a request
		msg := nats.NewMsg("foo1")
		msg.Data = []byte("data")

		reply, err := nc.RequestMsg(msg, 1*time.Second)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !bytes.Equal(msg.Data, reply.Data) {
			t.Errorf("responses do not match, expected %s, received %s", string(msg.Data), string(reply.Data))
		}
	})

	t.Run("subscribe with middleware and queue group to single subject", func(t *testing.T) {
		sub := nr.Queue("group").Subject("foo2")
		// _, err := sub.Subscribe(emptyHandler)
		_, err := sub.Subscribe(handler)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Create message and send a request
		msg := nats.NewMsg("foo2")
		msg.Data = []byte("data")

		reply, err := nc.RequestMsg(msg, 1*time.Second)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !bytes.Equal(msg.Data, reply.Data) {
			t.Errorf("responses do not match, expected %s, received %s", string(msg.Data), string(reply.Data))
		}
	})
}

func TestEncodedMiddleware(t *testing.T) {
	// define some middleware functions
	myMiddleware := func(next NatsCtxHandler) NatsCtxHandler {
		return func(msg *NatsMsg) error {
			ctx := context.WithValue(msg.Context(), "key1", "value1")
			return next(msg.WithContext(ctx))
		}
	}

	// Using any type of function without Msg requires "Reply" address to be saved
	replyMiddleware := func(next NatsCtxHandler) NatsCtxHandler {
		return func(msg *NatsMsg) error {
			ctx := context.WithValue(msg.Context(), "reply", msg.Reply)
			return next(msg.WithContext(ctx))
		}
	}

	type Person struct {
		Name string `json:"name,omitempty"`
		Age  uint   `json:"age,omitempty"`
	}

	// Create test server and router
	s, nr := getServerAndRouter(t)
	defer s.Shutdown()

	// Add first middleware
	nr = nr.Use(myMiddleware)
	defer nr.Close()

	// Get underlying connection
	nc := nr.Conn()

	t.Run("subscribe with encoded middleware (*NatsMsg)", func(t *testing.T) {
		// Basic function with one parameter
		handler := func(msg *NatsMsg) {
			if msg.Context().Value("key1") != "value1" {
				t.Errorf("Expected key1 to be value1")
			}
			if err := msg.Respond(msg.Data); err != nil {
				t.Errorf("Failed to publish reply: %v", err)
			}
		}

		// Setup middleware, note that the encoding type does not matter
		em, err := NewEncodedMiddleware(handler, "default")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Add middleware
		nr2 := nr.Use(em.EncodedMiddleware)

		sub := nr2.Subject("foo1")
		_, err = sub.Subscribe(emptyHandler) // emptyHandler is never called
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Create message and send a request
		msg := nats.NewMsg("foo1")
		msg.Data = []byte("data")

		reply, err := nc.RequestMsg(msg, 1*time.Second)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !bytes.Equal(msg.Data, reply.Data) {
			t.Errorf("responses do not match, expected %s, received %s", string(msg.Data), string(reply.Data))
		}
	})

	t.Run("subscribe with encoded middleware (context.Context, *nats.Msg)", func(t *testing.T) {
		// Basic function with context and nats.Msg parameter
		handler := func(ctx context.Context, msg *nats.Msg) {
			if ctx.Value("key1") != "value1" {
				t.Errorf("Expected key1 to be value1")
			}
			if err := msg.Respond(msg.Data); err != nil {
				t.Errorf("Failed to publish reply: %v", err)
			}
		}

		// Setup middleware, note that the encoding type does not matter
		em, err := NewEncodedMiddleware(handler, "default")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Add middleware
		nr2 := nr.Use(em.EncodedMiddleware)

		sub := nr2.Subject("foo2")
		_, err = sub.Subscribe(emptyHandler)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Create message and send a request
		msg := nats.NewMsg("foo2")
		msg.Data = []byte("data")

		reply, err := nc.RequestMsg(msg, 1*time.Second)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !bytes.Equal(msg.Data, reply.Data) {
			t.Errorf("responses do not match, expected %s, received %s", string(msg.Data), string(reply.Data))
		}
	})

	t.Run("subscribe with encoded middleware (context.Context, *Person)", func(t *testing.T) {
		// Basic function with context and json-encoded parameter
		handler := func(ctx context.Context, p *Person) {
			if ctx.Value("key1") != "value1" {
				t.Errorf("Expected key1 to be value1")
			}
			if p.Name != "someone" {
				t.Errorf("Expected name to be someone")
			}
			reply := ctx.Value("reply").(string)
			if err := nc.Publish(reply, []byte(p.Name)); err != nil {
				t.Errorf("Failed to publish reply: %v", err)
			}
		}

		// Setup middleware, note that the encoding type does not matter
		em, err := NewEncodedMiddleware(handler, "json")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Add middleware
		nr2 := nr.Use(replyMiddleware, em.EncodedMiddleware)

		sub := nr2.Subject("foo3")
		_, err = sub.Subscribe(emptyHandler)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Create message and send a request
		msg := nats.NewMsg("foo3")
		p := Person{Name: "someone", Age: 25}
		msg.Data, err = json.Marshal(p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		reply, err := nc.RequestMsg(msg, 1*time.Second)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !bytes.Equal([]byte("someone"), reply.Data) {
			t.Errorf("responses do not match, expected someone, received %s", string(reply.Data))
		}
	})

	t.Run("subscribe with encoded middleware (context.Context, subject, *Person)", func(t *testing.T) {
		// Basic function with context, subject and json-encoded parameter
		handler := func(ctx context.Context, subject string, p *Person) {
			if ctx.Value("key1") != "value1" {
				t.Errorf("Expected key1 to be value1")
			}
			if p.Name != "someone" {
				t.Errorf("Expected name to be someone")
			}
			if subject != "foo4" {
				t.Errorf("Expected subject to be foo")
			}
			reply := ctx.Value("reply").(string)
			if err := nc.Publish(reply, []byte(p.Name)); err != nil {
				t.Errorf("Failed to publish reply: %v", err)
			}
		}

		// Setup middleware, note that the encoding type does not matter
		em, err := NewEncodedMiddleware(handler, "json")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Add middleware
		nr2 := nr.Use(replyMiddleware, em.EncodedMiddleware)

		sub := nr2.Subject("foo4")
		_, err = sub.Subscribe(emptyHandler)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Create message and send a request
		msg := nats.NewMsg("foo4")
		p := Person{Name: "someone", Age: 25}
		msg.Data, err = json.Marshal(p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		reply, err := nc.RequestMsg(msg, 1*time.Second)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !bytes.Equal([]byte("someone"), reply.Data) {
			t.Errorf("responses do not match, expected someone, received %s", string(reply.Data))
		}
	})

	t.Run("subscribe with encoded middleware (context.Context, *Person)", func(t *testing.T) {
		// Basic function with context, subject. reply and json-encoded parameter
		handler := func(ctx context.Context, subject, reply string, p *Person) {
			if ctx.Value("key1") != "value1" {
				t.Errorf("Expected key1 to be value1")
			}
			if p.Name != "someone" {
				t.Errorf("Expected name to be someone")
			}
			if subject != "foo5" {
				t.Errorf("Expected subject to be foo")
			}
			if err := nc.Publish(reply, []byte(p.Name)); err != nil {
				t.Errorf("Failed to publish reply: %v", err)
			}
		}
		em, err := NewEncodedMiddleware(handler, "json")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Add middleware, note that the replyMiddleware is not needed here
		nr2 := nr.Use(em.EncodedMiddleware)

		sub := nr2.Subject("foo5")
		_, err = sub.Subscribe(emptyHandler)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Create message and send a request
		msg := nats.NewMsg("foo5")
		p := Person{Name: "someone", Age: 25}
		msg.Data, err = json.Marshal(p)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		reply, err := nc.RequestMsg(msg, 1*time.Second)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !bytes.Equal([]byte("someone"), reply.Data) {
			t.Errorf("responses do not match, expected someone, received %s", string(reply.Data))
		}
	})
}

func TestError(t *testing.T) {
	// define a handler that always fails
	errHandler := func(msg *NatsMsg) error {
		return errors.New("request failed")
	}

	t.Run("return default error", func(t *testing.T) {
		// Create test server and router
		s, nr := getServerAndRouter(t)
		defer s.Shutdown()

		nc := nr.Conn()
		defer nr.Close()

		sub := nr.Subject("foo")
		// _, err := sub.Subscribe(emptyHandler)
		_, err := sub.Subscribe(errHandler)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Create message and send a request
		msg := nats.NewMsg("foo")
		msg.Data = []byte("data")

		reply, err := nc.RequestMsg(msg, 1*time.Second)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		got := reply.Header.Get("error")
		if got != "json" {
			t.Errorf("error header does not match, expected %s, got %s", "json", got)
		}
		errJson := []byte("{\"message\":\"request failed\",\"code\":500}")
		if !bytes.Equal(errJson, reply.Data) {
			t.Errorf("responses do not match, expected %s, received %s", string(msg.Data), string(reply.Data))
		}
	})

	t.Run("return custom error", func(t *testing.T) {
		// Create test server and router
		s, nr := getServerAndRouter(t)
		defer s.Shutdown()

		// Create router and connect to test server
		tag := "err"
		format := "proto"
		nr, err := Connect(s.Addr().String(), WithErrorConfigString(tag, format))
		if err != nil {
			t.Fatalf("Could not connect to NATS server: %v", err)
		}

		nc := nr.Conn()
		defer nr.Close()

		sub := nr.Subject("foo")
		// _, err := sub.Subscribe(emptyHandler)
		_, err = sub.Subscribe(errHandler)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Create message and send a request
		msg := nats.NewMsg("foo")
		msg.Data = []byte("data")

		reply, err := nc.RequestMsg(msg, 1*time.Second)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		got, ok := reply.Header[tag]
		if !ok {
			t.Errorf("error header not received")
		} else if got[0] != format {
			t.Errorf("error header does not match, expected %s, received %s", format, got[0])
		}
		errJson := []byte("{\"message\":\"request failed\",\"code\":500}")
		if !bytes.Equal(errJson, reply.Data) {
			t.Errorf("responses do not match, expected %s, received %s", string(msg.Data), string(reply.Data))
		}
	})
}

func TestRequestId(t *testing.T) {
	// define a handler that always fails
	errHandler := func(msg *NatsMsg) error {
		return errors.New("request failed")
	}

	t.Run("return default request id tag", func(t *testing.T) {
		// Create test server and router
		s, nr := getServerAndRouter(t)
		defer s.Shutdown()

		nc := nr.Conn()
		defer nr.Close()

		sub := nr.Subject("foo")
		// _, err := sub.Subscribe(emptyHandler)
		_, err := sub.Subscribe(errHandler)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Create message and send a request
		msg := nats.NewMsg("foo")
		msg.Data = []byte("data")
		reqId := "req-1"
		msg.Header.Add("request_id", reqId)

		reply, err := nc.RequestMsg(msg, 1*time.Second)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		got := reply.Header.Get("request_id")
		if got != reqId {
			t.Errorf("header request_id does not match, expected %s, received %s", reqId, got)
		}
	})

	t.Run("return customized request id tag", func(t *testing.T) {
		// Create test server and router
		s := getServer(t)
		defer s.Shutdown()

		// Create router and connect to test server
		tag := "reqid"
		nr, err := Connect(s.Addr().String(), WithRequestIdTag(tag))
		if err != nil {
			t.Fatalf("Could not connect to NATS server: %v", err)
		}

		nc := nr.Conn()
		defer nr.Close()

		sub := nr.Subject("foo")
		// _, err := sub.Subscribe(emptyHandler)
		_, err = sub.Subscribe(errHandler)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Create message and send a request
		msg := nats.NewMsg("foo")
		msg.Data = []byte("data")
		reqId := "req-1"
		msg.Header.Add(tag, reqId)

		reply, err := nc.RequestMsg(msg, 1*time.Second)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		got := reply.Header.Get(tag)
		if got != reqId {
			t.Errorf("header request_id does not match, expected %s, received %s", reqId, got)
		}
	})
}

func TestChanSubscribe(t *testing.T) {
	// Create test server and router
	s, nr := getServerAndRouter(t)
	defer s.Shutdown()

	nc := nr.Conn()
	defer nr.Close()

	respond := func(ch chan *NatsMsg) {
		msg := <-ch
		err := msg.RespondWithOriginalHeaders(msg.Data)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}

	t.Run("channel-based subscribe", func(t *testing.T) {
		sub := nr.Subject("foo")
		ch := make(chan *NatsMsg, 4)
		// _, err := sub.Subscribe(emptyHandler)
		_, err := sub.ChanSubscribe(ch)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Start a function to read the request and send a response
		go respond(ch)

		// Create message and send a request
		msg := nats.NewMsg("foo")
		msg.Data = []byte("data")
		reqId := "req-1"
		msg.Header.Add("request_id", reqId)

		reply, err := nc.RequestMsg(msg, 1*time.Second)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		got := reply.Header.Get("request_id")
		if got != reqId {
			t.Errorf("header request_id does not match, expected %s, received %s", reqId, got)
		}
		if !bytes.Equal(msg.Data, reply.Data) {
			t.Errorf("responses do not match, expected %s, received %s", string(msg.Data), string(reply.Data))
		}
	})

	t.Run("channel-based queue subscribe", func(t *testing.T) {
		sub := nr.Queue("group").Subject("foo")
		ch := make(chan *NatsMsg, 4)
		// _, err := sub.Subscribe(emptyHandler)
		_, err := sub.ChanSubscribe(ch)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Start a function to read the request and send a response
		go respond(ch)

		// Create message and send a request
		msg := nats.NewMsg("foo")
		msg.Data = []byte("data")
		reqId := "req-1"
		msg.Header.Add("request_id", reqId)

		reply, err := nc.RequestMsg(msg, 1*time.Second)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		got := reply.Header.Get("request_id")
		if got != reqId {
			t.Errorf("header request_id does not match, expected %s, received %s", reqId, got)
		}
		if !bytes.Equal(msg.Data, reply.Data) {
			t.Errorf("responses do not match, expected %s, received %s", string(msg.Data), string(reply.Data))
		}
	})
}
