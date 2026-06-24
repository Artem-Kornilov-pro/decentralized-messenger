// Package service offers a high-level façade over the append-only log: it
// submits pre-signed messages and exposes read/verify/sync/subscribe
// operations. Signing happens client-side (see models.NewMessage and
// crypto.SignMessage) — this service never sees a private key.
package service

import (
	"fmt"

	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/broker"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/chatlog"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/crypto"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/models"
)

// MaxPhotoBytes caps the plaintext size of a photo attachment (10 MiB).
const MaxPhotoBytes = 10 << 20

// MaxVideoBytes caps the plaintext size of a video attachment (50 MiB).
const MaxVideoBytes = 50 << 20

// Messenger is the application-facing service.
type Messenger struct {
	log *chatlog.Log
}

// New returns a Messenger backed by the given log.
func New(log *chatlog.Log) *Messenger {
	return &Messenger{log: log}
}

// Submit appends a message the caller has already signed (see
// models.NewMessage + crypto.SignMessage). The log re-verifies the signature
// and schema version before accepting it. maxContentBytes, if > 0, rejects
// content larger than that (used to cap photo/video attachments); pass 0 for
// no extra cap beyond the server's request body limit.
func (m *Messenger) Submit(msg models.SignedMessage, maxContentBytes int) (models.LogEntry, error) {
	if maxContentBytes > 0 && len(msg.Content) > maxContentBytes {
		return models.LogEntry{}, fmt.Errorf("content exceeds %d bytes", maxContentBytes)
	}
	return m.log.Append(msg)
}

// DecryptContent decrypts a message's stored content with the chat's content
// key. It is the client-side counterpart to client-side encryption before
// signing (see crypto.Encrypt).
func (m *Messenger) DecryptContent(msg models.SignedMessage, contentKey []byte) ([]byte, error) {
	if !msg.Encrypted {
		return msg.Content, nil
	}
	return crypto.Decrypt(contentKey, msg.Content)
}

// History returns up to limit messages of a chat starting at sequence from.
// Content stays encrypted; clients decrypt with DecryptContent.
func (m *Messenger) History(chatID string, from uint64, limit int) ([]models.LogEntry, error) {
	return m.log.History(chatID, from, limit)
}

// Message returns a single message by its sequence number.
func (m *Messenger) Message(chatID string, sequence uint64) (models.LogEntry, error) {
	return m.log.Entry(chatID, sequence)
}

// ProveInclusion returns a Merkle inclusion proof for a message, letting any
// participant verify it belongs to the chat's history without the full log.
func (m *Messenger) ProveInclusion(chatID string, sequence uint64) (chatlog.InclusionProof, error) {
	return m.log.ProveInclusion(chatID, sequence)
}

// VerifyMessage checks a single message: signature, schema version, entry hash,
// and its chain link to the previous entry.
func (m *Messenger) VerifyMessage(chatID string, sequence uint64) (chatlog.MessageVerification, error) {
	return m.log.VerifyEntry(chatID, sequence)
}

// Verify checks the full integrity of a chat's history.
func (m *Messenger) Verify(chatID string) (chatlog.VerifyResult, error) {
	return m.log.Verify(chatID)
}

// Sync returns the catch-up bundle for a new participant.
func (m *Messenger) Sync(chatID string) (chatlog.SyncBundle, error) {
	return m.log.Sync(chatID)
}

// Subscribe returns a channel of log events (new entries, sealed snapshots)
// across all chats, plus a cancel func the caller must call to stop delivery
// and release the subscription. Callers filter by ChatID themselves.
func (m *Messenger) Subscribe() (<-chan broker.Event, func(), error) {
	return m.log.Subscribe()
}
