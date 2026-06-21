package chatlog

import (
	"errors"
	"testing"
	"time"

	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/broker"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/cache"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/crypto"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/merkle"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/models"
	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/storage"
)

func newSignedMessage(t *testing.T, chatID, text string, priv, pub []byte) models.SignedMessage {
	t.Helper()
	msg := models.SignedMessage{
		SchemaVersion: models.CurrentSchemaVersion,
		MessageID:     text,
		ChatID:        chatID,
		SenderID:      "alice",
		Content:       []byte(text),
		Timestamp:     time.Now().UTC(),
		PublicKey:     pub,
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
		SchemaVersion: models.CurrentSchemaVersion,
		MessageID:     "x",
		ChatID:        "c1",
		SenderID:      "alice",
		Content:       []byte("unsigned"),
		Timestamp:     time.Now().UTC(),
		PublicKey:     pub,
		Signature:     []byte("not a real signature"),
	}
	if _, err := lg.Append(msg); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("expected ErrInvalidSignature, got %v", err)
	}
}

func TestAppendRejectsUnsupportedVersion(t *testing.T) {
	lg := New(storage.NewInMemoryStorage())
	priv, pub, _ := crypto.GenerateKeyPair()
	msg := models.SignedMessage{
		SchemaVersion: models.CurrentSchemaVersion + 1, // a version this node can't validate
		MessageID:     "x",
		ChatID:        "c1",
		SenderID:      "alice",
		Content:       []byte("hi"),
		Timestamp:     time.Now().UTC(),
		PublicKey:     pub,
	}
	msg = crypto.SignMessage(msg, priv) // a perfectly valid signature...
	if _, err := lg.Append(msg); !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("expected ErrUnsupportedVersion, got %v", err)
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

func TestVerifyEntryValid(t *testing.T) {
	store := storage.NewInMemoryStorage()
	lg := New(store)
	priv, pub, _ := crypto.GenerateKeyPair()
	for i := 0; i < 3; i++ {
		if _, err := lg.Append(newSignedMessage(t, "c1", string(rune('a'+i)), priv, pub)); err != nil {
			t.Fatal(err)
		}
	}
	for seq := uint64(0); seq < 3; seq++ {
		res, err := lg.VerifyEntry("c1", seq)
		if err != nil {
			t.Fatal(err)
		}
		if !res.Valid || res.Sequence != seq {
			t.Fatalf("seq %d: unexpected result %+v", seq, res)
		}
	}
}

func TestVerifyEntryDetectsTamper(t *testing.T) {
	store := storage.NewInMemoryStorage()
	lg := New(store)
	priv, pub, _ := crypto.GenerateKeyPair()

	// Craft an entry whose hash is valid for the original content, then forge the
	// content so the stored entry hash no longer matches.
	entry := models.LogEntry{Sequence: 0, Message: newSignedMessage(t, "c1", "hello", priv, pub), PrevHash: models.GenesisHash}
	entry.EntryHash = entry.ComputeHash()
	entry.Message.Content = []byte("forged")
	if err := store.AppendEntry("c1", entry); err != nil {
		t.Fatal(err)
	}

	res, err := lg.VerifyEntry("c1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if res.Valid || res.Reason == "" {
		t.Fatalf("expected tamper to be detected, got %+v", res)
	}
}

func TestVerifyEntryNotFound(t *testing.T) {
	lg := New(storage.NewInMemoryStorage())
	if _, err := lg.VerifyEntry("c1", 0); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestProveInclusionVerifies(t *testing.T) {
	lg := New(storage.NewInMemoryStorage())
	priv, pub, _ := crypto.GenerateKeyPair()
	for i := 0; i < models.SnapshotInterval; i++ {
		if _, err := lg.Append(newSignedMessage(t, "c1", string(rune(i)), priv, pub)); err != nil {
			t.Fatal(err)
		}
	}

	for _, seq := range []uint64{0, 42, models.SnapshotInterval - 1} {
		proof, err := lg.ProveInclusion("c1", seq)
		if err != nil {
			t.Fatalf("seq %d: %v", seq, err)
		}
		if !merkle.VerifyProof(proof.EntryHash, proof.Proof, proof.MerkleRoot) {
			t.Fatalf("seq %d: proof failed to verify", seq)
		}
		// A wrong entry hash must not verify against the same proof.
		if merkle.VerifyProof("tampered", proof.Proof, proof.MerkleRoot) {
			t.Fatalf("seq %d: tampered leaf verified", seq)
		}
	}
}

func TestProveInclusionNotSnapshotted(t *testing.T) {
	lg := New(storage.NewInMemoryStorage())
	priv, pub, _ := crypto.GenerateKeyPair()
	// Only a handful of messages — no snapshot sealed yet.
	for i := 0; i < 5; i++ {
		if _, err := lg.Append(newSignedMessage(t, "c1", string(rune(i)), priv, pub)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := lg.ProveInclusion("c1", 2); !errors.Is(err, ErrNotSnapshotted) {
		t.Fatalf("expected ErrNotSnapshotted, got %v", err)
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
