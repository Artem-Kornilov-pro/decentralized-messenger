// Package chatlog implements the append-only message log: it chains entries by
// hash, verifies Ed25519 signatures, and emits Merkle snapshots every
// models.SnapshotInterval messages.
package chatlog

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/broker"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/cache"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/crypto"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/merkle"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/models"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/storage"
)

// ErrInvalidSignature is returned when a message fails Ed25519 verification.
var ErrInvalidSignature = errors.New("chatlog: invalid message signature")

// Log appends signed messages to per-chat append-only logs backed by a
// storage.Storage, maintaining the hash chain and Merkle snapshots. It caches
// the latest Merkle root and publishes log events to a broker.
type Log struct {
	store  storage.Storage
	cache  cache.Cache
	broker broker.Broker
	locks  sync.Map // chatID -> *sync.Mutex
}

// Option configures a Log.
type Option func(*Log)

// WithCache wires a cache for Merkle roots and public keys.
func WithCache(c cache.Cache) Option { return func(l *Log) { l.cache = c } }

// WithBroker wires a broker for log-event propagation.
func WithBroker(b broker.Broker) Option { return func(l *Log) { l.broker = b } }

// New returns a Log backed by the given storage. By default it uses no-op cache
// and broker; pass WithCache / WithBroker to attach real adapters.
func New(store storage.Storage, opts ...Option) *Log {
	l := &Log{store: store, cache: cache.Noop{}, broker: broker.Noop{}}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

func (l *Log) chatLock(chatID string) *sync.Mutex {
	m, _ := l.locks.LoadOrStore(chatID, &sync.Mutex{})
	return m.(*sync.Mutex)
}

// Append verifies the message, appends it to its chat's log with a chained
// hash, and creates a Merkle snapshot when the window fills. It returns the
// stored entry.
func (l *Log) Append(msg models.SignedMessage) (models.LogEntry, error) {
	if !crypto.VerifyMessage(msg) {
		return models.LogEntry{}, ErrInvalidSignature
	}

	lock := l.chatLock(msg.ChatID)
	lock.Lock()
	defer lock.Unlock()

	prevHash := models.GenesisHash
	var nextSeq uint64
	if last, err := l.store.LastEntry(msg.ChatID); err == nil {
		prevHash = last.EntryHash
		nextSeq = last.Sequence + 1
	} else if !errors.Is(err, storage.ErrNotFound) {
		return models.LogEntry{}, fmt.Errorf("read tip: %w", err)
	}

	entry := models.LogEntry{Sequence: nextSeq, Message: msg, PrevHash: prevHash}
	entry.EntryHash = entry.ComputeHash()

	if err := l.store.AppendEntry(msg.ChatID, entry); err != nil {
		return models.LogEntry{}, fmt.Errorf("append: %w", err)
	}

	_ = l.broker.Publish(broker.Event{
		Kind:      broker.EntryAppended,
		ChatID:    msg.ChatID,
		Sequence:  entry.Sequence,
		EntryHash: entry.EntryHash,
	})

	if (entry.Sequence+1)%models.SnapshotInterval == 0 {
		if err := l.snapshot(msg.ChatID, entry); err != nil {
			return models.LogEntry{}, fmt.Errorf("snapshot: %w", err)
		}
	}
	return entry, nil
}

// snapshot builds a Merkle root over every entry since the previous snapshot
// and persists it. Caller must hold the chat lock.
func (l *Log) snapshot(chatID string, tip models.LogEntry) error {
	var from uint64
	var index uint64
	if prev, err := l.store.LatestSnapshot(chatID); err == nil {
		from = prev.ToSequence + 1
		index = prev.SnapshotIndex + 1
	} else if !errors.Is(err, storage.ErrNotFound) {
		return err
	}

	window, err := l.store.EntriesSince(chatID, from)
	if err != nil {
		return err
	}
	leaves := make([]string, len(window))
	for i, e := range window {
		leaves[i] = e.EntryHash
	}

	root := merkle.Root(leaves)
	if err := l.store.SaveSnapshot(models.MerkleSnapshot{
		ChatID:        chatID,
		SnapshotIndex: index,
		FromSequence:  from,
		ToSequence:    tip.Sequence,
		MerkleRoot:    root,
		LastEntryHash: tip.EntryHash,
		CreatedAt:     time.Now().UTC(),
	}); err != nil {
		return err
	}

	l.cache.SetMerkleRoot(chatID, root)
	_ = l.broker.Publish(broker.Event{
		Kind:      broker.SnapshotCreated,
		ChatID:    chatID,
		Sequence:  tip.Sequence,
		EntryHash: tip.EntryHash,
	})
	return nil
}

// VerifyResult reports the outcome of a full-chain integrity check.
type VerifyResult struct {
	Valid   bool
	Entries uint64
	Reason  string // populated when Valid is false
}

// Verify walks a chat's entire log, checking the hash chain, recomputing each
// entry hash, and verifying every signature.
func (l *Log) Verify(chatID string) (VerifyResult, error) {
	entries, err := l.store.EntriesSince(chatID, 0)
	if err != nil {
		return VerifyResult{}, err
	}

	prevHash := models.GenesisHash
	for i, e := range entries {
		if e.Sequence != uint64(i) {
			return VerifyResult{Reason: fmt.Sprintf("sequence gap at %d", i)}, nil
		}
		if e.PrevHash != prevHash {
			return VerifyResult{Reason: fmt.Sprintf("broken chain at seq %d", e.Sequence)}, nil
		}
		if e.ComputeHash() != e.EntryHash {
			return VerifyResult{Reason: fmt.Sprintf("tampered entry at seq %d", e.Sequence)}, nil
		}
		if !crypto.VerifyMessage(e.Message) {
			return VerifyResult{Reason: fmt.Sprintf("bad signature at seq %d", e.Sequence)}, nil
		}
		prevHash = e.EntryHash
	}
	return VerifyResult{Valid: true, Entries: uint64(len(entries))}, nil
}

// SyncBundle is the minimal data a new participant needs to catch up: the
// latest snapshot (if any) plus all entries recorded after it.
type SyncBundle struct {
	Snapshot    *models.MerkleSnapshot `json:"snapshot,omitempty"`
	NewEntries  []models.LogEntry      `json:"new_entries"`
	CurrentHash string                 `json:"current_hash"`
}

// Sync returns the catch-up bundle for a chat. New participants verify the
// snapshot's Merkle root, then replay NewEntries from the snapshot tip.
func (l *Log) Sync(chatID string) (SyncBundle, error) {
	var bundle SyncBundle
	var from uint64

	if snap, err := l.store.LatestSnapshot(chatID); err == nil {
		bundle.Snapshot = &snap
		from = snap.ToSequence + 1
	} else if !errors.Is(err, storage.ErrNotFound) {
		return SyncBundle{}, err
	}

	entries, err := l.store.EntriesSince(chatID, from)
	if err != nil {
		return SyncBundle{}, err
	}
	bundle.NewEntries = entries

	if last, err := l.store.LastEntry(chatID); err == nil {
		bundle.CurrentHash = last.EntryHash
	} else if errors.Is(err, storage.ErrNotFound) {
		bundle.CurrentHash = models.GenesisHash
	} else {
		return SyncBundle{}, err
	}
	return bundle, nil
}
