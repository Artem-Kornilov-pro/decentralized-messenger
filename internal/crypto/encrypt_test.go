package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key, err := NewContentKey()
	if err != nil {
		t.Fatal(err)
	}
	plaintext := []byte("a photo's worth of bytes \x00\x01\x02")

	blob, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(blob, plaintext) {
		t.Fatal("ciphertext should not contain plaintext")
	}

	got, err := Decrypt(key, blob)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("round trip mismatch: %q", got)
	}
}

func TestDecryptWrongKeyFails(t *testing.T) {
	key, _ := NewContentKey()
	other, _ := NewContentKey()
	blob, _ := Encrypt(key, []byte("secret"))
	if _, err := Decrypt(other, blob); err == nil {
		t.Fatal("decrypt with wrong key should fail")
	}
}

func TestDecryptTamperedFails(t *testing.T) {
	key, _ := NewContentKey()
	blob, _ := Encrypt(key, []byte("secret"))
	blob[len(blob)-1] ^= 0xFF // flip a tag bit
	if _, err := Decrypt(key, blob); err == nil {
		t.Fatal("decrypt of tampered ciphertext should fail GCM auth")
	}
}

func TestEncryptRejectsBadKey(t *testing.T) {
	if _, err := Encrypt([]byte("short"), []byte("x")); err != ErrInvalidKey {
		t.Fatalf("expected ErrInvalidKey, got %v", err)
	}
}
