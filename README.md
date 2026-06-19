# Decentralized Messenger

[![CI](https://github.com/Artem-Kornilov-pro/decentralized-messenger/actions/workflows/ci.yml/badge.svg)](https://github.com/Artem-Kornilov-pro/decentralized-messenger/actions/workflows/ci.yml)

A decentralized messenger with cryptographic message verification — no blockchain required. Written in **Go**.

## Concept

Instead of a traditional blockchain, the system uses a **three-layer hybrid security model**:

| Layer | Mechanism | Guarantee |
|-------|-----------|-----------|
| 1 | Append-only log with chained hashes | Tamper-evident message history |
| 2 | Ed25519 digital signatures per message | Undeniable authorship |
| 3 | Merkle trees with periodic snapshots | Efficient integrity proofs |

### How It Works

1. The sender signs a message with their **Ed25519 private key**
2. The record is appended to the log with the **hash of the previous entry**
3. The chat's **Merkle tree is updated**
4. Every 100 messages a **snapshot** (Merkle root) is created automatically
5. New participants **sync via the latest snapshot** + new log entries — no full history download needed
6. Any participant can verify that no messages were deleted or altered by **walking the hash chain** and verifying signatures

### Advantages over Blockchain

- **High performance** — no mining or consensus protocol
- **GDPR-friendly** — configurable retention policies
- **Small storage footprint** — only hashes and signatures, not full blocks
- **Simple sync** — compare Merkle roots between nodes
- **No smart contracts** needed for basic messenger operations

## Architecture

```
cmd/
└── messenger/        # Node entrypoint (HTTP server + -demo mode)
internal/
├── models/           # Core types: SignedMessage, LogEntry, MerkleSnapshot
├── crypto/           # Ed25519 key management, signing, verification
├── merkle/           # Merkle root + inclusion proofs
├── chatlog/          # Append-only log: chaining, snapshots, verify, sync
├── storage/          # Persistence port: in-memory + ScyllaDB adapters
├── cache/            # Cache port: in-memory + Redis adapters
├── broker/           # Event port: in-memory + RabbitMQ adapters
├── service/          # High-level messenger façade
└── api/              # HTTP/JSON API (net/http)
```

The domain layer (`models`, `crypto`, `merkle`, `chatlog`) depends only on the
Go standard library. Infrastructure is hidden behind three ports —
`storage.Storage`, `cache.Cache`, `broker.Broker` — each with a zero-dependency
in-memory implementation (the default) and a production adapter:

| Port | In-memory default | Production adapter |
|------|-------------------|--------------------|
| `storage.Storage` | `storage.InMemoryStorage` | ScyllaDB (`gocql`) |
| `cache.Cache` | `cache.InMemory` | Redis (`go-redis`) |
| `broker.Broker` | `broker.InMemory` | RabbitMQ (`amqp091`) |

## Tech Stack

| Component | Technology |
|-----------|------------|
| Core logic & crypto | Go 1.24 (stdlib `crypto/ed25519`, `crypto/sha256`) |
| Message log storage | ScyllaDB (immutable critical data) |
| Key & root cache | Redis |
| Node synchronization | RabbitMQ |
| HTTP API | `net/http` |
| Monitoring | Grafana + Loki |
| Containerization | Docker |

## Getting Started

### Prerequisites

- Go 1.24+
- Docker & Docker Compose (for the full infrastructure stack)

### Build & test

```bash
git clone https://github.com/Artem-Kornilov-pro/decentralized-messenger.git
cd decentralized-messenger

go build ./...
go test ./...
```

Common tasks are wrapped in a `Makefile` (`make help` lists them):

```bash
make check        # gofmt-check + vet + build + test (the CI suite)
make demo         # run the in-process demonstration
make run          # start the HTTP node
make docker       # build the Docker image
make compose-up   # start the full stack
```

### Run the demo

A self-contained demonstration of signing, hash-chaining, and verification:

```bash
go run ./cmd/messenger -demo
```

### Run the HTTP node

```bash
go run ./cmd/messenger -addr :8080
```

| Method | Path | Description |
|--------|------|-------------|
| `GET`  | `/healthz` | Liveness probe |
| `POST` | `/keys` | Generate an Ed25519 key pair (dev only) |
| `POST` | `/keys/content` | Generate a symmetric content key (dev only) |
| `POST` | `/chats/{chatID}/messages` | Sign and append a text message |
| `POST` | `/chats/{chatID}/photos` | Encrypt, sign, and append a photo |
| `GET`  | `/chats/{chatID}/verify` | Verify full chat integrity |
| `GET`  | `/chats/{chatID}/sync` | Get the catch-up bundle for a new participant |

Example:

```bash
# mint a key pair (dev convenience)
curl -s -X POST localhost:8080/keys > keys.json

# send a message
curl -X POST localhost:8080/chats/demo/messages \
  -H 'Content-Type: application/json' \
  -d '{"sender_id":"alice","public_key":..., "private_key":..., "text":"hello"}'

# verify the whole chat
curl localhost:8080/chats/demo/verify
```

### Configuration

The node picks an adapter per port based on environment variables; unset means
the in-memory default. This lets the same binary run standalone or against the
full stack.

| Variable | Effect |
|----------|--------|
| `SCYLLA_HOSTS` | Comma-separated ScyllaDB hosts → ScyllaDB storage |
| `SCYLLA_KEYSPACE` | Keyspace name (default `messenger`) |
| `REDIS_ADDR` | `host:port` → Redis cache |
| `REDIS_PASSWORD` | Redis auth (optional) |
| `RABBITMQ_URL` | `amqp://…` → RabbitMQ broker |

Before first use, apply the ScyllaDB schema (exported as `storage.Schema`) to
your keyspace — it creates the immutable `log_entries` and `snapshots` tables.

### Run the infrastructure stack

```bash
docker compose up
```

The `messenger` service is pre-wired to the ScyllaDB, Redis, and RabbitMQ
containers via the environment variables above.

## Security Model

Every message in the system satisfies three cryptographic properties:

- **Authenticity** — Ed25519 signature proves the message came from the claimed sender
- **Integrity** — the chained hash makes any modification of a past message detectable
- **Completeness** — Merkle root snapshots let any party prove no messages are missing

### Encrypted content (text & photos)

Message bodies and photo attachments can be encrypted with a per-chat symmetric
key (**AES-256-GCM**) before they ever reach the server:

- The client encrypts the content and signs the **ciphertext**. The server
  stores and verifies only ciphertext — it never sees the content key, so it
  cannot read messages or photos.
- A node can still verify authorship and integrity (the signature and hash
  chain cover the ciphertext), keeping the security model intact.
- Only clients holding the chat's content key can decrypt. GCM authentication
  also detects any tampering with the stored bytes.

Photos are sent via `POST /chats/{chatID}/photos` with the encrypted bytes,
their MIME type, and an optional filename (plaintext capped at 10 MiB). The
message carries `content_type`, `filename`, and an `encrypted` flag, all bound
into the signature.

See [docs/architecture.md](docs/architecture.md) for diagrams and data flow.

## License

[MIT](LICENSE) © 2026 Artem Kornilov
