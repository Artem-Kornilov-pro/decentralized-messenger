# Architecture

## Security Model

```
┌─────────────────────────────────────────────────┐
│                  Sender                         │
│  message ──► Ed25519 sign ──► SignedMessage     │
└──────────────────────┬──────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────┐
│              Append-Only Log                    │
│                                                 │
│  [entry_0: hash_0 = H(msg_0 | "genesis")]       │
│  [entry_1: hash_1 = H(msg_1 | hash_0)]          │
│  [entry_2: hash_2 = H(msg_2 | hash_1)]          │
│  ...                                            │
│  [entry_99: hash_99 = H(msg_99 | hash_98)]      │
│       │                                         │
│       └──► Merkle snapshot (root_0)             │
│  [entry_100 ...]                                │
└─────────────────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────┐
│              Merkle Tree                        │
│                                                 │
│              root                               │
│             /    \                              │
│           h01    h23                            │
│          /   \  /   \                           │
│         h0  h1 h2  h3   ← entry hashes         │
└─────────────────────────────────────────────────┘
```

## Data Flow

```
Client ──► API (net/http)
             │
             ├──► crypto.SignMessage()       # Ed25519 signature
             ├──► chatlog.Append()           # append-only log entry (chained hash)
             ├──► merkle.Root()              # update Merkle tree (every 100 msgs)
             ├──► storage.Storage            # persist log + snapshots (ScyllaDB)
             ├──► cache (Redis)              # cache Merkle root
             └──► broker (RabbitMQ)          # notify other nodes
```

## Snapshot Cycle

Every 100 log entries:
1. Collect `entry_hash` values for entries N–N+99
2. Build Merkle tree → compute root
3. Persist `MerkleSnapshot` to ScyllaDB
4. Update Merkle root in Redis
5. Publish snapshot event to RabbitMQ

## New Participant Sync

1. Fetch the latest `MerkleSnapshot` from the nearest node
2. Verify the Merkle root matches the claimed entry hashes
3. Download only log entries since the snapshot's `to_sequence`
4. Walk the hash chain forward from the snapshot's last hash
