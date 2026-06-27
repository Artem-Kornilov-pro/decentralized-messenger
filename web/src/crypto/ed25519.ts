// Ed25519 key generation, signing, and verification, mirroring Go's
// internal/crypto/keys.go. The server never sees a private key — only the
// resulting signature.
import * as ed from '@noble/ed25519'

export interface KeyPair {
  publicKey: Uint8Array
  secretKey: Uint8Array
}

export async function generateKeyPair(): Promise<KeyPair> {
  return ed.keygenAsync()
}

export function sign(message: Uint8Array, secretKey: Uint8Array): Promise<Uint8Array> {
  return ed.signAsync(message, secretKey)
}

export function verify(
  signature: Uint8Array,
  message: Uint8Array,
  publicKey: Uint8Array,
): Promise<boolean> {
  return ed.verifyAsync(signature, message, publicKey)
}
