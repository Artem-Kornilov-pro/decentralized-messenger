// Package service offers a high-level façade over the append-only log: it
// constructs and signs messages and exposes send/verify/sync operations.
package service

import (
	"fmt"
	"time"

	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/chatlog"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/crypto"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/models"
)

// MaxPhotoBytes caps the plaintext size of a photo attachment (10 MiB).
const MaxPhotoBytes = 10 << 20

// Sender identifies who is sending and signing a message.
type Sender struct {
	ID         string
	PublicKey  []byte
	PrivateKey []byte
}

// Messenger is the application-facing service.
type Messenger struct {
	log *chatlog.Log
}

// New returns a Messenger backed by the given log.
func New(log *chatlog.Log) *Messenger {
	return &Messenger{log: log}
}

// SendText builds a SignedMessage from plain text, signs it, and appends it.
// The body is stored as-is (not encrypted); use SendEncryptedText for privacy.
func (m *Messenger) SendText(chatID, senderID string, publicKey, privateKey []byte, text string) (models.LogEntry, error) {
	return m.appendSigned(chatID, Sender{ID: senderID, PublicKey: publicKey, PrivateKey: privateKey},
		[]byte(text), models.ContentTypeText, "", false)
}

// SendEncryptedText encrypts the text with the chat's symmetric content key and
// appends the resulting ciphertext, signed by the sender.
func (m *Messenger) SendEncryptedText(chatID string, sender Sender, contentKey []byte, text string) (models.LogEntry, error) {
	ciphertext, err := crypto.Encrypt(contentKey, []byte(text))
	if err != nil {
		return models.LogEntry{}, fmt.Errorf("encrypt text: %w", err)
	}
	return m.appendSigned(chatID, sender, ciphertext, models.ContentTypeText, "", true)
}

// SendPhoto encrypts a photo with the chat's symmetric content key and appends
// the ciphertext, signed by the sender. contentType is the image MIME type
// (e.g. "image/jpeg"); filename is optional metadata.
func (m *Messenger) SendPhoto(chatID string, sender Sender, contentKey, photo []byte, contentType, filename string) (models.LogEntry, error) {
	if len(photo) == 0 {
		return models.LogEntry{}, fmt.Errorf("photo is empty")
	}
	if len(photo) > MaxPhotoBytes {
		return models.LogEntry{}, fmt.Errorf("photo exceeds %d bytes", MaxPhotoBytes)
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	ciphertext, err := crypto.Encrypt(contentKey, photo)
	if err != nil {
		return models.LogEntry{}, fmt.Errorf("encrypt photo: %w", err)
	}
	return m.appendSigned(chatID, sender, ciphertext, contentType, filename, true)
}

// DecryptContent decrypts a message's stored content with the chat's content
// key. It is the client-side counterpart to the SendEncrypted* methods.
func (m *Messenger) DecryptContent(msg models.SignedMessage, contentKey []byte) ([]byte, error) {
	if !msg.Encrypted {
		return msg.Content, nil
	}
	return crypto.Decrypt(contentKey, msg.Content)
}

// appendSigned assembles, signs, and appends a message.
func (m *Messenger) appendSigned(chatID string, sender Sender, content []byte, contentType, filename string, encrypted bool) (models.LogEntry, error) {
	msg := models.SignedMessage{
		MessageID:   newID(),
		ChatID:      chatID,
		SenderID:    sender.ID,
		Content:     content,
		ContentType: contentType,
		Filename:    filename,
		Encrypted:   encrypted,
		Timestamp:   time.Now().UTC(),
		PublicKey:   sender.PublicKey,
	}
	msg = crypto.SignMessage(msg, sender.PrivateKey)
	return m.log.Append(msg)
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

// Verify checks the full integrity of a chat's history.
func (m *Messenger) Verify(chatID string) (chatlog.VerifyResult, error) {
	return m.log.Verify(chatID)
}

// Sync returns the catch-up bundle for a new participant.
func (m *Messenger) Sync(chatID string) (chatlog.SyncBundle, error) {
	return m.log.Sync(chatID)
}
