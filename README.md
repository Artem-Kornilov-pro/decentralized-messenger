# Decentralized Messenger

A decentralized messenger with cryptographic message verification — no blockchain required.

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
src/decentralized_messenger/
├── crypto/          # Ed25519 key management, signing, verification
├── log/             # Append-only message log with chained hashes
├── merkle/          # Merkle tree construction and snapshot management
├── storage/         # ScyllaDB adapter for logs and snapshots
├── cache/           # Redis adapter for public keys and Merkle roots
├── broker/          # RabbitMQ adapter for inter-node log sync
├── api/             # HTTP/WebSocket API (aiohttp)
└── models/          # Pydantic data models
```

## Tech Stack

| Component | Technology |
|-----------|------------|
| Core logic & crypto | Python 3.12 |
| Message log storage | ScyllaDB |
| Key & root cache | Redis |
| Node synchronization | RabbitMQ |
| Data validation | Pydantic v2 |
| HTTP / WebSocket | aiohttp |
| Monitoring | Grafana + Loki |
| Containerization | Docker |

## Getting Started

### Prerequisites

- Python 3.12+
- Docker & Docker Compose

### Install (development)

```bash
git clone https://github.com/Artem-Kornilov-pro/decentralized-messenger.git
cd decentralized-messenger

python -m venv .venv
source .venv/bin/activate  # Windows: .venv\Scripts\activate

pip install -e ".[dev]"
```

### Run tests

```bash
pytest
```

### Run with Docker

```bash
docker compose up
```

## Project Structure

```
decentralized-messenger/
├── src/
│   └── decentralized_messenger/   # Main package
├── tests/                         # Test suite
├── docker/                        # Dockerfiles per service
├── docs/                          # Extended documentation
├── pyproject.toml                 # Project metadata & dependencies
├── docker-compose.yml             # Full stack for local development
└── README.md
```

## Security Model

Every message in the system satisfies three cryptographic properties:

- **Authenticity** — Ed25519 signature proves the message came from the claimed sender
- **Integrity** — the chained hash makes any modification of a past message detectable
- **Completeness** — Merkle root snapshots let any party prove no messages are missing

## License

[MIT](LICENSE) © 2026 Artem Kornilov
