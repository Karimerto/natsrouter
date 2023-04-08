package natsrouter

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/nats-io/nats.go"
)

// Simple middleware that simulated EncodedConn-type subscriptions
type EncodedMiddleware struct {
	cb  Handler
	enc nats.Encoder
}

// handler := func(m *NatsMsg)
// handler := func(ctx context.Context, m *Msg)
// handler := func(ctx context.Context, p *person)
// handler := func(ctx context.Context, subject string, o *obj)
// handler := func(ctx context.Context, subject, reply string, o *obj)
type Handler interface{}

// Create a new EncodedConn middleware with given encoder type
func NewEncodedMiddleware(cb Handler, encType string) (*EncodedMiddleware, error) {
	e := &EncodedMiddleware{
		cb:  cb,
		enc: nats.EncoderForType(encType),
	}
	if e.enc == nil {
		return nil, fmt.Errorf("no encoder registered for '%s'", encType)
	}
	return e, nil
}

// Dissect the cb Handler's signature
func argInfo(cb Handler) (reflect.Type, int) {
	cbType := reflect.TypeOf(cb)
	if cbType.Kind() != reflect.Func {
		panic("natsrouter: Handler needs to be a func")
	}
	numArgs := cbType.NumIn()
	if numArgs == 0 {
		return nil, numArgs
	}
	return cbType.In(numArgs - 1), numArgs
}

var emptyNatsMsgType = reflect.TypeOf(&NatsMsg{})
var emptyMsgType = reflect.TypeOf(&nats.Msg{})

// Actual middleware function
func (e *EncodedMiddleware) EncodedMiddleware(NatsCtxHandler) NatsCtxHandler {
	return func(msg *NatsMsg) error {
		// Check how many arguments the middleware function takes
		argType, numArgs := argInfo(e.cb)
		if argType == nil {
			return errors.New("natsrouter: Handler requires at least one argument")
		}

		cbValue := reflect.ValueOf(e.cb)

		var oV []reflect.Value
		if argType == emptyNatsMsgType {
			oV = []reflect.Value{reflect.ValueOf(msg)}
		} else if argType == emptyMsgType {
			ctx := reflect.ValueOf(msg.Context())
			oV = []reflect.Value{ctx, reflect.ValueOf(msg.Msg)}
		} else {
			var oPtr reflect.Value
			if argType.Kind() != reflect.Ptr {
				oPtr = reflect.New(argType)
			} else {
				oPtr = reflect.New(argType.Elem())
			}
			if err := e.enc.Decode(msg.Subject, msg.Data, oPtr.Interface()); err != nil {
				return errors.New("natsrouter: Got an error trying to unmarshal: " + err.Error())
			}
			if argType.Kind() != reflect.Ptr {
				oPtr = reflect.Indirect(oPtr)
			}

			// Callback Arity
			switch numArgs {
			case 1:
				oV = []reflect.Value{oPtr}
			case 2:
				ctxV := reflect.ValueOf(msg.Context())
				oV = []reflect.Value{ctxV, oPtr}
			case 3:
				ctxV := reflect.ValueOf(msg.Context())
				subV := reflect.ValueOf(msg.Subject)
				oV = []reflect.Value{ctxV, subV, oPtr}
			case 4:
				ctxV := reflect.ValueOf(msg.Context())
				subV := reflect.ValueOf(msg.Subject)
				replyV := reflect.ValueOf(msg.Reply)
				oV = []reflect.Value{ctxV, subV, replyV, oPtr}
			}
		}

		// Call the handler function with the appropriate arguments
		cbValue.Call(oV)

		// Note: The "next" handler is not called
		return nil
	}
}
