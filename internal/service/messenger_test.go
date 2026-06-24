package service

import (
	"bytes"
	"testing"

	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/chatlog"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/crypto"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/models"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/storage"
)

func newMessenger() *Messenger {
	return New(chatlog.New(storage.NewInMemoryStorage()))
}

// identity is a test-only stand-in for a client's local key material.
type identity struct {
	id         string
	publicKey  []byte
	privateKey []byte
}

func newIdentity(t *testing.T) identity {
	t.Helper()
	priv, pub, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	return identity{id: "alice", publicKey: pub, privateKey: priv}
}

// signAttachment builds and signs an encrypted attachment the way a real
// client would: encrypt locally, then sign the ciphertext.
func signAttachment(t *testing.T, sender identity, chatID string, contentKey, data []byte, contentType, filename string) models.SignedMessage {
	t.Helper()
	ciphertext, err := crypto.Encrypt(contentKey, data)
	if err != nil {
		t.Fatal(err)
	}
	msg := models.NewMessage(chatID, sender.id, sender.publicKey, ciphertext, contentType, filename, true)
	return crypto.SignMessage(msg, sender.privateKey)
}

func signText(t *testing.T, sender identity, chatID, text string) models.SignedMessage {
	t.Helper()
	msg := models.NewMessage(chatID, sender.id, sender.publicKey, []byte(text), models.ContentTypeText, "", false)
	return crypto.SignMessage(msg, sender.privateKey)
}

func TestSendPhotoEncryptsAndVerifies(t *testing.T) {
	m := newMessenger()
	sender := newIdentity(t)
	contentKey, _ := crypto.NewContentKey()
	photo := []byte("\xff\xd8\xff\xe0 fake JPEG bytes \x00\x01\x02")

	msg := signAttachment(t, sender, "c1", contentKey, photo, "image/jpeg", "cat.jpg")
	entry, err := m.Submit(msg, MaxPhotoBytes)
	if err != nil {
		t.Fatal(err)
	}
	if !entry.Message.Encrypted || entry.Message.ContentType != "image/jpeg" || entry.Message.Filename != "cat.jpg" {
		t.Fatalf("unexpected message metadata: %+v", entry.Message)
	}
	if bytes.Contains(entry.Message.Content, photo) {
		t.Fatal("stored content must be ciphertext, not the raw photo")
	}

	// The signature over the ciphertext verifies without the content key.
	res, err := m.Verify("c1")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid {
		t.Fatalf("chat failed verification: %s", res.Reason)
	}

	// A client with the content key recovers the original photo.
	got, err := m.DecryptContent(entry.Message, contentKey)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, photo) {
		t.Fatal("decrypted photo does not match original")
	}
}

func TestSendPhotoRejectsOversize(t *testing.T) {
	m := newMessenger()
	sender := newIdentity(t)
	key, _ := crypto.NewContentKey()
	big := make([]byte, MaxPhotoBytes+1)

	msg := signAttachment(t, sender, "c1", key, big, "image/png", "")
	if _, err := m.Submit(msg, MaxPhotoBytes); err == nil {
		t.Fatal("expected oversize photo to be rejected")
	}
}

func TestSendVideoEncryptsAndVerifies(t *testing.T) {
	m := newMessenger()
	sender := newIdentity(t)
	contentKey, _ := crypto.NewContentKey()
	video := []byte("\x00\x00\x00\x18ftypmp42 fake MP4 bytes")

	msg := signAttachment(t, sender, "c1", contentKey, video, "video/mp4", "clip.mp4")
	entry, err := m.Submit(msg, MaxVideoBytes)
	if err != nil {
		t.Fatal(err)
	}
	if !entry.Message.Encrypted || entry.Message.ContentType != "video/mp4" || entry.Message.Filename != "clip.mp4" {
		t.Fatalf("unexpected message metadata: %+v", entry.Message)
	}

	got, err := m.DecryptContent(entry.Message, contentKey)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, video) {
		t.Fatal("decrypted video does not match original")
	}
}

func TestSendVideoRejectsOversize(t *testing.T) {
	m := newMessenger()
	sender := newIdentity(t)
	key, _ := crypto.NewContentKey()
	big := make([]byte, MaxVideoBytes+1)

	msg := signAttachment(t, sender, "c1", key, big, "video/mp4", "")
	if _, err := m.Submit(msg, MaxVideoBytes); err == nil {
		t.Fatal("expected oversize video to be rejected")
	}
}

func TestSendEncryptedTextRoundTrip(t *testing.T) {
	m := newMessenger()
	sender := newIdentity(t)
	key, _ := crypto.NewContentKey()

	msg := signAttachment(t, sender, "c1", key, []byte("private hello"), models.ContentTypeText, "")
	entry, err := m.Submit(msg, 0)
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.DecryptContent(entry.Message, key)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "private hello" {
		t.Fatalf("got %q", got)
	}
}

func TestSubmitRejectsMessageSignedForAnotherChat(t *testing.T) {
	m := newMessenger()
	sender := newIdentity(t)

	msg := signText(t, sender, "chat-a", "hello")
	msg.ChatID = "chat-b" // tampering after signing, like the API layer rebinding chat_id from the path
	if _, err := m.Submit(msg, 0); err == nil {
		t.Fatal("expected submit to reject a message signed for a different chat")
	}
}
