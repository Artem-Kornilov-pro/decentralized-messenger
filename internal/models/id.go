package models

import (
	"crypto/rand"
	"encoding/hex"
)

// NewMessageID returns a random 128-bit identifier as a 32-char hex string. It
// avoids an external UUID dependency while giving collision-resistant message
// IDs that clients generate locally when constructing a message to sign.
func NewMessageID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
