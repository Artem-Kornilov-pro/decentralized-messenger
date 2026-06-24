// Package broker defines the messaging port used to propagate log events
// between nodes and provides in-memory and no-op implementations.
//
// Production deployments back this port with RabbitMQ for guaranteed delivery
// of chat-state updates; the in-memory broker mirrors the same contract.
package broker

import "sync"

// EventKind distinguishes the log events published by a node.
type EventKind string

const (
	// EntryAppended is published for every new log entry.
	EntryAppended EventKind = "entry_appended"
	// SnapshotCreated is published when a Merkle snapshot is sealed.
	SnapshotCreated EventKind = "snapshot_created"
)

// Event is a notification that a chat's log advanced. Subscribers use it to
// pull new entries or snapshots from the originating node.
type Event struct {
	Kind      EventKind `json:"kind"`
	ChatID    string    `json:"chat_id"`
	Sequence  uint64    `json:"sequence"`
	EntryHash string    `json:"entry_hash"`
}

// Broker publishes and delivers log events.
type Broker interface {
	Publish(event Event) error
	// Subscribe returns a channel of future events, a cancel func the caller
	// must invoke to stop delivery and release any resources, and an error if
	// the subscription could not be established.
	Subscribe() (<-chan Event, func(), error)
}

// Noop is a Broker that drops events. It is the default when none is wired.
type Noop struct{}

func (Noop) Publish(Event) error { return nil }

// Subscribe returns a channel that never receives anything, since Noop drops
// every published event.
func (Noop) Subscribe() (<-chan Event, func(), error) {
	return make(chan Event), func() {}, nil
}

// InMemory is a concurrency-safe in-process Broker that fans out events to
// registered subscribers. Useful for tests and single-node demos.
type InMemory struct {
	mu          sync.Mutex
	nextID      int
	subscribers map[int]chan Event
}

// NewInMemory returns an empty in-memory broker.
func NewInMemory() *InMemory {
	return &InMemory{subscribers: make(map[int]chan Event)}
}

// Subscribe returns a buffered channel that receives every future event. The
// returned cancel func removes the subscriber and closes its channel; callers
// must call it when done to avoid leaking the registration.
func (b *InMemory) Subscribe() (<-chan Event, func(), error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := b.nextID
	b.nextID++
	ch := make(chan Event, 64)
	b.subscribers[id] = ch

	cancel := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if existing, ok := b.subscribers[id]; ok {
			delete(b.subscribers, id)
			close(existing)
		}
	}
	return ch, cancel, nil
}

// Publish delivers the event to all subscribers without blocking; if a
// subscriber's buffer is full the event is dropped for that subscriber.
func (b *InMemory) Publish(event Event) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ch := range b.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
	return nil
}
