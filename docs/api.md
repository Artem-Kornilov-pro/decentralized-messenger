# HTTP API

The node exposes a small JSON API over HTTP. All examples assume it is running
locally:

```bash
go run ./cmd/messenger -addr :8080
# or: make run
```

Set a base URL for the snippets below:

```bash
BASE=http://localhost:8080
```

## Conventions

- All request and response bodies are JSON.
- **Binary fields are base64-encoded strings** in JSON. This applies to
  `public_key`, `private_key`, `content_key`, `photo`, and a message's
  `content`, `public_key`, and `signature`.
- Keys and content keys are generated client-side in real deployments. The
  `/keys` and `/keys/content` endpoints exist only as a local convenience —
  **never send a real private key to a server.**
- `chatID` is any string you choose; chats are created implicitly on first
  message.

## Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| `GET`  | `/healthz` | Liveness probe |
| `POST` | `/keys` | Generate an Ed25519 key pair (dev only) |
| `POST` | `/keys/content` | Generate a symmetric content key (dev only) |
| `POST` | `/chats/{chatID}/messages` | Sign and append a text message |
| `GET`  | `/chats/{chatID}/messages` | List history (paginated) |
| `GET`  | `/chats/{chatID}/messages/{sequence}` | Fetch one message by sequence |
| `GET`  | `/chats/{chatID}/messages/{sequence}/proof` | Merkle inclusion proof |
| `GET`  | `/chats/{chatID}/messages/{sequence}/verify` | Verify one message |
| `POST` | `/chats/{chatID}/photos` | Encrypt, sign, and append a photo |
| `GET`  | `/chats/{chatID}/verify` | Verify full chat integrity |
| `GET`  | `/chats/{chatID}/sync` | Catch-up bundle for a new participant |

---

### `GET /healthz`

```bash
curl -s $BASE/healthz
# {"status":"ok"}
```

### `POST /keys`

Mint an Ed25519 key pair (base64 fields).

```bash
curl -s -X POST $BASE/keys
# {"public_key":"<base64>","private_key":"<base64>"}
```

### `POST /keys/content`

Mint a 32-byte AES-256 content key for encrypting message bodies and photos.

```bash
curl -s -X POST $BASE/keys/content
# {"content_key":"<base64>"}
```

---

### `POST /chats/{chatID}/messages`

Sign a plain-text message and append it. The body is stored **as-is** (not
encrypted) — see the photo flow for encrypted content.

Request fields: `sender_id`, `public_key`, `private_key`, `text`.

```bash
# Generate a key pair and send a message in one go (jq assembles the body).
curl -s -X POST $BASE/keys > keys.json

jq -n --argjson k "$(cat keys.json)" \
  '{sender_id:"alice", public_key:$k.public_key, private_key:$k.private_key, text:"hello"}' \
| curl -s -X POST $BASE/chats/demo/messages -H 'Content-Type: application/json' -d @-
```

Response `201 Created` — a log entry:

```json
{
  "sequence": 0,
  "message": {
    "schema_version": 1,
    "message_id": "…",
    "chat_id": "demo",
    "sender_id": "alice",
    "content": "<base64>",
    "content_type": "text/plain",
    "encrypted": false,
    "timestamp": "2026-06-20T12:00:00Z",
    "public_key": "<base64>",
    "signature": "<base64>"
  },
  "prev_hash": "0000…0000",
  "entry_hash": "08ded891e4fa6408…"
}
```

### `GET /chats/{chatID}/messages`

Paginated history. Query params: `from` (start sequence, default `0`) and
`limit` (default `50`, max `200`).

```bash
curl -s "$BASE/chats/demo/messages?from=0&limit=50"
```

```json
{
  "messages": [ /* log entries */ ],
  "next_from": 50      // cursor for the next page, or null at the end
}
```

### `GET /chats/{chatID}/messages/{sequence}`

Fetch a single message by its position in the log. `404` if it does not exist.

```bash
curl -s $BASE/chats/demo/messages/0
```

### `GET /chats/{chatID}/messages/{sequence}/proof`

