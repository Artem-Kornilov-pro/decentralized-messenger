package cache

import (
	"bytes"
	"testing"
)

func TestInMemoryMerkleRoot(t *testing.T) {
	c := NewInMemory()
	if _, ok := c.GetMerkleRoot("c1"); ok {
		t.Fatal("expected miss on empty cache")
	}
	c.SetMerkleRoot("c1", "root123")
	got, ok := c.GetMerkleRoot("c1")
	if !ok || got != "root123" {
		t.Fatalf("expected root123, got %q ok=%t", got, ok)
	}
}

func TestInMemoryPublicKeyIsCopied(t *testing.T) {
	c := NewInMemory()
	key := []byte{1, 2, 3}
	c.SetPublicKey("u1", key)
	key[0] = 9 // mutate caller's slice

	got, ok := c.GetPublicKey("u1")
	if !ok || !bytes.Equal(got, []byte{1, 2, 3}) {
		t.Fatalf("cache should store an independent copy, got %v", got)
	}
}

func TestNoopAlwaysMisses(t *testing.T) {
	var c Cache = Noop{}
	c.SetMerkleRoot("c1", "x")
	if _, ok := c.GetMerkleRoot("c1"); ok {
		t.Fatal("noop cache should never hit")
	}
}
