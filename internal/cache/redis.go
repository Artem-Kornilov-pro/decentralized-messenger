package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis is a Cache backed by Redis. It stores the latest Merkle root per chat
// and registered public keys per user.
type Redis struct {
	client *redis.Client
	ttl    time.Duration
}

// compile-time assertion that Redis satisfies the Cache port.
var _ Cache = (*Redis)(nil)

// NewRedis returns a Redis-backed cache. ttl <= 0 means keys never expire.
func NewRedis(addr, password string, db int, ttl time.Duration) *Redis {
	return &Redis{
		client: redis.NewClient(&redis.Options{Addr: addr, Password: password, DB: db}),
		ttl:    ttl,
	}
}

// Ping verifies connectivity to the Redis server.
func (r *Redis) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Close releases the underlying connection pool.
func (r *Redis) Close() error { return r.client.Close() }

func (r *Redis) ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 2*time.Second)
}

func (r *Redis) SetMerkleRoot(chatID, root string) {
	ctx, cancel := r.ctx()
	defer cancel()
	r.client.Set(ctx, "merkle_root:"+chatID, root, r.ttl)
}

func (r *Redis) GetMerkleRoot(chatID string) (string, bool) {
	ctx, cancel := r.ctx()
	defer cancel()
	root, err := r.client.Get(ctx, "merkle_root:"+chatID).Result()
	if err != nil {
		return "", false
	}
	return root, true
}

func (r *Redis) SetPublicKey(userID string, publicKey []byte) {
	ctx, cancel := r.ctx()
	defer cancel()
	r.client.Set(ctx, "pubkey:"+userID, publicKey, r.ttl)
}

func (r *Redis) GetPublicKey(userID string) ([]byte, bool) {
	ctx, cancel := r.ctx()
	defer cancel()
	key, err := r.client.Get(ctx, "pubkey:"+userID).Bytes()
	if err != nil {
		return nil, false
	}
	return key, true
}
