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
  /** Extra classes applied to the panel wrapper. */
  className?: string
  /** Alignment of the panel relative to the trigger. Defaults to "right". */
  align?: 'left' | 'right'
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
 * A lightweight popover primitive: a trigger + a portaled fixed-position panel
 * that closes on outside-click and Escape. The panel is rendered via
 * createPortal to document.body so it is never clipped by an ancestor
 * overflow container (e.g. the library table's overflow-x-auto wrapper).
 *
 * The panel flips upward automatically when there is not enough space below
 * the trigger, and its height is capped to the available viewport space.
 */
export function Popover({ trigger, children, className, align = 'right' }: PopoverProps) {
  const [open, setOpen] = useState(false)
  const [coords, setCoords] = useState<PanelCoords>({ top: 0, maxHeight: 400 })
  const triggerRef = useRef<HTMLDivElement>(null)
  const panelRef = useRef<HTMLDivElement>(null)

  const close = useCallback(() => setOpen(false), [])

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

    if (align === 'right') {
      c.right = window.innerWidth - rect.right
    } else {
      c.left = rect.left
    }
    setCoords(c)
  }, [align])

  // Recompute on open
  useEffect(() => {
    if (open) computeCoords()
  }, [open, computeCoords])

  // Recompute on scroll/resize while open
  useEffect(() => {
    if (!open) return
    window.addEventListener('scroll', computeCoords, true)
    window.addEventListener('resize', computeCoords)
    return () => {
      window.removeEventListener('scroll', computeCoords, true)
      window.removeEventListener('resize', computeCoords)
    }
  }, [open, computeCoords])

  // Close on outside click — must exclude both trigger and panel
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

  // Close on Escape
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
      {trigger({ open, onClick: () => setOpen((v) => !v) })}
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
              'z-50 min-w-55 rounded-2xl border border-border bg-card shadow-elevated p-3',
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
        'flex items-center justify-center rounded-lg px-2 py-1 text-sm text-subtle',
        'transition-colors hover:bg-hover hover:text-fg',
        'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent',
        className
      )}
      {...props}
    />
  )
}
