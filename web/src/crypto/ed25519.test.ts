import { describe, expect, it } from 'vitest'
import { generateKeyPair, sign, verify } from './ed25519'

describe('ed25519', () => {
  it('signs and verifies a message', async () => {
    const { publicKey, secretKey } = await generateKeyPair()
    const message = new TextEncoder().encode('hello world')

    const signature = await sign(message, secretKey)
    expect(await verify(signature, message, publicKey)).toBe(true)
  })

  it('rejects a signature after the message is tampered with', async () => {
    const { publicKey, secretKey } = await generateKeyPair()
    const message = new TextEncoder().encode('hello world')
    const signature = await sign(message, secretKey)

    const tampered = new TextEncoder().encode('hello WORLD')
    expect(await verify(signature, tampered, publicKey)).toBe(false)
  })
})
