import { describe, expect, it } from 'vitest'
import { signingPayload, type UnsignedMessage } from './signedMessage'
import { formatTimestamp } from './timestamp'

// Golden values captured by running internal/models.SignedMessage.SigningPayload()
// in Go for these exact inputs (see the plan / PR description for the
// one-off Go snippet used). Any mismatch here means a signature the
// frontend produces will fail server-side verification.
function fixedPublicKey(): Uint8Array {
  const pub = new Uint8Array(32)
  for (let i = 0; i < 32; i++) pub[i] = i
  return pub
}

describe('signingPayload', () => {
  it('matches Go for plain ASCII fields', () => {
    const m: UnsignedMessage = {
      schemaVersion: 1,
      messageId: '0123456789abcdef0123456789abcdef',
      chatId: 'demo',
      senderId: 'alice',
      content: new TextEncoder().encode('hello'),
      contentType: 'text/plain',
      filename: '',
      encrypted: false,
      timestamp: '2026-01-02T03:04:05.123456789Z',
      publicKey: fixedPublicKey(),
    }
    const got = new TextDecoder().decode(signingPayload(m))
    expect(got).toBe(
      '{"chat_id":"demo","content":"aGVsbG8=","content_type":"text/plain","encrypted":"false","filename":"","message_id":"0123456789abcdef0123456789abcdef","public_key":"AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=","schema_version":"1","sender_id":"alice","timestamp":"2026-01-02T03:04:05.123456789Z"}',
    )
  })

  it('matches Go HTML-escaping of <, >, &, ", \\\\ in sender_id and filename', () => {
    const m: UnsignedMessage = {
      schemaVersion: 1,
      messageId: '0123456789abcdef0123456789abcdef',
      chatId: 'demo',
      senderId: 'bob<3&"quote"\\back',
      content: new TextEncoder().encode('x'),
      contentType: 'text/plain',
      filename: '<script>&"x"\\y',
      encrypted: true,
      timestamp: '2026-01-02T03:04:05.123456789Z',
      publicKey: fixedPublicKey(),
    }
    const got = new TextDecoder().decode(signingPayload(m))
    expect(got).toBe(
      '{"chat_id":"demo","content":"eA==","content_type":"text/plain","encrypted":"true","filename":"\\u003cscript\\u003e\\u0026\\"x\\"\\\\y","message_id":"0123456789abcdef0123456789abcdef","public_key":"AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=","schema_version":"1","sender_id":"bob\\u003c3\\u0026\\"quote\\"\\\\back","timestamp":"2026-01-02T03:04:05.123456789Z"}',
    )
  })

  it('matches Go for non-ASCII (passed through as raw UTF-8, unescaped)', () => {
    const m: UnsignedMessage = {
      schemaVersion: 1,
      messageId: '0123456789abcdef0123456789abcdef',
      chatId: 'demo',
      senderId: 'héllo\u{1F600}世界',
      content: new TextEncoder().encode('y'),
      contentType: 'text/plain',
      filename: '',
      encrypted: false,
      timestamp: '2026-01-02T03:04:05.123456789Z',
      publicKey: fixedPublicKey(),
    }
    const got = new TextDecoder().decode(signingPayload(m))
    expect(got).toBe(
      '{"chat_id":"demo","content":"eQ==","content_type":"text/plain","encrypted":"false","filename":"","message_id":"0123456789abcdef0123456789abcdef","public_key":"AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8=","schema_version":"1","sender_id":"héllo\u{1F600}世界","timestamp":"2026-01-02T03:04:05.123456789Z"}',
    )
  })
})

describe('formatTimestamp', () => {
  it('omits the fractional part on an exact second (Go golden)', () => {
    expect(formatTimestamp(new Date('2026-01-02T03:04:05.000Z'))).toBe('2026-01-02T03:04:05Z')
  })

  it('pads and strips trailing zeros like Go RFC3339Nano (5ms)', () => {
    expect(formatTimestamp(new Date('2026-01-02T03:04:05.005Z'))).toBe('2026-01-02T03:04:05.005Z')
  })

  it('strips trailing zeros like Go RFC3339Nano (120ms)', () => {
    expect(formatTimestamp(new Date('2026-01-02T03:04:05.120Z'))).toBe('2026-01-02T03:04:05.12Z')
  })

  it('strips trailing zeros like Go RFC3339Nano (999ms)', () => {
    expect(formatTimestamp(new Date('2026-01-02T03:04:05.999Z'))).toBe('2026-01-02T03:04:05.999Z')
  })
})