Return a **Merkle inclusion proof** for a message. Verify it by recomputing the
root from `entry_hash` and the `proof` path and comparing against `merkle_root`
(which you cross-check against a trusted snapshot). Returns `409 Conflict` until
the covering snapshot is sealed (every 100 messages).

```bash
curl -s $BASE/chats/demo/messages/7/proof
```

```json
{
  "chat_id": "demo",
  "sequence": 7,
  "entry_hash": "…",
  "snapshot_index": 0,
  "merkle_root": "…",
  "proof": [
    {"hash": "…", "is_left": false},
    {"hash": "…", "is_left": true}
  ]
}
```

Verification (pseudocode): start with `running = entry_hash`; for each node,
`running = is_left ? H(node.hash + running) : H(running + node.hash)`; accept if
`running == merkle_root`.

### `GET /chats/{chatID}/messages/{sequence}/verify`

Verify a single message without scanning the whole chat: supported schema
version, intact entry hash, valid Ed25519 signature, and a correct chain link to
its predecessor. `404` if the message does not exist.

```bash
curl -s $BASE/chats/demo/messages/0/verify
# {"sequence":0,"valid":true}
# on failure: {"sequence":0,"valid":false,"reason":"bad signature"}
```

---

### `POST /chats/{chatID}/photos`

Encrypt a photo with the chat's content key, sign the ciphertext, and append it.
The server stores and signs **only ciphertext** — it never sees the content key.

Request fields: `sender_id`, `public_key`, `private_key`, `content_key`,
`photo` (raw bytes, base64), `content_type` (e.g. `image/jpeg`), `filename`
(optional). Plaintext is capped at 10 MiB.

```bash
curl -s -X POST $BASE/keys > keys.json
curl -s -X POST $BASE/keys/content > ckey.json

# Build the request body: base64 the image into the `photo` field.
jq -n \
  --argjson k "$(cat keys.json)" \
  --argjson c "$(cat ckey.json)" \
  --arg photo "$(base64 -w0 cat.jpg)" \
  '{sender_id:"alice", public_key:$k.public_key, private_key:$k.private_key,
    content_key:$c.content_key, photo:$photo, content_type:"image/jpeg", filename:"cat.jpg"}' \
| curl -s -X POST $BASE/chats/demo/photos -H 'Content-Type: application/json' -d @-
```

Response `201 Created` — a log entry whose `message.encrypted` is `true` and
`message.content` is the ciphertext. To read the photo back, a client fetches
the message and decrypts `content` with the same `content_key` (AES-256-GCM,
nonce prepended). The server cannot do this for you by design.

---

### `GET /chats/{chatID}/verify`

Walk the entire log: check the hash chain, recompute every entry hash, and
verify every signature.

```bash
curl -s $BASE/chats/demo/verify
# {"valid":true,"entries":3}
# on failure: {"valid":false,"entries":3,"reason":"broken chain at seq 2"}
```

### `GET /chats/{chatID}/sync`

Catch-up bundle for a new participant: the latest sealed snapshot (if any) plus
every entry recorded after it, and the current tip hash.

```bash
curl -s $BASE/chats/demo/sync
```

```json
{
  "snapshot": { "chat_id": "demo", "snapshot_index": 0, "merkle_root": "…", "…": "…" },
  "new_entries": [ /* entries after the snapshot */ ],
  "current_hash": "…"
}
```

## Error responses

Errors are returned as `{"error": "message"}` with an appropriate status code:

| Status | Meaning |
|--------|---------|
| `400 Bad Request` | Malformed body or query parameter |
| `404 Not Found` | Message does not exist |
| `409 Conflict` | No snapshot yet covers the requested message |
| `422 Unprocessable Entity` | Invalid signature, unsupported schema version, oversized photo, etc. |
| `500 Internal Server Error` | Unexpected server failure |

## Schema version

Every message carries a `schema_version` that is **bound into the signature**.
A node only accepts and verifies versions it understands (currently `1`);
messages with an unknown version are rejected with `422`. Because the version is
part of the signed canonical payload, a future format change bumps the version
without ever making an old message's signature ambiguous or invalid.
