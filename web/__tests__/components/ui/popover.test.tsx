import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { Popover, PopoverTrigger } from '@/components/ui/popover'

function BasicPopover({ align }: { align?: 'left' | 'right' }) {
  return (
    <Popover
      align={align}
      trigger={({ open, onClick }) => (
        <PopoverTrigger onClick={onClick} aria-expanded={open} aria-label="Open menu">
          Open
        </PopoverTrigger>
      )}
    >
      <p>Panel content</p>
    </Popover>
  )
}

describe('Popover', () => {
  it('renders the trigger', () => {
    render(<BasicPopover />)
    expect(screen.getByLabelText('Open menu')).toBeInTheDocument()
  })

  it('does not show panel initially', () => {
    render(<BasicPopover />)
    expect(screen.queryByText('Panel content')).not.toBeInTheDocument()
  })

  it('opens panel on trigger click', () => {
    render(<BasicPopover />)
    fireEvent.click(screen.getByLabelText('Open menu'))
    expect(screen.getByText('Panel content')).toBeInTheDocument()
  })

  it('closes panel on second click (toggle)', () => {
    render(<BasicPopover />)
    fireEvent.click(screen.getByLabelText('Open menu'))
    fireEvent.click(screen.getByLabelText('Open menu'))
    expect(screen.queryByText('Panel content')).not.toBeInTheDocument()
  })

  it('closes panel on Escape', () => {
    render(<BasicPopover />)
    fireEvent.click(screen.getByLabelText('Open menu'))
    expect(screen.getByText('Panel content')).toBeInTheDocument()

    fireEvent.keyDown(document, { key: 'Escape' })
    expect(screen.queryByText('Panel content')).not.toBeInTheDocument()
  })

  it('closes on outside click', () => {
    render(
      <div>
        <BasicPopover />
        <button>Outside</button>
      </div>
    )
    fireEvent.click(screen.getByLabelText('Open menu'))
    expect(screen.getByText('Panel content')).toBeInTheDocument()

    fireEvent.mouseDown(screen.getByRole('button', { name: 'Outside' }))
    expect(screen.queryByText('Panel content')).not.toBeInTheDocument()
  })

  it('sets aria-expanded on trigger when open', () => {
    render(<BasicPopover />)
    const trigger = screen.getByLabelText('Open menu')
    expect(trigger).toHaveAttribute('aria-expanded', 'false')
    fireEvent.click(trigger)
    expect(trigger).toHaveAttribute('aria-expanded', 'true')
  })

  it('panel has role=dialog', () => {
    render(<BasicPopover />)
    fireEvent.click(screen.getByLabelText('Open menu'))
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('panel is portaled to document.body (not inside the trigger container)', () => {
    const { container } = render(<BasicPopover />)
    fireEvent.click(screen.getByLabelText('Open menu'))
    const panel = screen.getByRole('dialog')
    // The panel must be a direct child of document.body, not nested inside
    // the component's container div — this confirms it escaped overflow clipping.
    expect(document.body).toContainElement(panel)
    expect(container).not.toContainElement(panel)
  })
})
