'use client'

import {
  useState,
  useRef,
  useEffect,
  useCallback,
  type ReactNode,
  type ButtonHTMLAttributes
} from 'react'
import { cn } from '@/lib/cn'

interface PopoverProps {
  trigger: (props: { open: boolean; onClick: () => void }) => ReactNode
  children: ReactNode
  /** Extra classes applied to the panel wrapper. */
  className?: string
  /** Alignment of the panel relative to the trigger. Defaults to "right". */
  align?: 'left' | 'right'
}

/**
 * A lightweight popover primitive: a trigger + an absolutely-positioned panel
 * that closes on outside-click and Escape. No Radix dependency.
 */
export function Popover({ trigger, children, className, align = 'right' }: PopoverProps) {
  const [open, setOpen] = useState(false)
  const containerRef = useRef<HTMLDivElement>(null)

  const close = useCallback(() => setOpen(false), [])

  // Close on outside click
  useEffect(() => {
    if (!open) return
    const handler = (e: MouseEvent) => {
      if (
        containerRef.current &&
        !containerRef.current.contains(e.target instanceof Node ? e.target : null)
      ) {
        close()
      }
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
    <div ref={containerRef} className="relative">
      {trigger({ open, onClick: () => setOpen((v) => !v) })}
      {open && (
        <div
          className={cn(
            'absolute z-50 mt-1 min-w-55 rounded-2xl border border-border bg-card shadow-elevated p-3',
            align === 'right' ? 'right-0' : 'left-0',
            className
          )}
          role="dialog"
        >
          {children}
        </div>
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
