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

  it('panel has a maxHeight style applied to cap its height', () => {
    render(<BasicPopover />)
    fireEvent.click(screen.getByLabelText('Open menu'))
    const panel = screen.getByRole('dialog')
    // maxHeight must be set to a positive number to prevent off-screen overflow
    const maxH = panel.style.maxHeight
    expect(maxH).toBeTruthy()
    expect(parseFloat(maxH)).toBeGreaterThan(0)
  })

  it('panel has overflow-y-auto class to scroll within its bounded height', () => {
    render(<BasicPopover />)
    fireEvent.click(screen.getByLabelText('Open menu'))
    expect(screen.getByRole('dialog')).toHaveClass('overflow-y-auto')
  })

  it('flips upward (uses bottom style) when trigger is near the viewport bottom', () => {
    // Position trigger so there is very little space below (~10px) but plenty above (~400px)
    const spy = jest.spyOn(Element.prototype, 'getBoundingClientRect').mockReturnValue(
      new DOMRect(100, 400, 100, 90) // x, y, width, height → top=400, bottom=490
    )
    Object.defineProperty(window, 'innerHeight', { value: 500, writable: true, configurable: true })

    render(<BasicPopover />)
    fireEvent.click(screen.getByLabelText('Open menu'))
    const panel = screen.getByRole('dialog')

    // When flipped up, `bottom` is set and `top` is absent
    expect(panel.style.bottom).toBeTruthy()
    expect(panel.style.top).toBe('')

    spy.mockRestore()
    Object.defineProperty(window, 'innerHeight', { value: 768, writable: true, configurable: true })
  })

  it('opens downward (uses top style) when there is enough space below', () => {
    const spy = jest.spyOn(Element.prototype, 'getBoundingClientRect').mockReturnValue(
      new DOMRect(100, 50, 100, 30) // x, y, width, height → top=50, bottom=80
    )
    Object.defineProperty(window, 'innerHeight', { value: 768, writable: true, configurable: true })

    render(<BasicPopover />)
    fireEvent.click(screen.getByLabelText('Open menu'))
    const panel = screen.getByRole('dialog')

    expect(panel.style.top).toBeTruthy()
    expect(panel.style.bottom).toBe('')

    spy.mockRestore()
  })
})
