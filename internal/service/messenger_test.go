package service

import (
	"bytes"
	"testing"

	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/chatlog"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/crypto"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/storage"
)

func newMessenger() *Messenger {
	return New(chatlog.New(storage.NewInMemoryStorage()))
}

func newSender(t *testing.T) Sender {
	t.Helper()
	priv, pub, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	return Sender{ID: "alice", PublicKey: pub, PrivateKey: priv}
}

func TestSendPhotoEncryptsAndVerifies(t *testing.T) {
	m := newMessenger()
	sender := newSender(t)
	contentKey, _ := crypto.NewContentKey()
	photo := []byte("\xff\xd8\xff\xe0 fake JPEG bytes \x00\x01\x02")

	entry, err := m.SendPhoto("c1", sender, contentKey, photo, "image/jpeg", "cat.jpg")
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
	sender := newSender(t)
	key, _ := crypto.NewContentKey()
	big := make([]byte, MaxPhotoBytes+1)
	if _, err := m.SendPhoto("c1", sender, key, big, "image/png", ""); err == nil {
		t.Fatal("expected oversize photo to be rejected")
	}
}

func TestSendEncryptedTextRoundTrip(t *testing.T) {
	m := newMessenger()
	sender := newSender(t)
	key, _ := crypto.NewContentKey()

	entry, err := m.SendEncryptedText("c1", sender, key, "private hello")
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
