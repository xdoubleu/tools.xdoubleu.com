'use client'

import { useState, type ReactNode } from 'react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/cn'

interface CollapsibleProps {
  title: ReactNode
  defaultCollapsed?: boolean
  children: ReactNode
  className?: string
  /** Applied to the trigger, e.g. to change its type scale. */
  triggerClassName?: string
}

/**
 * Disclosure section with a chevron trigger. Keeps its own open state — lift it
 * out only if something outside needs to drive it.
 */
function Collapsible({
  title,
  defaultCollapsed = true,
  children,
  className,
  triggerClassName
}: CollapsibleProps) {
  const [collapsed, setCollapsed] = useState(defaultCollapsed)

  return (
    <div className={cn('space-y-2', className)}>
      <Button
        variant="secondary"
        onClick={() => setCollapsed((prev) => !prev)}
        aria-expanded={!collapsed}
        className={cn(
          'h-auto w-full justify-start gap-2 rounded-2xl px-4 py-3 text-left text-lg font-semibold',
          triggerClassName
        )}
      >
        <span aria-hidden className="text-muted">
          {collapsed ? '▸' : '▾'}
        </span>
        {title}
      </Button>
      {!collapsed && children}
    </div>
  )
}

export { Collapsible }
export type { CollapsibleProps }
