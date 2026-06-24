package models

import "testing"

// TestNewMessageTimestampIsMillisecondAligned guards against a regression of
// the ScyllaDB round-trip bug: the storage adapter persists Timestamp in a
// CQL `timestamp` column (millisecond precision), so the value signed here
// must already be millisecond-aligned or a stored message would fail
// verification after being read back.
func TestNewMessageTimestampIsMillisecondAligned(t *testing.T) {
	msg := NewMessage("c1", "alice", []byte("pub"), []byte("hi"), ContentTypeText, "", false)
	if msg.Timestamp.Nanosecond()%1_000_000 != 0 {
		t.Fatalf("timestamp has sub-millisecond precision: %v", msg.Timestamp)
	}
}

// TestSigningPayloadStableAcrossMillisecondRoundTrip simulates what a lossy
// storage backend does to Timestamp (truncating to millisecond) and asserts
// the canonical signing payload — and therefore the signature and entry
// hash — is unaffected, since NewMessage already truncates before signing.
func TestSigningPayloadStableAcrossMillisecondRoundTrip(t *testing.T) {
	msg := NewMessage("c1", "alice", []byte("pub"), []byte("hi"), ContentTypeText, "", false)
	before := msg.SigningPayload()

	roundTripped := msg
	roundTripped.Timestamp = msg.Timestamp.Truncate(1_000_000) // 1ms, matching a CQL timestamp column
	after := roundTripped.SigningPayload()

	if string(before) != string(after) {
		t.Fatalf("signing payload changed after a millisecond-precision round trip:\nbefore: %s\nafter:  %s", before, after)
	}
}
