// Standard (padded) base64 helpers, matching Go's base64.StdEncoding — the
// encoding used for every []byte field in the JSON API.

export function bytesToBase64(bytes: Uint8Array): string {
  let binary = ''
  for (const b of bytes) binary += String.fromCharCode(b)
  return btoa(binary)
}

export function base64ToBytes(b64: string): Uint8Array {
  const binary = atob(b64)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
  return bytes
}

export function utf8ToBytes(s: string): Uint8Array {
  return new TextEncoder().encode(s)
}
