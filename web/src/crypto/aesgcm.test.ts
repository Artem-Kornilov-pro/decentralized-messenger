import { describe, expect, it } from 'vitest'
import { base64ToBytes } from './base64'
import { decrypt, encrypt, generateContentKey, MalformedCiphertextError } from './aesgcm'

describe('aesgcm', () => {
  it('round-trips plaintext through encrypt/decrypt', async () => {
    const key = generateContentKey()
    const plaintext = new TextEncoder().encode('a fake photo, sort of')

    const blob = await encrypt(key, plaintext)
    const got = await decrypt(key, blob)

    expect(got).toEqual(plaintext)
  })

  it('produces a different nonce (and ciphertext) each time', async () => {
    const key = generateContentKey()
    const plaintext = new TextEncoder().encode('same plaintext')

    const a = await encrypt(key, plaintext)
    const b = await encrypt(key, plaintext)

    expect(a).not.toEqual(b)
  })

  it('fails to decrypt with the wrong key', async () => {
    const blob = await encrypt(generateContentKey(), new TextEncoder().encode('secret'))
    await expect(decrypt(generateContentKey(), blob)).rejects.toThrow()
  })

  it('fails to decrypt tampered ciphertext', async () => {
    const key = generateContentKey()
    const blob = await encrypt(key, new TextEncoder().encode('secret'))
    blob[blob.length - 1] ^= 0xff // flip a bit in the tag/ciphertext

    await expect(decrypt(key, blob)).rejects.toThrow()
  })

  it('rejects a blob too short to contain a nonce', async () => {
    await expect(decrypt(generateContentKey(), new Uint8Array(4))).rejects.toBeInstanceOf(
      MalformedCiphertextError,
    )
  })

  // Golden cross-check: this exact key/blob pair was produced by Go's
  // internal/crypto.Encrypt (cipher.AEAD.Seal) with a fixed key and nonce.
  // If Web Crypto's AES-GCM framing ever diverged from Go's (nonce size,
  // tag placement/length), this would catch it — the same class of bug
  // SigningPayload's golden tests guard against.
  it('decrypts a blob produced by the real Go backend', async () => {
    const key = base64ToBytes('AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=')
    const blob = base64ToBytes(
      'ZGVmZ2hpamtsbW5vIH6yChbJMOxRD3+PtUlKmSehdHP7GNMfwvHFJtvX3DjxmiOyJWLvdasnG2WTTrmE/LkERlXAyQ==',
    )

    const plaintext = await decrypt(key, blob)

    expect(new TextDecoder().decode(plaintext)).toBe('hello from go, decrypt me in typescript')
  })
})
