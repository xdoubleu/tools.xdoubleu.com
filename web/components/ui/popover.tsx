'use client'

import {
  useState,
  useRef,
  useEffect,
  useCallback,
  type ReactNode,
  type ButtonHTMLAttributes
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

/**
 * Trigger plus a portalled fixed panel (never clipped by overflow ancestors)
 * that closes on outside click and Escape, flips upward when space below is
 * short, and caps its height to the viewport.
 */
export function Popover({
  trigger,
  children,
  className,
  align = 'right',
  open: controlledOpen,
  onOpenChange
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
              'overflow-y-auto',
              className
            )}
            role="dialog"
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
