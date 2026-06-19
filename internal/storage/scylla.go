package storage

import (
	"time"

	"github.com/gocql/gocql"

	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/models"
)

// Schema is the CQL required by the ScyllaDB adapter. Apply it once per cluster
// before starting nodes. The keyspace name is the caller's responsibility.
//
// Critical data is immutable: entries are inserted with IF NOT EXISTS and never
// updated or deleted, satisfying the append-only guarantee at the storage tier.
const Schema = `
CREATE TABLE IF NOT EXISTS log_entries (
    chat_id      text,
    sequence     bigint,
    message_id   text,
    sender_id    text,
    content      blob,
    content_type text,
    filename     text,
    encrypted    boolean,
    ts           timestamp,
    public_key   blob,
    signature    blob,
    prev_hash    text,
    entry_hash   text,
    PRIMARY KEY (chat_id, sequence)
) WITH CLUSTERING ORDER BY (sequence ASC);

CREATE TABLE IF NOT EXISTS snapshots (
    chat_id         text,
    snapshot_index  bigint,
    from_sequence   bigint,
    to_sequence     bigint,
    merkle_root     text,
    last_entry_hash text,
    created_at      timestamp,
    PRIMARY KEY (chat_id, snapshot_index)
) WITH CLUSTERING ORDER BY (snapshot_index ASC);
`

// Scylla is a Storage backed by ScyllaDB (or Cassandra) via gocql.
type Scylla struct {
	session *gocql.Session
}

// compile-time assertion that Scylla satisfies the Storage port.
var _ Storage = (*Scylla)(nil)

// NewScylla connects to the cluster at the given hosts and uses the given
// keyspace. The keyspace and Schema must already exist.
func NewScylla(keyspace string, hosts ...string) (*Scylla, error) {
	cluster := gocql.NewCluster(hosts...)
	cluster.Keyspace = keyspace
	cluster.Consistency = gocql.Quorum
	cluster.Timeout = 5 * time.Second
	session, err := cluster.CreateSession()
	if err != nil {
		return nil, err
	}
	return &Scylla{session: session}, nil
}

// Close releases the session.
func (s *Scylla) Close() { s.session.Close() }

func (s *Scylla) AppendEntry(chatID string, entry models.LogEntry) error {
	m := entry.Message
	applied, err := s.session.Query(
		`INSERT INTO log_entries
		    (chat_id, sequence, message_id, sender_id, content, content_type, filename, encrypted, ts, public_key, signature, prev_hash, entry_hash)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) IF NOT EXISTS`,
		chatID, int64(entry.Sequence), m.MessageID, m.SenderID, m.Content, m.ContentType, m.Filename, m.Encrypted,
		m.Timestamp, m.PublicKey, m.Signature, entry.PrevHash, entry.EntryHash,
	).MapScanCAS(map[string]any{})
	if err != nil {
		return err
	}
	if !applied {
		// A row already occupies this (chat_id, sequence) — immutability holds.
		return ErrSequenceGap
	}
	return nil
}

func (s *Scylla) LastEntry(chatID string) (models.LogEntry, error) {
	iter := s.session.Query(
		`SELECT sequence, message_id, sender_id, content, content_type, filename, encrypted, ts, public_key, signature, prev_hash, entry_hash
		 FROM log_entries WHERE chat_id = ? ORDER BY sequence DESC LIMIT 1`,
		chatID,
	).Iter()
	entry, ok := scanEntry(chatID, iter)
	if err := iter.Close(); err != nil {
		return models.LogEntry{}, err
	}
	if !ok {
		return models.LogEntry{}, ErrNotFound
	}
	return entry, nil
}

func (s *Scylla) Entry(chatID string, sequence uint64) (models.LogEntry, error) {
	iter := s.session.Query(
		`SELECT sequence, message_id, sender_id, content, content_type, filename, encrypted, ts, public_key, signature, prev_hash, entry_hash
		 FROM log_entries WHERE chat_id = ? AND sequence = ?`,
		chatID, int64(sequence),
	).Iter()
	entry, ok := scanEntry(chatID, iter)
	if err := iter.Close(); err != nil {
		return models.LogEntry{}, err
	}
	if !ok {
		return models.LogEntry{}, ErrNotFound
	}
	return entry, nil
}

func (s *Scylla) EntriesSince(chatID string, fromSequence uint64) ([]models.LogEntry, error) {
	iter := s.session.Query(
		`SELECT sequence, message_id, sender_id, content, content_type, filename, encrypted, ts, public_key, signature, prev_hash, entry_hash
		 FROM log_entries WHERE chat_id = ? AND sequence >= ? ORDER BY sequence ASC`,
		chatID, int64(fromSequence),
	).Iter()

	var out []models.LogEntry
	for {
		entry, ok := scanEntry(chatID, iter)
		if !ok {
			break
		}
		out = append(out, entry)
	}
	if err := iter.Close(); err != nil {
		return nil, err
	}
	return out, nil
}

// scanEntry reads one row into a LogEntry; ok is false when the iterator is
// exhausted.
func scanEntry(chatID string, iter *gocql.Iter) (models.LogEntry, bool) {
	var (
		seq                                        int64
		messageID, senderID, contentType, filename string
		prevHash, entryHash                        string
		encrypted                                  bool
		content, publicKey, signature              []byte
		ts                                         time.Time
	)
	if !iter.Scan(&seq, &messageID, &senderID, &content, &contentType, &filename, &encrypted, &ts, &publicKey, &signature, &prevHash, &entryHash) {
		return models.LogEntry{}, false
	}
	return models.LogEntry{
		Sequence: uint64(seq),
		Message: models.SignedMessage{
			MessageID:   messageID,
			ChatID:      chatID,
			SenderID:    senderID,
			Content:     content,
			ContentType: contentType,
			Filename:    filename,
			Encrypted:   encrypted,
			Timestamp:   ts.UTC(),
			PublicKey:   publicKey,
			Signature:   signature,
		},
		PrevHash:  prevHash,
		EntryHash: entryHash,
	}, true
}

func (s *Scylla) SaveSnapshot(snap models.MerkleSnapshot) error {
	return s.session.Query(
		`INSERT INTO snapshots
		    (chat_id, snapshot_index, from_sequence, to_sequence, merkle_root, last_entry_hash, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		snap.ChatID, int64(snap.SnapshotIndex), int64(snap.FromSequence), int64(snap.ToSequence),
		snap.MerkleRoot, snap.LastEntryHash, snap.CreatedAt,
	).Exec()
}

func (s *Scylla) LatestSnapshot(chatID string) (models.MerkleSnapshot, error) {
	var (
		index, from, to int64
		root, lastHash  string
		createdAt       time.Time
	)
	err := s.session.Query(
		`SELECT snapshot_index, from_sequence, to_sequence, merkle_root, last_entry_hash, created_at
		 FROM snapshots WHERE chat_id = ? ORDER BY snapshot_index DESC LIMIT 1`,
		chatID,
	).Scan(&index, &from, &to, &root, &lastHash, &createdAt)
	if err == gocql.ErrNotFound {
		return models.MerkleSnapshot{}, ErrNotFound
	}
	if err != nil {
		return models.MerkleSnapshot{}, err
	}
	return models.MerkleSnapshot{
		ChatID:        chatID,
		SnapshotIndex: uint64(index),
		FromSequence:  uint64(from),
		ToSequence:    uint64(to),
		MerkleRoot:    root,
		LastEntryHash: lastHash,
		CreatedAt:     createdAt.UTC(),
	}, nil
}
