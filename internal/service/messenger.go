// Package service offers a high-level façade over the append-only log: it
// constructs and signs messages and exposes send/verify/sync operations.
package service

import (
	"time"

	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/chatlog"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/crypto"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/models"
)

// Messenger is the application-facing service.
type Messenger struct {
	log *chatlog.Log
}

// New returns a Messenger backed by the given log.
func New(log *chatlog.Log) *Messenger {
	return &Messenger{log: log}
}

// SendText builds a SignedMessage from plain text, signs it with the sender's
// private key, and appends it to the chat log.
func (m *Messenger) SendText(chatID, senderID string, publicKey, privateKey []byte, text string) (models.LogEntry, error) {
	msg := models.SignedMessage{
		MessageID: newID(),
		ChatID:    chatID,
		SenderID:  senderID,
		Content:   []byte(text),
		Timestamp: time.Now().UTC(),
		PublicKey: publicKey,
	}
	msg = crypto.SignMessage(msg, privateKey)
	return m.log.Append(msg)
}

// Verify checks the full integrity of a chat's history.
func (m *Messenger) Verify(chatID string) (chatlog.VerifyResult, error) {
	return m.log.Verify(chatID)
}

// Sync returns the catch-up bundle for a new participant.
func (m *Messenger) Sync(chatID string) (chatlog.SyncBundle, error) {
	return m.log.Sync(chatID)
}
