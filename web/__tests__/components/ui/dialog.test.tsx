import { render, screen, fireEvent } from '@testing-library/react'
import { ConfirmDialog, DialogFooter } from '@/components/ui/dialog'

describe('DialogFooter', () => {
  it('renders its children in a right-aligned row', () => {
    const { container } = render(
      <DialogFooter>
        <span>action</span>
      </DialogFooter>
    )
    expect(screen.getByText('action')).toBeInTheDocument()
    expect(container.firstChild).toHaveClass('justify-end')
  })

  it('merges a className override', () => {
    const { container } = render(
      <DialogFooter className="justify-between">
        <span>action</span>
      </DialogFooter>
    )
    expect(container.firstChild).toHaveClass('justify-between')
  })
})

describe('ConfirmDialog', () => {
  const baseProps = {
    open: true,
    onOpenChange: jest.fn(),
    title: 'Remove book',
    onConfirm: jest.fn()
  }

  beforeEach(() => {
    jest.clearAllMocks()
  })

  it('renders nothing while closed', () => {
    render(<ConfirmDialog {...baseProps} open={false} />)
    expect(screen.queryByText('Remove book')).not.toBeInTheDocument()
  })

  it('renders its title and description', () => {
    render(<ConfirmDialog {...baseProps} description="This cannot be undone." />)
    expect(screen.getByText('Remove book')).toBeInTheDocument()
    expect(screen.getByText('This cannot be undone.')).toBeInTheDocument()
  })

  it('uses the default confirm and cancel labels', () => {
    render(<ConfirmDialog {...baseProps} />)
    expect(screen.getByRole('button', { name: 'Confirm' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument()
  })

  it('calls onConfirm when the confirm action is clicked', () => {
    render(<ConfirmDialog {...baseProps} confirmLabel="Remove" />)
    fireEvent.click(screen.getByRole('button', { name: 'Remove' }))
    expect(baseProps.onConfirm).toHaveBeenCalledTimes(1)
  })

  it('closes via onOpenChange when cancelled', () => {
    render(<ConfirmDialog {...baseProps} />)
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(baseProps.onOpenChange).toHaveBeenCalledWith(false)
  })

  it('shows the pending label and disables both actions while pending', () => {
    render(<ConfirmDialog {...baseProps} confirmLabel="Remove" pendingLabel="Removing…" pending />)
    expect(screen.getByRole('button', { name: 'Removing…' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeDisabled()
  })

  it('keeps the confirm label while pending when no pendingLabel is given', () => {
    render(<ConfirmDialog {...baseProps} confirmLabel="Remove" pending />)
    expect(screen.getByRole('button', { name: 'Remove' })).toBeDisabled()
  })

  it('disables only the confirm action when confirmDisabled is set', () => {
    render(<ConfirmDialog {...baseProps} confirmLabel="Remove" confirmDisabled />)
    expect(screen.getByRole('button', { name: 'Remove' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeEnabled()
  })

  it('styles the confirm action as destructive when asked', () => {
    render(<ConfirmDialog {...baseProps} confirmLabel="Remove" destructive />)
    expect(screen.getByRole('button', { name: 'Remove' })).toHaveClass('bg-danger')
  })

  it('renders extra children between the description and the actions', () => {
    render(
      <ConfirmDialog {...baseProps}>
        <p>extra detail</p>
      </ConfirmDialog>
    )
    expect(screen.getByText('extra detail')).toBeInTheDocument()
  })
})
