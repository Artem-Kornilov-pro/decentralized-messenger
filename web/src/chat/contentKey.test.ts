// @vitest-environment jsdom
import { beforeEach, describe, expect, it } from 'vitest'
import { clearContentKey, loadContentKey, saveContentKey } from './contentKey'

beforeEach(() => {
  localStorage.clear()
})

describe('content key storage', () => {
  it('returns null when nothing is stored for a chat', () => {
    expect(loadContentKey('c1')).toBeNull()
  })

  it('round-trips a key through localStorage', () => {
    const key = new Uint8Array(32).fill(7)
    saveContentKey('c1', key)
    expect(loadContentKey('c1')).toEqual(key)
  })

  it('keeps keys for different chats independent', () => {
    saveContentKey('c1', new Uint8Array(32).fill(1))
    saveContentKey('c2', new Uint8Array(32).fill(2))

    expect(loadContentKey('c1')).toEqual(new Uint8Array(32).fill(1))
    expect(loadContentKey('c2')).toEqual(new Uint8Array(32).fill(2))
  })

  it('clears the stored key for a chat without affecting others', () => {
    saveContentKey('c1', new Uint8Array(32).fill(1))
    saveContentKey('c2', new Uint8Array(32).fill(2))

    clearContentKey('c1')

    expect(loadContentKey('c1')).toBeNull()
    expect(loadContentKey('c2')).toEqual(new Uint8Array(32).fill(2))
  })

  it('returns null for malformed stored data instead of throwing', () => {
    localStorage.setItem('messenger.contentKey.c1', 'not valid base64!!')
    expect(loadContentKey('c1')).toBeNull()
  })
})
