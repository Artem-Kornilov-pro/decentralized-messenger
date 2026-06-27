// Persists the local Ed25519 identity in localStorage. The private key never
// leaves this browser — it's used only to sign messages locally (see
// crypto/signedMessage.ts), never sent to the server.
import { base64ToBytes, bytesToBase64 } from '../crypto/base64'
import type { Identity } from './types'

const STORAGE_KEY = 'messenger.identity'

interface StoredIdentity {
  senderId: string
  publicKey: string
  secretKey: string
}

export function loadIdentity(): Identity | null {
  const raw = localStorage.getItem(STORAGE_KEY)
  if (!raw) return null
  try {
    const stored: StoredIdentity = JSON.parse(raw)
    return {
      senderId: stored.senderId,
      publicKey: base64ToBytes(stored.publicKey),
      secretKey: base64ToBytes(stored.secretKey),
    }
  } catch {
    return null
  }
}

export function saveIdentity(identity: Identity): void {
  const stored: StoredIdentity = {
    senderId: identity.senderId,
    publicKey: bytesToBase64(identity.publicKey),
    secretKey: bytesToBase64(identity.secretKey),
  }
  localStorage.setItem(STORAGE_KEY, JSON.stringify(stored))
}

export function clearIdentity(): void {
  localStorage.removeItem(STORAGE_KEY)
}
