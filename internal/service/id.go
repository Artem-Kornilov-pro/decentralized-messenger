package service

import (
	"crypto/rand"
	"encoding/hex"
)

// newID returns a random 128-bit identifier as a 32-char hex string. It avoids
// an external UUID dependency while giving collision-resistant message IDs.
func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
