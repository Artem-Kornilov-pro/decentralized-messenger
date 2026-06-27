// @vitest-environment jsdom
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { IdentityGate } from './IdentityGate'

describe('IdentityGate', () => {
  it('disables submit until a display name is entered', async () => {
    const onCreate = vi.fn().mockResolvedValue(undefined)
    render(<IdentityGate onCreate={onCreate} />)

    expect(screen.getByRole('button', { name: /generate identity/i })).toBeDisabled()

    await userEvent.type(screen.getByPlaceholderText('display name'), 'alice')
    expect(screen.getByRole('button', { name: /generate identity/i })).toBeEnabled()
  })

  it('calls onCreate with the trimmed display name on submit', async () => {
    const onCreate = vi.fn().mockResolvedValue(undefined)
    render(<IdentityGate onCreate={onCreate} />)

    await userEvent.type(screen.getByPlaceholderText('display name'), '  alice  ')
    await userEvent.click(screen.getByRole('button', { name: /generate identity/i }))

    expect(onCreate).toHaveBeenCalledWith('alice')
  })

  it('does not submit a blank/whitespace-only name', async () => {
    const onCreate = vi.fn().mockResolvedValue(undefined)
    render(<IdentityGate onCreate={onCreate} />)

    await userEvent.type(screen.getByPlaceholderText('display name'), '   ')
    // The button is disabled, so the only way to "submit" is the form's
    // submit event directly (e.g. pressing Enter); the handler itself must
    // also guard against a blank trimmed value.
    await userEvent.keyboard('{Enter}')

    expect(onCreate).not.toHaveBeenCalled()
  })

  it('shows a busy state while onCreate is pending and disables the button', async () => {
    let resolveCreate: () => void = () => {}
    const onCreate = vi.fn(
      () =>
        new Promise<void>((resolve) => {
          resolveCreate = resolve
        }),
    )
    render(<IdentityGate onCreate={onCreate} />)

    await userEvent.type(screen.getByPlaceholderText('display name'), 'alice')
    await userEvent.click(screen.getByRole('button', { name: /generate identity/i }))

    expect(screen.getByRole('button', { name: /generating/i })).toBeDisabled()
    resolveCreate()
  })
})
