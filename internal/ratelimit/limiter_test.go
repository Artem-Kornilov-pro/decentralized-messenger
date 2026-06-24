package ratelimit

import (
	"testing"
	"time"
)

func TestAllowsUpToBurstThenBlocks(t *testing.T) {
	l := New(1, 3, time.Minute)
	defer l.Close()

	for i := 0; i < 3; i++ {
		if !l.Allow("k") {
			t.Fatalf("expected request %d within burst to be allowed", i)
		}
	}
	if l.Allow("k") {
		t.Fatal("expected request beyond burst to be blocked")
	}
}

func TestKeysAreIndependent(t *testing.T) {
	l := New(1, 1, time.Minute)
	defer l.Close()

	if !l.Allow("a") {
		t.Fatal("expected first request for key a to be allowed")
	}
	if !l.Allow("b") {
		t.Fatal("expected first request for key b to be allowed (independent bucket)")
	}
	if l.Allow("a") {
		t.Fatal("expected second request for key a to be blocked")
	}
}

func TestEvictIdleRemovesStaleEntries(t *testing.T) {
	l := New(1, 1, time.Minute)
	defer l.Close()

	l.Allow("stale")
	l.Allow("fresh")

	l.mu.Lock()
	l.buckets["stale"].lastSeen = time.Now().Add(-2 * time.Minute)
	l.mu.Unlock()

	l.evictIdle()

	l.mu.Lock()
	_, staleStillPresent := l.buckets["stale"]
	_, freshStillPresent := l.buckets["fresh"]
	l.mu.Unlock()

	if staleStillPresent {
		t.Fatal("expected stale entry to be evicted")
	}
	if !freshStillPresent {
		t.Fatal("expected fresh entry to survive eviction")
	}
}
