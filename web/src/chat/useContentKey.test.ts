// @vitest-environment jsdom
import { act, renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { useContentKey } from './useContentKey'
import { loadContentKey } from './contentKey'

beforeEach(() => {
  localStorage.clear()
})

describe('useContentKey', () => {
  it('starts with whatever is already stored for the chat, or null', () => {
    const { result } = renderHook(() => useContentKey('c1'))
    expect(result.current.contentKey).toBeNull()
  })

  it('generate() creates and persists a 32-byte key', () => {
    const { result } = renderHook(() => useContentKey('c1'))

    act(() => {
      result.current.generate()
    })

    expect(result.current.contentKey).toHaveLength(32)
    expect(loadContentKey('c1')).toEqual(result.current.contentKey)
  })

  it('setFromBase64() adopts a pasted key', () => {
    const { result } = renderHook(() => useContentKey('c1'))
    const key = new Uint8Array(32).fill(9)
    const b64 = btoa(String.fromCharCode(...key))

    act(() => {
      result.current.setFromBase64(b64)
    })

    expect(result.current.contentKey).toEqual(key)
  })

  it('setFromBase64() rejects a key of the wrong length', () => {
    const { result } = renderHook(() => useContentKey('c1'))

    expect(() => result.current.setFromBase64(btoa('too short'))).toThrow()
    expect(result.current.contentKey).toBeNull()
  })

  it('clear() removes the stored key', () => {
    const { result } = renderHook(() => useContentKey('c1'))
    act(() => {
      result.current.generate()
    })

    act(() => {
      result.current.clear()
    })

    expect(result.current.contentKey).toBeNull()
    expect(loadContentKey('c1')).toBeNull()
  })

  it('reloads the key for the new chat when chatId changes', () => {
    const key1 = new Uint8Array(32).fill(1)
    const key2 = new Uint8Array(32).fill(2)
    const { result, rerender } = renderHook(({ chatId }) => useContentKey(chatId), {
      initialProps: { chatId: 'c1' },
    })

    act(() => {
      result.current.setFromBase64(btoa(String.fromCharCode(...key1)))
    })
    expect(result.current.contentKey).toEqual(key1)

    rerender({ chatId: 'c2' })
    expect(result.current.contentKey).toBeNull()

    act(() => {
      result.current.setFromBase64(btoa(String.fromCharCode(...key2)))
    })
    expect(result.current.contentKey).toEqual(key2)

    rerender({ chatId: 'c1' })
    expect(result.current.contentKey).toEqual(key1)
  })
})
