// AES-256-GCM encryption for photo/video attachments, mirroring
// internal/crypto/encrypt.go. The server only ever stores ciphertext — it
// never sees the content key, the same way it never sees a private key.
const CONTENT_KEY_SIZE = 32
const NONCE_SIZE = 12

// TypeScript's DOM lib types Web Crypto's BufferSource as backed strictly by
// ArrayBuffer, while Uint8Array is now generically Uint8Array<ArrayBufferLike>
// (which also covers SharedArrayBuffer) — a lib.dom.d.ts/TS version friction
// point, not a real runtime concern here (these are always plain
// ArrayBuffer-backed views).
function asBufferSource(bytes: Uint8Array): BufferSource {
  return bytes as BufferSource
}

export class MalformedCiphertextError extends Error {
  constructor() {
    super('crypto: malformed ciphertext')
  }
}

/** A fresh random 32-byte AES-256 content key. Shared between chat
 * participants out of band (e.g. copy/paste) — never sent to the server. */
export function generateContentKey(): Uint8Array {
  const key = new Uint8Array(CONTENT_KEY_SIZE)
  crypto.getRandomValues(key)
  return key
}

async function importKey(key: Uint8Array): Promise<CryptoKey> {
  return crypto.subtle.importKey('raw', asBufferSource(key), 'AES-GCM', false, ['encrypt', 'decrypt'])
}

/** Seals plaintext with AES-256-GCM under key. The returned blob is
 * nonce || ciphertext||tag — the same layout Go's cipher.AEAD.Seal produces
 * — and is what gets stored and signed. */
export async function encrypt(key: Uint8Array, plaintext: Uint8Array): Promise<Uint8Array> {
  const cryptoKey = await importKey(key)
  const nonce = new Uint8Array(NONCE_SIZE)
  crypto.getRandomValues(nonce)

  const ciphertext = await crypto.subtle.encrypt(
    { name: 'AES-GCM', iv: asBufferSource(nonce) },
    cryptoKey,
    asBufferSource(plaintext),
  )

  const blob = new Uint8Array(nonce.length + ciphertext.byteLength)
  blob.set(nonce, 0)
  blob.set(new Uint8Array(ciphertext), nonce.length)
  return blob
}

/** Opens a blob produced by encrypt (here or in the Go backend/clients).
 * Throws if the key is wrong or the ciphertext was tampered with (GCM
 * authentication) — including MalformedCiphertextError if blob is too short
 * to contain a nonce. */
export async function decrypt(key: Uint8Array, blob: Uint8Array): Promise<Uint8Array> {
  if (blob.length < NONCE_SIZE) {
    throw new MalformedCiphertextError()
  }
  const cryptoKey = await importKey(key)
  const nonce = blob.slice(0, NONCE_SIZE)
  const ciphertext = blob.slice(NONCE_SIZE)

  const plaintext = await crypto.subtle.decrypt(
    { name: 'AES-GCM', iv: asBufferSource(nonce) },
    cryptoKey,
    asBufferSource(ciphertext),
  )
  return new Uint8Array(plaintext)
}
