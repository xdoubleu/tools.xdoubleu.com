import {
  useState,
  useRef,
  useEffect,
  useCallback,
  useId,
  type ReactNode,
  type ButtonHTMLAttributes,
  type KeyboardEvent
} from 'react'
import { createPortal } from 'react-dom'
import { cn } from '@/lib/cn'

interface PopoverProps {
  trigger: (props: { open: boolean; onClick: () => void }) => ReactNode
  children: ReactNode
  className?: string
  /** Alignment of the panel relative to the trigger. Defaults to "right". */
  align?: 'left' | 'right'
  /** Controls the open state, e.g. to close the panel after a menu action. */
  open?: boolean
  onOpenChange?: (open: boolean) => void
  /** Names the panel for screen readers; without it the trigger labels the panel. */
  'aria-label'?: string
}

interface PanelCoords {
  /** Top anchor (opens downward). Mutually exclusive with bottom. */
  top?: number
  /** Bottom anchor (opens upward). Mutually exclusive with top. */
  bottom?: number
  left?: number
  right?: number
  /** Available height for the panel in the chosen direction. */
  maxHeight: number
}

const MARGIN = 8 // px clearance from viewport edges
const FOCUSABLE =
  'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
const MENU_ITEM = '[role="menuitem"]:not([disabled])'

const focusables = (root: HTMLElement | null, selector = FOCUSABLE) =>
  root ? Array.from(root.querySelectorAll<HTMLElement>(selector)) : []

/**
 * Trigger plus a portalled fixed panel (never clipped by overflow ancestors)
 * that closes on outside click and Escape, flips upward when space below is
 * short, and caps its height to the viewport. Opening moves focus into the
 * panel (its first `menuitem` for a menu, which arrow keys then move between);
 * closing, or tabbing out, returns focus to the trigger.
 */
export function Popover({
  trigger,
  children,
  className,
  align = 'right',
  open: controlledOpen,
  onOpenChange,
  'aria-label': ariaLabel
}: PopoverProps) {
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false)
  const open = controlledOpen ?? uncontrolledOpen
  const setOpen = useCallback(
    (next: boolean) => {
      if (controlledOpen === undefined) setUncontrolledOpen(next)
      onOpenChange?.(next)
    },
    [controlledOpen, onOpenChange]
  )
  const [coords, setCoords] = useState<PanelCoords>({ top: 0, maxHeight: 400 })
  const triggerRef = useRef<HTMLDivElement>(null)
  const panelRef = useRef<HTMLDivElement>(null)
  const fallbackTriggerId = useId()
  const [triggerId, setTriggerId] = useState('')

  const close = useCallback(() => setOpen(false), [setOpen])

  const computeCoords = useCallback(() => {
    if (!triggerRef.current) return
    const rect = triggerRef.current.getBoundingClientRect()
    const vh = window.innerHeight

    const spaceBelow = vh - rect.bottom - MARGIN
    const spaceAbove = rect.top - MARGIN

    const c: PanelCoords =
      spaceAbove > spaceBelow && spaceBelow < 200
        ? // Flip upward — anchor to bottom edge of trigger
          { bottom: vh - rect.top + 4, maxHeight: Math.max(spaceAbove, 100) }
        : // Default — open downward
          { top: rect.bottom + 4, maxHeight: Math.max(spaceBelow, 100) }

    // Keep the panel's anchored edge on-screen when the trigger is scrolled
    // partly out of view (e.g. inside a horizontally scrolling table).
    const vw = window.innerWidth
    if (align === 'right') {
      c.right = Math.min(Math.max(vw - rect.right, MARGIN), vw - MARGIN)
    } else {
      c.left = Math.min(Math.max(rect.left, MARGIN), vw - MARGIN)
    }
    setCoords(c)
  }, [align])

  useEffect(() => {
    if (open) computeCoords()
  }, [open, computeCoords])

  useEffect(() => {
    if (!open) return
    const trigger = focusables(triggerRef.current)[0]
    const panel = panelRef.current
    if (trigger && !trigger.id) trigger.id = fallbackTriggerId
    setTriggerId(trigger?.id ?? '')
    ;(focusables(panel, MENU_ITEM)[0] ?? panel)?.focus({ preventScroll: true })
    return () => {
      // Only reclaim focus that was in the panel (now gone), never a control clicked outside.
      const active = document.activeElement
      if (!active || active === document.body || panel?.contains(active)) {
        trigger?.focus({ preventScroll: true })
      }
    }
  }, [open, fallbackTriggerId])

  const handlePanelKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const panel = panelRef.current
    const items = focusables(panel, MENU_ITEM)
    if (e.key === 'Tab') {
      const controls = focusables(panel)
      const active = document.activeElement
      const edge = e.shiftKey ? controls[0] : controls[controls.length - 1]
      const leaving = !edge || active === edge || (e.shiftKey && active === panel)
      // A menu closes on Tab; a dialog only once focus would leave it.
      if (items.length > 0 || leaving) {
        e.preventDefault()
        close()
      }
      return
    }
    if (items.length === 0) return
    const i = items.findIndex((item) => item === document.activeElement)
    const next =
      e.key === 'ArrowDown'
        ? (i + 1) % items.length
        : e.key === 'ArrowUp'
          ? (i - 1 + items.length) % items.length
          : e.key === 'Home'
            ? 0
            : e.key === 'End'
              ? items.length - 1
              : -1
    if (next < 0) return
    e.preventDefault()
    items[next].focus()
  }

  useEffect(() => {
    if (!open) return
    window.addEventListener('scroll', computeCoords, true)
    window.addEventListener('resize', computeCoords)
    return () => {
      window.removeEventListener('scroll', computeCoords, true)
      window.removeEventListener('resize', computeCoords)
    }
  }, [open, computeCoords])

  // Outside click must exclude both trigger and panel.
  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      const target = e.target instanceof Node ? e.target : null
      const inTrigger = triggerRef.current?.contains(target) ?? false
      const inPanel = panelRef.current?.contains(target) ?? false
      if (!inTrigger && !inPanel) close()
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [open, close])

  useEffect(() => {
    if (!open) return
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') close()
    }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [open, close])

  return (
    <div ref={triggerRef} className="relative">
      {trigger({ open, onClick: () => setOpen(!open) })}
      {open &&
        createPortal(
          <div
            ref={panelRef}
            style={{
              position: 'fixed',
              ...(coords.top !== undefined ? { top: coords.top } : { bottom: coords.bottom }),
              ...(coords.right !== undefined ? { right: coords.right } : { left: coords.left }),
              maxHeight: coords.maxHeight
            }}
            className={cn(
              'z-50 min-w-55 max-w-[calc(100vw-1rem)] rounded-2xl border border-border bg-card shadow-elevated p-3',
              'overflow-y-auto outline-none',
              className
            )}
            role="dialog"
            aria-label={ariaLabel}
            aria-labelledby={ariaLabel || !triggerId ? undefined : triggerId}
            tabIndex={-1}
            onKeyDown={handlePanelKeyDown}
          >
            {children}
          </div>,
          document.body
        )}
    </div>
  )
}

/** A plain button styled for use as a popover trigger. */
export function PopoverTrigger({ className, ...props }: ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      type="button"
      className={cn(
        'flex min-h-11 items-center justify-center rounded-lg px-2 py-1 text-sm text-subtle sm:min-h-0',
        'transition-colors hover:bg-hover hover:text-fg active:bg-hover active:text-fg',
        'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent',
        className
      )}
      {...props}
    />
  )
}
