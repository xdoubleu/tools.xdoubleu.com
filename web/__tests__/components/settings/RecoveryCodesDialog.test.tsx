import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import RecoveryCodesDialog from '@/components/settings/RecoveryCodesDialog'

describe('RecoveryCodesDialog', () => {
  beforeEach(() => {
    Object.assign(navigator, { clipboard: { writeText: jest.fn().mockResolvedValue(undefined) } })
  })

  it('renders every code', () => {
    render(<RecoveryCodesDialog codes={['aaaa-bbbb', 'cccc-dddd']} onDismiss={jest.fn()} />)
    expect(screen.getByText('aaaa-bbbb')).toBeInTheDocument()
    expect(screen.getByText('cccc-dddd')).toBeInTheDocument()
  })

  it('copies the codes to the clipboard, joined by newlines', async () => {
    render(<RecoveryCodesDialog codes={['aaaa-bbbb', 'cccc-dddd']} onDismiss={jest.fn()} />)

    fireEvent.click(screen.getByRole('button', { name: 'Copy codes' }))

    await waitFor(() => {
      expect(navigator.clipboard.writeText).toHaveBeenCalledWith('aaaa-bbbb\ncccc-dddd')
      expect(screen.getByRole('button', { name: 'Copied!' })).toBeInTheDocument()
    })
  })

  it('keeps Done disabled until the confirmation checkbox is checked', () => {
    const onDismiss = jest.fn()
    render(<RecoveryCodesDialog codes={['aaaa-bbbb']} onDismiss={onDismiss} />)

    const doneButton = screen.getByRole('button', { name: 'Done' })
    expect(doneButton).toBeDisabled()

    fireEvent.click(screen.getByLabelText("I've saved these recovery codes somewhere safe."))
    expect(doneButton).toBeEnabled()

    fireEvent.click(doneButton)
    expect(onDismiss).toHaveBeenCalled()
  })
})
