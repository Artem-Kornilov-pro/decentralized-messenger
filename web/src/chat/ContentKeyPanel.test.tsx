// @vitest-environment jsdom
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ContentKeyPanel } from './ContentKeyPanel'

beforeEach(() => {
  Object.assign(navigator, { clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } })
})

describe('ContentKeyPanel', () => {
  it('shows a "no key" message and no copy field when there is no key', () => {
    render(<ContentKeyPanel contentKey={null} onGenerate={vi.fn()} onSetFromBase64={vi.fn()} />)

    expect(screen.getByText(/no content key set/i)).toBeInTheDocument()
    expect(screen.queryByLabelText('content key')).not.toBeInTheDocument()
  })

  it('shows the current key base64-encoded and copies it on click', async () => {
    const key = new Uint8Array(32).fill(5)
    render(<ContentKeyPanel contentKey={key} onGenerate={vi.fn()} onSetFromBase64={vi.fn()} />)

    const field = screen.getByLabelText('content key') as HTMLInputElement
    expect(field.value).toBe(btoa(String.fromCharCode(...key)))

    await userEvent.click(screen.getByRole('button', { name: /copy/i }))
    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(field.value)
  })

  it('calls onGenerate when "Generate new key" is clicked', async () => {
    const onGenerate = vi.fn()
    render(<ContentKeyPanel contentKey={null} onGenerate={onGenerate} onSetFromBase64={vi.fn()} />)

    await userEvent.click(screen.getByRole('button', { name: /generate new key/i }))
    expect(onGenerate).toHaveBeenCalledTimes(1)
  })

  it('calls onSetFromBase64 with the pasted value and clears the input on success', async () => {
    const onSetFromBase64 = vi.fn()
    render(<ContentKeyPanel contentKey={null} onGenerate={vi.fn()} onSetFromBase64={onSetFromBase64} />)

    const input = screen.getByPlaceholderText(/paste a key/i)
    await userEvent.type(input, 'some-key-text')
    await userEvent.click(screen.getByRole('button', { name: /^use$/i }))

    expect(onSetFromBase64).toHaveBeenCalledWith('some-key-text')
    expect(input).toHaveValue('')
  })

  it('shows an error and keeps the input when onSetFromBase64 throws', async () => {
    const onSetFromBase64 = vi.fn(() => {
      throw new Error('content key must decode to 32 bytes')
    })
    render(<ContentKeyPanel contentKey={null} onGenerate={vi.fn()} onSetFromBase64={onSetFromBase64} />)

    const input = screen.getByPlaceholderText(/paste a key/i)
    await userEvent.type(input, 'bad-key')
    await userEvent.click(screen.getByRole('button', { name: /^use$/i }))

    expect(screen.getByText(/must decode to 32 bytes/)).toBeInTheDocument()
    expect(input).toHaveValue('bad-key')
  })

  it('disables Use until something is pasted', () => {
    render(<ContentKeyPanel contentKey={null} onGenerate={vi.fn()} onSetFromBase64={vi.fn()} />)
    expect(screen.getByRole('button', { name: /^use$/i })).toBeDisabled()
  })
})
