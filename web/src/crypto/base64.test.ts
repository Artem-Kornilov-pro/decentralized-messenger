import { describe, expect, it } from 'vitest'
import { base64ToBytes, bytesToBase64 } from './base64'

describe('base64', () => {
  it('round-trips arbitrary bytes', () => {
    const bytes = new Uint8Array([0, 1, 2, 254, 255, 16, 32])
    expect(base64ToBytes(bytesToBase64(bytes))).toEqual(bytes)
  })

  it('matches Go base64.StdEncoding for "hello"', () => {
    expect(bytesToBase64(new TextEncoder().encode('hello'))).toBe('aGVsbG8=')
  })
})
