// Persists each chat's AES-256-GCM content key in localStorage, keyed by
// chatId. Never sent to the server — participants share it out of band
// (copy/paste). Same pattern as identity/storage.ts.
import { base64ToBytes, bytesToBase64 } from '../crypto/base64'

function storageKey(chatId: string): string {
  return `messenger.contentKey.${chatId}`
}

export function loadContentKey(chatId: string): Uint8Array | null {
  const raw = localStorage.getItem(storageKey(chatId))
  if (!raw) return null
  try {
    return base64ToBytes(raw)
  } catch {
    return null
  }
}

export function saveContentKey(chatId: string, key: Uint8Array): void {
  localStorage.setItem(storageKey(chatId), bytesToBase64(key))
}

export function clearContentKey(chatId: string): void {
  localStorage.removeItem(storageKey(chatId))
}
