// Package models defines the core data types of the messenger: signed
// messages, append-only log entries, and Merkle snapshots.
package models

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"time"
)

// GenesisHash is the prev_hash of the first entry in any chat log.
const GenesisHash = "0000000000000000000000000000000000000000000000000000000000000000"

// SnapshotInterval is the number of messages between Merkle snapshots.
const SnapshotInterval = 100

// Content types carried by a message. Text is the default; photos use their
// concrete image MIME type.
const (
	ContentTypeText = "text/plain"
)

// CurrentSchemaVersion is the message schema version produced by this build.
// It is bound into the signing payload so a verifier always knows exactly which
// canonical format a signature covers; future format changes bump this constant
// without invalidating signatures over older versions.
const CurrentSchemaVersion = 1

// SignedMessage is a chat message authenticated by the sender's Ed25519 key.
//
// Content holds the message body or attachment bytes. When Encrypted is true it
// is AES-256-GCM ciphertext (the server stores and signs only ciphertext; only
// clients holding the chat's content key can decrypt it).
//
// PublicKey and Signature are NOT part of the signed payload; the payload is
// the deterministic content produced by SigningPayload.
type SignedMessage struct {
	SchemaVersion int       `json:"schema_version"`
	MessageID     string    `json:"message_id"`
	ChatID        string    `json:"chat_id"`
	SenderID      string    `json:"sender_id"`
	Content       []byte    `json:"content"`
	ContentType   string    `json:"content_type"`
	Filename      string    `json:"filename,omitempty"`
	Encrypted     bool      `json:"encrypted"`
	Timestamp     time.Time `json:"timestamp"`
	PublicKey     []byte    `json:"public_key"`
	Signature     []byte    `json:"signature"`
}

// SigningPayload returns the canonical bytes that are signed and verified.
//
// The encoding is deterministic across processes (sorted keys, no insignificant
// whitespace) and binds the sender's public key in, so a message can never be
// re-attributed to a different author.
func (m SignedMessage) SigningPayload() []byte {
	encrypted := "false"
	if m.Encrypted {
		encrypted = "true"
	}
	payload := map[string]string{
		"schema_version": strconv.Itoa(m.SchemaVersion),
		"message_id":     m.MessageID,
		"chat_id":        m.ChatID,
		"sender_id":      m.SenderID,
		"content":        base64.StdEncoding.EncodeToString(m.Content),
		"content_type":   m.ContentType,
		"filename":       m.Filename,
		"encrypted":      encrypted,
		"timestamp":      m.Timestamp.UTC().Format(time.RFC3339Nano),
		"public_key":     base64.StdEncoding.EncodeToString(m.PublicKey),
	}
	// json.Marshal of a map sorts keys lexicographically, giving us a stable
	// canonical form without insignificant whitespace.
	b, _ := json.Marshal(payload)
	return b
}

// LogEntry is an immutable record in a chat's append-only log.
//
// EntryHash chains the previous entry's hash with a digest of the full signed
// message (payload + signature), so altering any past message or its author
// breaks the chain.
type LogEntry struct {
	Sequence  uint64        `json:"sequence"`
	Message   SignedMessage `json:"message"`
	PrevHash  string        `json:"prev_hash"`
	EntryHash string        `json:"entry_hash"`
}

// ComputeHash derives the entry hash from the entry's contents. It does not
// mutate the entry; callers assign the result to EntryHash when appending and
// compare against it when verifying.
func (e LogEntry) ComputeHash() string {
	msgDigest := sha256.Sum256(append(e.Message.SigningPayload(), e.Message.Signature...))
	chained := e.PrevHash + ":" + hex.EncodeToString(msgDigest[:])
	// Prepend the sequence so identical messages at different positions differ.
	chained = formatUint(e.Sequence) + ":" + chained
	sum := sha256.Sum256([]byte(chained))
	return hex.EncodeToString(sum[:])
}

func formatUint(v uint64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

// MerkleSnapshot captures the Merkle root of a contiguous range of log entries.
type MerkleSnapshot struct {
	ChatID        string    `json:"chat_id"`
	SnapshotIndex uint64    `json:"snapshot_index"`
	FromSequence  uint64    `json:"from_sequence"`
	ToSequence    uint64    `json:"to_sequence"`
	MerkleRoot    string    `json:"merkle_root"`
	LastEntryHash string    `json:"last_entry_hash"`
	CreatedAt     time.Time `json:"created_at"`
}
