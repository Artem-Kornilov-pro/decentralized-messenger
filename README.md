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
| Real-time push | WebSocket (`gorilla/websocket`) |
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
| `POST` | `/chats/{chatID}/messages` | Append a pre-signed text message |
| `GET`  | `/chats/{chatID}/messages` | List history (`?from=&limit=`, paginated) |
| `GET`  | `/chats/{chatID}/messages/{sequence}` | Fetch a single message by sequence |
| `GET`  | `/chats/{chatID}/messages/{sequence}/proof` | Merkle inclusion proof for a message |
| `GET`  | `/chats/{chatID}/messages/{sequence}/verify` | Verify a single message |
| `POST` | `/chats/{chatID}/photos` | Append a pre-signed, encrypted photo |
| `POST` | `/chats/{chatID}/videos` | Append a pre-signed, encrypted video |
| `GET`  | `/chats/{chatID}/verify` | Verify full chat integrity |
| `GET`  | `/chats/{chatID}/sync` | Get the catch-up bundle for a new participant |
| `GET`  | `/chats/{chatID}/ws` | WebSocket stream of new-entry/snapshot events |

Clients sign messages locally and never send a private key to the server:

```go
priv, pub, _ := crypto.GenerateKeyPair() // persisted locally by the client

msg := models.NewMessage("demo", "alice", pub, []byte("hello"), models.ContentTypeText, "", false)
msg = crypto.SignMessage(msg, priv)

body, _ := json.Marshal(msg)
http.Post("http://localhost:8080/chats/demo/messages", "application/json", bytes.NewReader(body))
```

```bash
# verify the whole chat
curl localhost:8080/chats/demo/verify
```

See [docs/api.md](docs/api.md) for the full request/response shapes, the
photo/video/WebSocket flows, and a `curl`-only walkthrough of the read-only
endpoints.

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

### Operational hardening

The HTTP node is built for unattended operation rather than just demos:

- **Request timeouts** — `ReadHeaderTimeout` (5s), `ReadTimeout` (30s),
  `WriteTimeout` (30s), and `IdleTimeout` (120s) bound slow clients and defend
  against slowloris-style attacks.
- **Body size limit** — request bodies are capped at 72 MiB (headroom for a
  base64-encoded 50 MiB video); larger uploads get `413`.
- **Per-IP rate limiting** — every endpoint except `/healthz` is throttled per
  client IP via a token bucket (`-rate-limit-rps`, default `5`;
  `-rate-limit-burst`, default `20`); a client beyond its budget gets `429
  Too Many Requests` with a `Retry-After` header. Set `-rate-limit-rps=0` to
  disable. Note this keys on the immediate TCP peer, not
  `X-Forwarded-For` — behind a reverse proxy, every client shares the proxy's
  budget unless you front it with your own limiting.
- **Graceful shutdown** — on `SIGINT`/`SIGTERM` the server stops accepting new
  connections and drains in-flight requests within a 15s deadline before
  exiting, so deploys and restarts don't drop active requests.
- **Liveness** — `GET /healthz` for container/orchestrator probes.

### Run the infrastructure stack

```bash
docker compose up
```

The `messenger` service is pre-wired to the ScyllaDB, Redis, and RabbitMQ
containers via the environment variables above.

## Frontend

A React/TypeScript SPA in [`web/`](web/) — generate a local Ed25519 identity,
join a chat by ID, and send/receive text, photo, and video messages live
over `/ws`. It signs every message client-side (see "Client-side signing"
below); the server never sees a private key.

Photos/videos are AES-256-GCM-encrypted with a per-chat content key (see
"Encrypted content" below) that the frontend generates and stores locally —
participants share it out of band (copy/paste, via the chat's content-key
panel). Without a content key, text still works but media can't be sent or
viewed.

```bash
cd web
npm install
npm run dev    # http://localhost:5173, proxied to the API on :8080
```

Run a node first (`make run` or the Docker stack above) — the dev server
proxies `/chats`, `/keys`, and `/healthz` to `localhost:8080`. Proof/verify
UI isn't built yet; see `web/src/chat` and `web/src/identity` for the
current scope.

## Security Model

Every message in the system satisfies three cryptographic properties:

- **Authenticity** — Ed25519 signature proves the message came from the claimed sender
- **Integrity** — the chained hash makes any modification of a past message detectable
- **Completeness** — Merkle root snapshots let any party prove no messages are missing

### Inclusion proofs

`GET /chats/{chatID}/messages/{sequence}/proof` returns a **Merkle inclusion
proof**: the entry hash, the sibling hashes along its path, and the snapshot's
Merkle root. A participant verifies a message belongs to the chat's history by
recomputing the root from the entry hash and the proof — without downloading the
full log. Proofs become available once the covering snapshot is sealed (every
100 messages); requesting one earlier returns `409 Conflict`.

### Client-side signing

Clients build and sign messages locally (`models.NewMessage` +
`crypto.SignMessage`) and submit only the result — a server never sees a
private key, only the resulting signature. The `chat_id` a client signed must
match the `chatID` in the URL; the server binds it from the path, so a
message signed for one chat is rejected (`422`) if posted to another.

### Encrypted content (text, photos & videos)

Message bodies and attachments can be encrypted with a per-chat symmetric key
(**AES-256-GCM**) before they ever reach the server:

- The client encrypts the content and signs the **ciphertext**. The server
  stores and verifies only ciphertext — it never sees the content key, so it
  cannot read messages, photos, or videos.
- A node can still verify authorship and integrity (the signature and hash
  chain cover the ciphertext), keeping the security model intact.
- Only clients holding the chat's content key can decrypt. GCM authentication
  also detects any tampering with the stored bytes.

Photos are sent via `POST /chats/{chatID}/photos` (plaintext capped at 10
MiB) and videos via `POST /chats/{chatID}/videos` (capped at 50 MiB), each
with the encrypted bytes, MIME type, and an optional filename. The message
carries `content_type`, `filename`, and an `encrypted` flag, all bound into
the signature.

### Real-time delivery

`GET /chats/{chatID}/ws` upgrades to a WebSocket and pushes a small
notification for every new entry or sealed snapshot, so clients don't have to
poll history. Clients still fetch the actual message via the REST endpoints —
the socket only carries "something changed" events.

See [docs/api.md](docs/api.md) for the full HTTP API with examples (including
the signed-message, encrypted-photo/video, and WebSocket flows, and
inclusion-proof verification), and [docs/architecture.md](docs/architecture.md)
for diagrams and data flow.

## License

[MIT](LICENSE) © 2026 Artem Kornilov
