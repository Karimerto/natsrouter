// This is a package called `natsrouter` that provides functionality for routing
// NATS messages based on their subject. The package defines a type `Subject`
// that represents a NATS subject, and provides methods for creating and
// modifying `Subject` instances, including `Subject`, `Any`, `All`, and
// `Queue`. The package also defines constants `ANY_SUBJECT` and `ALL_SUBJECT`
// that represent wildcard subjects. The `Subject` type has a method `Subscribe`
// that can be used to subscribe to messages that match the subject, and the
// `Subject` type also has a method `getSubject` that returns the subject as a
// string and verifies that any "all" subjects are at the end of the subject
// chain.

package natsrouter

import (
	"errors"
	"strings"

	"github.com/nats-io/nats.go"
)

// ANY_SUBJECT is a wildcard subject that can match any single token in a subject string.
const ANY_SUBJECT = "*"

// ALL_SUBJECT is a wildcard subject that can match one or more tokens in a subject string.
const ALL_SUBJECT = ">"

// This error is returned when an "all" subject is not the last one in a subject chain.
var ErrNonLastAllSubject = errors.New("'all' subject must be last in subject chain")

// This type defines a subject in NATS messaging.
type Subject struct {
	n        *NatsRouter
	queue    *Queue
	subjects []string
}

// This method returns a new subject that includes the specified subject strings.
// It appends the new subject strings to the existing subjects slice.
func (s *Subject) Subject(subjects ...string) *Subject {
	return &Subject{
		n:        s.n,
		queue:    s.queue,
		subjects: append(s.subjects, subjects...),
	}
}

// This method returns a new subject that includes the wildcard ANY_SUBJECT.
func (s *Subject) Any() *Subject {
	return &Subject{
		n:        s.n,
		queue:    s.queue,
		subjects: append(s.subjects, ANY_SUBJECT),
	}
}

// This method returns a new subject that includes the wildcard ALL_SUBJECT.
func (s *Subject) All() *Subject {
	return &Subject{
		n:        s.n,
		queue:    s.queue,
		subjects: append(s.subjects, ALL_SUBJECT),
	}
}

// This method returns a new subject with a queue group that is used to load
// balance messages across multiple subscribers.
func (s *Subject) Queue(queue string) *Subject {
	return &Subject{
		n: s.n,
		queue: &Queue{
			n:     s.n,
			queue: queue,
		},
		subjects: s.subjects,
	}
}

// Create subject, and verify that possible ">" is the last in the chain
// This method creates a subject string from the subjects slice and verifies
// that the wildcard ALL_SUBJECT is the last in the chain.
// If the ALL_SUBJECT is not the last subject string, the method returns an error.
func (s *Subject) getSubject() (string, error) {
	idx := -1
	for i, sub := range s.subjects {
		if sub == ALL_SUBJECT {
			idx = i
			break
		}
	}
	if idx > -1 && idx < len(s.subjects)-1 {
		return "", ErrNonLastAllSubject
	}
	return strings.Join(s.subjects, "."), nil
}

// This function subscribes a NATS context handler to a subject or a queue.
func (s *Subject) Subscribe(handler NatsCtxHandler) (*nats.Subscription, error) {
	if s.queue != nil {
		subject, err := s.getSubject()
		if err != nil {
			return nil, err
		}
		return s.queue.Subscribe(subject, handler)
	} else {
		subject, err := s.getSubject()
		if err != nil {
			return nil, err
		}
		return s.n.Subscribe(subject, handler)
	}
}

// Same as Subscribe, with channel support
func (s *Subject) ChanSubscribe(ch chan *NatsMsg) (*nats.Subscription, error) {
	if s.queue != nil {
		subject, err := s.getSubject()
		if err != nil {
			return nil, err
		}
		return s.queue.ChanSubscribe(subject, ch)
	} else {
		subject, err := s.getSubject()
		if err != nil {
			return nil, err
		}
		return s.n.ChanSubscribe(subject, ch)
	}
}
