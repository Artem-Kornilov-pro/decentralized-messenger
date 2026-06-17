// Package cache defines the caching port for hot data — Merkle roots and user
// public keys — and provides in-memory and no-op implementations.
//
// Production deployments back this port with Redis; the in-memory store mirrors
// the same contract for tests and local runs.
package cache

import "sync"

// Cache stores the latest Merkle root per chat and registered public keys per
// user. Lookups that miss return (nil/"" , false).
type Cache interface {
	SetMerkleRoot(chatID, root string)
	GetMerkleRoot(chatID string) (string, bool)
	SetPublicKey(userID string, publicKey []byte)
	GetPublicKey(userID string) ([]byte, bool)
}

// Noop is a Cache that stores nothing. It is the default when no cache is wired.
type Noop struct{}

func (Noop) SetMerkleRoot(string, string)        {}
func (Noop) GetMerkleRoot(string) (string, bool) { return "", false }
func (Noop) SetPublicKey(string, []byte)         {}
func (Noop) GetPublicKey(string) ([]byte, bool)  { return nil, false }

// InMemory is a concurrency-safe in-memory Cache.
type InMemory struct {
	mu    sync.RWMutex
	roots map[string]string
	keys  map[string][]byte
}

// NewInMemory returns an empty in-memory cache.
func NewInMemory() *InMemory {
	return &InMemory{
		roots: make(map[string]string),
		keys:  make(map[string][]byte),
	}
}

func (c *InMemory) SetMerkleRoot(chatID, root string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.roots[chatID] = root
}

func (c *InMemory) GetMerkleRoot(chatID string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	root, ok := c.roots[chatID]
	return root, ok
}

func (c *InMemory) SetPublicKey(userID string, publicKey []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := make([]byte, len(publicKey))
	copy(cp, publicKey)
	c.keys[userID] = cp
}

func (c *InMemory) GetPublicKey(userID string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	key, ok := c.keys[userID]
	return key, ok
}
