import { act, createEvent, render, screen, fireEvent } from '@testing-library/react'
import {
  ConfirmDialog,
  Dialog,
  DialogClose,
  DialogContent,
  DialogFooter,
  DialogTitle
} from '@/components/ui/dialog'

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
    onOpenChange: vi.fn(),
    title: 'Remove book',
    onConfirm: vi.fn()
  }

  beforeEach(() => {
    vi.clearAllMocks()
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

describe('DialogClose', () => {
  it('defaults to a labelled × with a 44px target', () => {
    render(
      <Dialog open onOpenChange={vi.fn()}>
        <DialogContent side="sheet">
          <DialogTitle>Sheet</DialogTitle>
          <DialogClose />
        </DialogContent>
      </Dialog>
    )
    const close = screen.getByRole('button', { name: 'Close' })
    expect(close).toHaveTextContent('×')
    expect(close).toHaveClass('size-11', 'rounded-xl')
    expect(close).not.toHaveClass('rounded-full')
  })
})

describe('DialogContent sheet', () => {
  afterEach(() => {
    Reflect.deleteProperty(window, 'visualViewport')
  })

  it('rides above the on-screen keyboard', () => {
    const viewport = Object.assign(new EventTarget(), { height: window.innerHeight, offsetTop: 0 })
    Object.defineProperty(window, 'visualViewport', { value: viewport, configurable: true })
    render(
      <Dialog open onOpenChange={vi.fn()}>
        <DialogContent side="sheet">
          <DialogTitle>Sheet</DialogTitle>
        </DialogContent>
      </Dialog>
    )
    const sheet = screen.getByRole('dialog')
    expect(sheet.style.getPropertyValue('--keyboard-inset')).toBe('0px')
    expect(sheet).toHaveClass('bottom-(--keyboard-inset)')

    act(() => {
      viewport.height = window.innerHeight - 320
      viewport.dispatchEvent(new Event('resize'))
    })
    expect(sheet.style.getPropertyValue('--keyboard-inset')).toBe('320px')
  })

  it('leaves other sides without a keyboard inset', () => {
    render(
      <Dialog open onOpenChange={vi.fn()}>
        <DialogContent side="center">
          <DialogTitle>Centered</DialogTitle>
        </DialogContent>
      </Dialog>
    )
    expect(screen.getByRole('dialog').style.getPropertyValue('--keyboard-inset')).toBe('')
  })

  it('is the default and renders a drag handle', () => {
    const { baseElement } = render(
      <Dialog open onOpenChange={vi.fn()}>
        <DialogContent>
          <DialogTitle>Default</DialogTitle>
        </DialogContent>
      </Dialog>
    )
    expect(screen.getByRole('dialog')).toHaveAttribute('data-sheet')
    expect(baseElement.querySelector('[aria-hidden="true"].touch-none')).not.toBeNull()
  })

  it('drags down and closes on a fling, snaps back on a short pull', () => {
    const onOpenChange = vi.fn()
    const { baseElement } = render(
      <Dialog open onOpenChange={onOpenChange}>
        <DialogContent>
          <DialogTitle>Drag</DialogTitle>
        </DialogContent>
      </Dialog>
    )
    const handle = baseElement.querySelector<HTMLElement>('.touch-none')!
    handle.setPointerCapture = vi.fn()
    const sheet = screen.getByRole('dialog')
    Object.defineProperty(sheet, 'offsetHeight', { value: 500 })

    // Velocity derives from event timestamps, so pin them instead of using wall-clock time.
    // React treats a 0 timeStamp as missing and substitutes Date.now(), so stay above 0.
    const pointer = (
      type: 'pointerDown' | 'pointerMove' | 'pointerUp',
      clientY: number,
      timeStamp: number
    ) => {
      const event = createEvent[type](handle, { clientY, pointerId: 1 })
      Object.defineProperty(event, 'timeStamp', { value: timeStamp })
      fireEvent(handle, event)
    }

    pointer('pointerDown', 100, 1000)
    expect(handle.setPointerCapture).toHaveBeenCalledWith(1)
    expect(sheet.style.transition).toBe('none')
    pointer('pointerMove', 110, 1100)
    expect(sheet.style.transform).toBe('translateY(10px)')
    pointer('pointerUp', 110, 1100)
    expect(onOpenChange).not.toHaveBeenCalled()
    expect(sheet.style.transform).toBe('')
    expect(sheet.style.transition).toContain('--ease-spring')

    // 150px of 500 is under the pull threshold; 150px in 100ms is a fling.
    pointer('pointerDown', 100, 5000)
    pointer('pointerMove', 250, 5100)
    pointer('pointerUp', 250, 5100)
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it('ignores moves and releases that were not preceded by a press', () => {
    const onOpenChange = vi.fn()
    const { baseElement } = render(
      <Dialog open onOpenChange={onOpenChange}>
        <DialogContent>
          <DialogTitle>Idle</DialogTitle>
        </DialogContent>
      </Dialog>
    )
    const handle = baseElement.querySelector<HTMLElement>('.touch-none')!
    const sheet = screen.getByRole('dialog')
    fireEvent.pointerMove(handle, { clientY: 300, pointerId: 1 })
    fireEvent.pointerUp(handle, { clientY: 300, pointerId: 1 })
    expect(sheet.style.transform).toBe('')
    expect(onOpenChange).not.toHaveBeenCalled()
  })

  it('never translates the sheet upward', () => {
    const { baseElement } = render(
      <Dialog open onOpenChange={vi.fn()}>
        <DialogContent>
          <DialogTitle>Up</DialogTitle>
        </DialogContent>
      </Dialog>
    )
    const handle = baseElement.querySelector<HTMLElement>('.touch-none')!
    handle.setPointerCapture = vi.fn()
    fireEvent.pointerDown(handle, { clientY: 200, pointerId: 1 })
    fireEvent.pointerMove(handle, { clientY: 150, pointerId: 1 })
    expect(screen.getByRole('dialog').style.transform).toBe('translateY(0px)')
  })

  it('styles each side', () => {
    const open = (side: 'sheet' | 'center' | 'right' | 'fullscreen') => {
      const { unmount } = render(
        <Dialog open onOpenChange={vi.fn()}>
          <DialogContent side={side}>
            <DialogTitle>{side}</DialogTitle>
          </DialogContent>
        </Dialog>
      )
      const el = screen.getByRole('dialog')
      return { el, unmount }
    }
    const sheet = open('sheet')
    expect(sheet.el).toHaveAttribute('data-sheet', '')
    expect(sheet.el).toHaveClass('bg-glass', 'border', 'shadow-elevated', 'overflow-y-auto')
    expect(sheet.el.className).toContain('animate-sheet-in')
    expect(sheet.el).not.toHaveClass('bg-card')
    sheet.unmount()
    for (const [side, animation] of [
      ['center', 'animate-pop-in'],
      ['right', 'animate-panel-in'],
      ['fullscreen', 'animate-pop-in']
    ] as const) {
      const { el, unmount } = open(side)
      expect(el).not.toHaveAttribute('data-sheet')
      expect(el).toHaveClass('bg-card', 'border', 'shadow-elevated')
      expect(el.className).toContain(animation)
      expect(el.className).toContain('motion-reduce:data-[state=open]:animate-fade-in')
      unmount()
    }
  })
})

describe('DialogContent right', () => {
  it('stays open on an outside pointer-down', async () => {
    const onOpenChange = vi.fn()
    render(
      <Dialog open onOpenChange={onOpenChange}>
        <DialogContent side="right">
          <DialogTitle>Panel</DialogTitle>
        </DialogContent>
      </Dialog>
    )
    await act(() => new Promise((resolve) => setTimeout(resolve, 0)))
    fireEvent.pointerDown(document.body)
    expect(screen.getByText('Panel')).toBeInTheDocument()
    expect(onOpenChange).not.toHaveBeenCalled()
  })
})
