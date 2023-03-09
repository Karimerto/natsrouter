package natsrouter

import (
	"github.com/nats-io/nats.go"
)

// Queue group that can be subscribed to subjects
type Queue struct {
	n     *NatsRouter
	group string
}

// Returns a new `Queue` object with additional middleware functions
func (q *Queue) WithMiddleware(fns ...NatsMiddlewareFunc) *Queue {
	return &Queue{
		n:     q.n.Use(fns...),
		group: q.group,
	}
}

// Alias for `WithMiddleware`
func (q *Queue) Use(fns ...NatsMiddlewareFunc) *Queue {
	return q.WithMiddleware(fns...)
}

// Subscribe to a subject as a part of this queue group with the specified
// handler function
func (q *Queue) Subscribe(subject string, handler NatsCtxHandler) (*nats.Subscription, error) {
	return q.n.QueueSubscribe(subject, q.group, handler)
}

// Same as Subscribe, with channel support
func (q *Queue) ChanSubscribe(subject string, ch chan *NatsMsg) (*nats.Subscription, error) {
	return q.n.ChanQueueSubscribe(subject, q.group, ch)
}

// Create a new `Subject` object that is part of this `Queue` group
func (q *Queue) Subject(subjects ...string) *Subject {
	return &Subject{
		n:        q.n,
		queue:    q,
		subjects: subjects,
	}
}
