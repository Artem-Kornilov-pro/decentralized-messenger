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
}

// Noop is a Broker that drops events. It is the default when none is wired.
type Noop struct{}

func (Noop) Publish(Event) error { return nil }

// InMemory is a concurrency-safe in-process Broker that fans out events to
// registered subscribers. Useful for tests and single-node demos.
type InMemory struct {
	mu          sync.RWMutex
	subscribers []chan Event
}

// NewInMemory returns an empty in-memory broker.
func NewInMemory() *InMemory {
	return &InMemory{}
}

// Subscribe returns a buffered channel that receives every future event.
func (b *InMemory) Subscribe() <-chan Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan Event, 64)
	b.subscribers = append(b.subscribers, ch)
	return ch
}

// Publish delivers the event to all subscribers without blocking; if a
// subscriber's buffer is full the event is dropped for that subscriber.
func (b *InMemory) Publish(event Event) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
	return nil
}
