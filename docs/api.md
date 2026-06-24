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
  `public_key`, `content_key`, and a message's `content`, `public_key`, and
  `signature`.
- **Clients sign locally and never send a private key to a server.** Build a
  message with `models.NewMessage(...)`, sign it with `crypto.SignMessage`,
  then POST the result to `/messages`, `/photos`, or `/videos` — see those
  sections below for a worked example. The `/keys` and `/keys/content`
  endpoints exist only as a local convenience for minting test identities;
  they are unrelated to sending and never used by the send endpoints.
- `chatID` is any string you choose; chats are created implicitly on first
  message. The `chat_id` you signed must match the `chatID` in the URL — the
  server binds it from the path, so a message signed for a different chat
  fails verification (`422`).

## Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| `GET`  | `/healthz` | Liveness probe |
| `POST` | `/keys` | Generate an Ed25519 key pair (dev only) |
| `POST` | `/keys/content` | Generate a symmetric content key (dev only) |
| `POST` | `/chats/{chatID}/messages` | Append a pre-signed text message |
| `GET`  | `/chats/{chatID}/messages` | List history (paginated) |
| `GET`  | `/chats/{chatID}/messages/{sequence}` | Fetch one message by sequence |
| `GET`  | `/chats/{chatID}/messages/{sequence}/proof` | Merkle inclusion proof |
| `GET`  | `/chats/{chatID}/messages/{sequence}/verify` | Verify one message |
| `POST` | `/chats/{chatID}/photos` | Append a pre-signed, encrypted photo |
| `POST` | `/chats/{chatID}/videos` | Append a pre-signed, encrypted video |
| `GET`  | `/chats/{chatID}/verify` | Verify full chat integrity |
| `GET`  | `/chats/{chatID}/sync` | Catch-up bundle for a new participant |
| `GET`  | `/chats/{chatID}/ws` | WebSocket stream of new-entry/snapshot events |

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

Append a text message you have already signed locally. The body is stored
**as-is** (not encrypted) — see the photo/video flow for encrypted content.

The request body is a complete `SignedMessage` (see the response shape
below): `schema_version`, `message_id`, `chat_id`, `sender_id`, `content`
(base64), `content_type`, `filename` (optional), `encrypted`, `timestamp`,
`public_key`, and `signature` — all base64 for binary fields. Since
`curl`/`jq` cannot compute an Ed25519 signature, build and sign it in Go:

```go
priv, pub, _ := crypto.GenerateKeyPair() // a real client persists this key pair locally

msg := models.NewMessage("demo", "alice", pub, []byte("hello"), models.ContentTypeText, "", false)
msg = crypto.SignMessage(msg, priv)

body, _ := json.Marshal(msg)
http.Post(BASE+"/chats/demo/messages", "application/json", bytes.NewReader(body))
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

Encrypt a photo with the chat's content key, sign the ciphertext locally, and
append it. The server stores and signs **only ciphertext** — it never sees the
content key (or a private key). Plaintext is capped at 10 MiB.

```go
contentKey, _ := crypto.NewContentKey() // shared with chat participants out of band
ciphertext, _ := crypto.Encrypt(contentKey, photoBytes)

msg := models.NewMessage("demo", "alice", pub, ciphertext, "image/jpeg", "cat.jpg", true)
msg = crypto.SignMessage(msg, priv)

body, _ := json.Marshal(msg)
http.Post(BASE+"/chats/demo/photos", "application/json", bytes.NewReader(body))
```

Response `201 Created` — a log entry whose `message.encrypted` is `true` and
`message.content` is the ciphertext. To read the photo back, a client fetches
the message and decrypts `content` with the same `content_key` (AES-256-GCM,
nonce prepended). The server cannot do this for you by design.

### `POST /chats/{chatID}/videos`

Identical to `/photos` — encrypt, sign, and append — but for video
attachments, with `content_type` set to the video's MIME type (e.g.
`video/mp4`). Plaintext is capped at 50 MiB.

---

### `GET /chats/{chatID}/ws`

Upgrades to a WebSocket and pushes a small JSON event for every new entry or
sealed snapshot in `chatID`, so clients don't have to poll `GET
/chats/{chatID}/messages`. Events are a notification only — fetch the actual
message via the REST endpoints above.

```json
{"kind":"entry_appended","chat_id":"demo","sequence":3,"entry_hash":"…"}
```

The server pings the connection periodically to keep it alive through
proxies; clients don't need to send anything.

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
| `429 Too Many Requests` | Client IP exceeded its rate limit; see `Retry-After` |
| `500 Internal Server Error` | Unexpected server failure |

## Rate limiting

Every endpoint except `/healthz` is throttled per client IP with a token
bucket (default `5` requests/second, burst `20`, configurable via the node's
`-rate-limit-rps`/`-rate-limit-burst` flags). Exceeding it returns `429` with
a `Retry-After` header; back off and retry.

## Schema version

Every message carries a `schema_version` that is **bound into the signature**.
A node only accepts and verifies versions it understands (currently `1`);
messages with an unknown version are rejected with `422`. Because the version is
part of the signed canonical payload, a future format change bumps the version
without ever making an old message's signature ambiguous or invalid.
