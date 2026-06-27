// @vitest-environment jsdom
import { beforeEach, describe, expect, it } from 'vitest'
import { clearIdentity, loadIdentity, saveIdentity } from './storage'
import type { Identity } from './types'

beforeEach(() => {
  localStorage.clear()
})

describe('identity storage', () => {
  it('returns null when nothing is stored', () => {
    expect(loadIdentity()).toBeNull()
  })

  it('round-trips an identity through localStorage', () => {
    const identity: Identity = {
      senderId: 'alice',
      publicKey: new Uint8Array([1, 2, 3]),
      secretKey: new Uint8Array([4, 5, 6, 7]),
    }
    saveIdentity(identity)
    expect(loadIdentity()).toEqual(identity)
  })

  it('clears the stored identity', () => {
    saveIdentity({ senderId: 'alice', publicKey: new Uint8Array([1]), secretKey: new Uint8Array([2]) })
    clearIdentity()
    expect(loadIdentity()).toBeNull()
  })

  it('returns null for malformed stored data instead of throwing', () => {
    localStorage.setItem('messenger.identity', '{not json')
    expect(loadIdentity()).toBeNull()
  })
})
