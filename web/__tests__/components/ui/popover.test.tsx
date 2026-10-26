import React from 'react'
import { render, screen, fireEvent, createEvent } from '@testing-library/react'
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
    // A direct child of body, so it escapes overflow clipping.
    expect(document.body).toContainElement(panel)
    expect(container).not.toContainElement(panel)
  })

  it('panel has a maxHeight style applied to cap its height', () => {
    render(<BasicPopover />)
    fireEvent.click(screen.getByLabelText('Open menu'))
    const panel = screen.getByRole('dialog')
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
    // Little space below, plenty above.
    const spy = vi.spyOn(Element.prototype, 'getBoundingClientRect').mockReturnValue(
      new DOMRect(100, 400, 100, 90) // x, y, width, height → top=400, bottom=490
    )
    Object.defineProperty(window, 'innerHeight', { value: 500, writable: true, configurable: true })

    render(<BasicPopover />)
    fireEvent.click(screen.getByLabelText('Open menu'))
    const panel = screen.getByRole('dialog')

    expect(panel.style.bottom).toBeTruthy()
    expect(panel.style.top).toBe('')

    spy.mockRestore()
    Object.defineProperty(window, 'innerHeight', { value: 768, writable: true, configurable: true })
  })

  it('opens downward (uses top style) when there is enough space below', () => {
    const spy = vi.spyOn(Element.prototype, 'getBoundingClientRect').mockReturnValue(
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

  it('follows a controlled open prop and reports toggles through onOpenChange', () => {
    const onOpenChange = vi.fn()
    const trigger = ({ open, onClick }: { open: boolean; onClick: () => void }) => (
      <PopoverTrigger onClick={onClick} aria-expanded={open} aria-label="Open menu">
        Open
      </PopoverTrigger>
    )
    const { rerender } = render(
      <Popover open={false} onOpenChange={onOpenChange} trigger={trigger}>
        <p>Panel content</p>
      </Popover>
    )
    fireEvent.click(screen.getByLabelText('Open menu'))
    expect(onOpenChange).toHaveBeenCalledWith(true)
    expect(screen.queryByText('Panel content')).not.toBeInTheDocument()

    rerender(
      <Popover open onOpenChange={onOpenChange} trigger={trigger}>
        <p>Panel content</p>
      </Popover>
    )
    expect(screen.getByText('Panel content')).toBeInTheDocument()
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(onOpenChange).toHaveBeenLastCalledWith(false)
  })

  it('moves focus into the panel and names it after the trigger', () => {
    render(<BasicPopover />)
    fireEvent.click(screen.getByLabelText('Open menu'))
    expect(screen.getByRole('dialog', { name: 'Open menu' })).toHaveFocus()
  })

  it('leaves focus on a control in the panel that took it on mount', () => {
    render(
      <Popover trigger={({ onClick }) => <PopoverTrigger onClick={onClick}>Tags</PopoverTrigger>}>
        <input aria-label="New tag" autoFocus />
      </Popover>
    )
    fireEvent.click(screen.getByRole('button', { name: 'Tags' }))
    expect(screen.getByRole('textbox', { name: 'New tag' })).toHaveFocus()
  })

  it('takes its accessible name from aria-label when given', () => {
    render(
      <Popover
        aria-label="Book filters"
        trigger={({ onClick }) => <PopoverTrigger onClick={onClick}>Filters</PopoverTrigger>}
      >
        <p>Panel content</p>
      </Popover>
    )
    fireEvent.click(screen.getByRole('button', { name: 'Filters' }))
    expect(screen.getByRole('dialog', { name: 'Book filters' })).toBeInTheDocument()
  })

  it('returns focus to the trigger when Escape closes it', () => {
    render(<BasicPopover />)
    const trigger = screen.getByLabelText('Open menu')
    fireEvent.click(trigger)
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(trigger).toHaveFocus()
  })

  it('closes and returns focus to the trigger when Tab leaves the panel', () => {
    render(
      <Popover
        trigger={({ onClick }) => <PopoverTrigger onClick={onClick}>Columns</PopoverTrigger>}
      >
        <button>First</button>
        <button>Last</button>
      </Popover>
    )
    const trigger = screen.getByRole('button', { name: 'Columns' })
    fireEvent.click(trigger)
    const last = screen.getByRole('button', { name: 'Last' })
    last.focus()
    fireEvent.keyDown(last, { key: 'Tab' })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(trigger).toHaveFocus()
  })

  it('lets Tab move between controls inside the panel', () => {
    render(
      <Popover
        trigger={({ onClick }) => <PopoverTrigger onClick={onClick}>Columns</PopoverTrigger>}
      >
        <button>First</button>
        <button>Last</button>
      </Popover>
    )
    fireEvent.click(screen.getByRole('button', { name: 'Columns' }))
    const first = screen.getByRole('button', { name: 'First' })
    first.focus()
    const tab = createEvent.keyDown(first, { key: 'Tab' })
    fireEvent(first, tab)
    expect(tab.defaultPrevented).toBe(false)
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('keeps an existing trigger id when labelling the panel by it', () => {
    render(
      <Popover
        trigger={({ onClick }) => (
          <PopoverTrigger id="columns-trigger" onClick={onClick}>
            Columns
          </PopoverTrigger>
        )}
      >
        <p>Panel content</p>
      </Popover>
    )
    fireEvent.click(screen.getByRole('button', { name: 'Columns' }))
    expect(screen.getByRole('dialog', { name: 'Columns' })).toHaveAttribute(
      'aria-labelledby',
      'columns-trigger'
    )
  })

  it('closes on Shift+Tab from the panel or its first control', () => {
    render(
      <Popover
        trigger={({ onClick }) => <PopoverTrigger onClick={onClick}>Columns</PopoverTrigger>}
      >
        <button>First</button>
      </Popover>
    )
    const trigger = screen.getByRole('button', { name: 'Columns' })
    fireEvent.click(trigger)
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Tab', shiftKey: true })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(trigger).toHaveFocus()

    fireEvent.click(trigger)
    const first = screen.getByRole('button', { name: 'First' })
    first.focus()
    fireEvent.keyDown(first, { key: 'Tab', shiftKey: true })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('leaves arrow keys alone in a panel without menu items', () => {
    render(<BasicPopover />)
    fireEvent.click(screen.getByLabelText('Open menu'))
    const arrow = createEvent.keyDown(screen.getByRole('dialog'), { key: 'ArrowDown' })
    fireEvent(screen.getByRole('dialog'), arrow)
    expect(arrow.defaultPrevented).toBe(false)
    expect(screen.getByRole('dialog')).toHaveFocus()
  })

  describe('with a menu', () => {
    function MenuPopover() {
      return (
        <Popover
          trigger={({ open, onClick }) => (
            <PopoverTrigger onClick={onClick} aria-haspopup="menu" aria-expanded={open}>
              Actions
            </PopoverTrigger>
          )}
        >
          <div role="menu">
            <button role="menuitem">Swap</button>
            <button role="menuitem">Edit</button>
            <button role="menuitem">Delete</button>
          </div>
        </Popover>
      )
    }

    it('focuses the first item and moves through items with the arrow keys', () => {
      render(<MenuPopover />)
      fireEvent.click(screen.getByRole('button', { name: 'Actions' }))
      const item = (name: string) => screen.getByRole('menuitem', { name })
      expect(item('Swap')).toHaveFocus()
      fireEvent.keyDown(item('Swap'), { key: 'ArrowDown' })
      expect(item('Edit')).toHaveFocus()
      fireEvent.keyDown(item('Edit'), { key: 'End' })
      expect(item('Delete')).toHaveFocus()
      fireEvent.keyDown(item('Delete'), { key: 'ArrowDown' })
      expect(item('Swap')).toHaveFocus()
      fireEvent.keyDown(item('Swap'), { key: 'ArrowUp' })
      expect(item('Delete')).toHaveFocus()
      fireEvent.keyDown(item('Delete'), { key: 'Home' })
      expect(item('Swap')).toHaveFocus()
    })

    it('ignores keys other than the arrows, Home and End', () => {
      render(<MenuPopover />)
      fireEvent.click(screen.getByRole('button', { name: 'Actions' }))
      const swap = screen.getByRole('menuitem', { name: 'Swap' })
      const key = createEvent.keyDown(swap, { key: 'x' })
      fireEvent(swap, key)
      expect(key.defaultPrevented).toBe(false)
      expect(swap).toHaveFocus()
    })

    it('closes on Tab from any item, returning focus to the trigger', () => {
      render(<MenuPopover />)
      const trigger = screen.getByRole('button', { name: 'Actions' })
      fireEvent.click(trigger)
      fireEvent.keyDown(screen.getByRole('menuitem', { name: 'Swap' }), { key: 'Tab' })
      expect(screen.queryByRole('menu')).not.toBeInTheDocument()
      expect(trigger).toHaveFocus()
    })
  })
})
