'use client'

import { useState, type ReactNode } from 'react'

interface CollapsibleSectionProps {
  title: string
  defaultCollapsed?: boolean
  children: ReactNode
}

export default function CollapsibleSection({
  title,
  defaultCollapsed = true,
  children
}: CollapsibleSectionProps) {
  const [collapsed, setCollapsed] = useState(defaultCollapsed)

  return (
    <div className="space-y-2">
      <button
        type="button"
        onClick={() => setCollapsed((prev) => !prev)}
        aria-expanded={!collapsed}
        className="flex w-full items-center gap-2 rounded-2xl border border-border bg-surface px-4 py-3 text-left text-lg font-semibold text-fg"
      >
        <span aria-hidden className="text-muted">
          {collapsed ? '▸' : '▾'}
        </span>
        {title}
      </button>
      {!collapsed && children}
    </div>
  )
}
