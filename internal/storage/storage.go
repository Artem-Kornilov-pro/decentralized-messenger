// Package storage defines the persistence port for log entries and Merkle
// snapshots and provides an in-memory implementation for tests and local runs.
//
// Production deployments back this port with ScyllaDB configured for immutable
// critical data; the in-memory store mirrors the same contract.
package storage

import (
	"errors"
	"sync"

	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/models"
)

// ErrNotFound is returned when a requested entry or snapshot does not exist.
var ErrNotFound = errors.New("storage: not found")

// Storage is the append-only persistence port for a node.
type Storage interface {
	// AppendEntry persists a new log entry. Implementations must reject any
	// sequence that is not exactly one greater than the current tip.
	AppendEntry(chatID string, entry models.LogEntry) error
	// LastEntry returns the tip of a chat's log, or ErrNotFound if empty.
	LastEntry(chatID string) (models.LogEntry, error)
	// Entry returns a single entry by sequence, or ErrNotFound.
	Entry(chatID string, sequence uint64) (models.LogEntry, error)
	// EntriesSince returns entries with sequence >= fromSequence, in order.
	EntriesSince(chatID string, fromSequence uint64) ([]models.LogEntry, error)
	// SaveSnapshot persists a Merkle snapshot.
	SaveSnapshot(snap models.MerkleSnapshot) error
	// LatestSnapshot returns the most recent snapshot, or ErrNotFound.
	LatestSnapshot(chatID string) (models.MerkleSnapshot, error)
}

// ErrSequenceGap is returned when an appended entry is not contiguous.
var ErrSequenceGap = errors.New("storage: non-contiguous sequence")

// InMemoryStorage is a concurrency-safe in-memory Storage implementation.
type InMemoryStorage struct {
	mu        sync.RWMutex
	entries   map[string][]models.LogEntry
	snapshots map[string][]models.MerkleSnapshot
}

// NewInMemoryStorage returns an empty in-memory store.
func NewInMemoryStorage() *InMemoryStorage {
	return &InMemoryStorage{
		entries:   make(map[string][]models.LogEntry),
		snapshots: make(map[string][]models.MerkleSnapshot),
	}
}

func (s *InMemoryStorage) AppendEntry(chatID string, entry models.LogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing := s.entries[chatID]
	if uint64(len(existing)) != entry.Sequence {
		return ErrSequenceGap
	}
	s.entries[chatID] = append(existing, entry)
	return nil
}

func (s *InMemoryStorage) LastEntry(chatID string) (models.LogEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	existing := s.entries[chatID]
	if len(existing) == 0 {
		return models.LogEntry{}, ErrNotFound
	}
	return existing[len(existing)-1], nil
}

func (s *InMemoryStorage) Entry(chatID string, sequence uint64) (models.LogEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	existing := s.entries[chatID]
	if sequence >= uint64(len(existing)) {
		return models.LogEntry{}, ErrNotFound
	}
	return existing[sequence], nil
}

func (s *InMemoryStorage) EntriesSince(chatID string, fromSequence uint64) ([]models.LogEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	existing := s.entries[chatID]
	if fromSequence >= uint64(len(existing)) {
		return []models.LogEntry{}, nil
	}
	out := make([]models.LogEntry, len(existing)-int(fromSequence))
	copy(out, existing[fromSequence:])
	return out, nil
}

func (s *InMemoryStorage) SaveSnapshot(snap models.MerkleSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshots[snap.ChatID] = append(s.snapshots[snap.ChatID], snap)
	return nil
}

func (s *InMemoryStorage) LatestSnapshot(chatID string) (models.MerkleSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snaps := s.snapshots[chatID]
	if len(snaps) == 0 {
		return models.MerkleSnapshot{}, ErrNotFound
	}
	return snaps[len(snaps)-1], nil
}
