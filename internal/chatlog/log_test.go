package chatlog

import (
	"testing"
	"time"

	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/broker"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/cache"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/crypto"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/models"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/storage"
)

func newSignedMessage(t *testing.T, chatID, text string, priv, pub []byte) models.SignedMessage {
	t.Helper()
	msg := models.SignedMessage{
		MessageID: text,
		ChatID:    chatID,
		SenderID:  "alice",
		Content:   []byte(text),
		Timestamp: time.Now().UTC(),
		PublicKey: pub,
	}
	return crypto.SignMessage(msg, priv)
}

func TestAppendChainsHashes(t *testing.T) {
	store := storage.NewInMemoryStorage()
	lg := New(store)
	priv, pub, _ := crypto.GenerateKeyPair()

	e0, err := lg.Append(newSignedMessage(t, "c1", "first", priv, pub))
	if err != nil {
		t.Fatal(err)
	}
	if e0.PrevHash != models.GenesisHash {
		t.Fatal("first entry should chain from genesis")
	}

	e1, err := lg.Append(newSignedMessage(t, "c1", "second", priv, pub))
	if err != nil {
		t.Fatal(err)
	}
	if e1.PrevHash != e0.EntryHash {
		t.Fatal("second entry should chain from first")
	}
	if e1.Sequence != 1 {
		t.Fatalf("expected sequence 1, got %d", e1.Sequence)
	}
}

func TestAppendRejectsBadSignature(t *testing.T) {
	lg := New(storage.NewInMemoryStorage())
	_, pub, _ := crypto.GenerateKeyPair()

	msg := models.SignedMessage{
		MessageID: "x",
		ChatID:    "c1",
		SenderID:  "alice",
		Content:   []byte("unsigned"),
		Timestamp: time.Now().UTC(),
		PublicKey: pub,
		Signature: []byte("not a real signature"),
	}
	if _, err := lg.Append(msg); err == nil {
		t.Fatal("expected invalid signature error")
	}
}

func TestVerifyValidLog(t *testing.T) {
	lg := New(storage.NewInMemoryStorage())
	priv, pub, _ := crypto.GenerateKeyPair()
	for i := 0; i < 5; i++ {
		if _, err := lg.Append(newSignedMessage(t, "c1", string(rune('a'+i)), priv, pub)); err != nil {
			t.Fatal(err)
		}
	}
	res, err := lg.Verify("c1")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid || res.Entries != 5 {
		t.Fatalf("expected valid log of 5 entries, got %+v", res)
	}
}

func TestSnapshotCreatedAtInterval(t *testing.T) {
	store := storage.NewInMemoryStorage()
	lg := New(store)
	priv, pub, _ := crypto.GenerateKeyPair()

	for i := 0; i < models.SnapshotInterval; i++ {
		if _, err := lg.Append(newSignedMessage(t, "c1", string(rune(i)), priv, pub)); err != nil {
			t.Fatal(err)
		}
	}
	snap, err := store.LatestSnapshot("c1")
	if err != nil {
		t.Fatalf("expected a snapshot after %d messages: %v", models.SnapshotInterval, err)
	}
	if snap.ToSequence != models.SnapshotInterval-1 || snap.MerkleRoot == "" {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}
}

func TestAppendPublishesEventAndCachesRoot(t *testing.T) {
	store := storage.NewInMemoryStorage()
	c := cache.NewInMemory()
	b := broker.NewInMemory()
	sub := b.Subscribe()
	lg := New(store, WithCache(c), WithBroker(b))
	priv, pub, _ := crypto.GenerateKeyPair()

	entry, err := lg.Append(newSignedMessage(t, "c1", "hi", priv, pub))
	if err != nil {
		t.Fatal(err)
	}

	select {
	case evt := <-sub:
		if evt.Kind != broker.EntryAppended || evt.EntryHash != entry.EntryHash {
			t.Fatalf("unexpected event %+v", evt)
		}
	default:
		t.Fatal("expected an EntryAppended event")
	}

	// Fill the rest of the window to trigger a snapshot, then check the cache.
	for i := 1; i < models.SnapshotInterval; i++ {
		if _, err := lg.Append(newSignedMessage(t, "c1", string(rune(i)), priv, pub)); err != nil {
			t.Fatal(err)
		}
	}
	if root, ok := c.GetMerkleRoot("c1"); !ok || root == "" {
		t.Fatal("expected Merkle root cached after snapshot")
	}
}

func TestSyncReturnsTailAfterSnapshot(t *testing.T) {
	store := storage.NewInMemoryStorage()
	lg := New(store)
	priv, pub, _ := crypto.GenerateKeyPair()

	total := models.SnapshotInterval + 3
	for i := 0; i < total; i++ {
		if _, err := lg.Append(newSignedMessage(t, "c1", string(rune(i)), priv, pub)); err != nil {
			t.Fatal(err)
		}
	}
	bundle, err := lg.Sync("c1")
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Snapshot == nil {
		t.Fatal("expected a snapshot in sync bundle")
	}
	if len(bundle.NewEntries) != 3 {
		t.Fatalf("expected 3 tail entries, got %d", len(bundle.NewEntries))
	}
}
