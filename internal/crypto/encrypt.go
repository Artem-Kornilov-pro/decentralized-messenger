package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
)

// ContentKeySize is the required length of a symmetric content key (AES-256).
const ContentKeySize = 32

// ErrInvalidKey is returned when a content key is not ContentKeySize bytes.
var ErrInvalidKey = errors.New("crypto: content key must be 32 bytes")

// ErrMalformedCiphertext is returned when a blob is too short to contain a nonce.
var ErrMalformedCiphertext = errors.New("crypto: malformed ciphertext")

// NewContentKey returns a fresh random 32-byte AES-256 content key. Clients
// share this key out of band (per chat) and never send it to the server.
func NewContentKey() ([]byte, error) {
	key := make([]byte, ContentKeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}

// Encrypt seals plaintext with AES-256-GCM under key. The returned blob is
// nonce || ciphertext||tag and is what gets stored and signed.
func Encrypt(key, plaintext []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt opens a blob produced by Encrypt. It fails if the key is wrong or the
// ciphertext was tampered with (GCM authentication).
func Decrypt(key, blob []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(blob) < gcm.NonceSize() {
		return nil, ErrMalformedCiphertext
	}
	nonce, ciphertext := blob[:gcm.NonceSize()], blob[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func newGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != ContentKeySize {
		return nil, ErrInvalidKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
