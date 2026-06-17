// Package crypto provides Ed25519 key generation, signing, and verification,
// including helpers that operate directly on SignedMessage values.
package crypto

import (
	"crypto/ed25519"
	"crypto/rand"

	"github.com/Artem-Kornilov-pro/decentralized-messenger/internal/models"
)

// GenerateKeyPair returns a fresh Ed25519 (privateKey, publicKey) pair as raw
// byte slices (64 bytes and 32 bytes respectively).
func GenerateKeyPair() (priv, pub []byte, err error) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return privKey, pubKey, nil
}

// Sign signs data with the given raw Ed25519 private key.
func Sign(privateKey, data []byte) []byte {
	return ed25519.Sign(ed25519.PrivateKey(privateKey), data)
}

// Verify reports whether sig is a valid signature of data under publicKey.
func Verify(publicKey, data, sig []byte) bool {
	if len(publicKey) != ed25519.PublicKeySize {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(publicKey), data, sig)
}

// SignMessage signs the message's canonical payload and returns a copy with the
// signature attached.
func SignMessage(msg models.SignedMessage, privateKey []byte) models.SignedMessage {
	msg.Signature = Sign(privateKey, msg.SigningPayload())
	return msg
}

// VerifyMessage reports whether the message carries a valid signature under its
// own public key.
func VerifyMessage(msg models.SignedMessage) bool {
	if len(msg.Signature) == 0 {
		return false
	}
	return Verify(msg.PublicKey, msg.SigningPayload(), msg.Signature)
}
