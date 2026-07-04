'use client'

import { Button } from '@/components/ui/button'

const SHORTCUTS = [
  { key: '/', desc: 'new task' },
  { key: '↑↓', desc: 'navigate tasks' },
  { key: 's', desc: 'jump to sections' },
  { key: 'Esc / →', desc: 'exit sections' }
]

interface ShortcutHintsProps {
  onDismiss: () => void
}

export default function ShortcutHints({ onDismiss }: ShortcutHintsProps) {
  return (
    <div className="flex items-center gap-4 rounded-lg border border-border bg-surface px-3 py-1.5 text-xs text-muted">
      {SHORTCUTS.map(({ key, desc }) => (
        <span key={key}>
          <kbd className="rounded-lg border border-border bg-card px-1 py-0.5 font-mono text-xs">
            {key}
          </kbd>{' '}
          {desc}
        </span>
      ))}
      <Button
        type="button"
        variant="ghost"
        size="iconSm"
        onClick={onDismiss}
        className="ml-auto h-5 w-5 text-muted hover:bg-transparent hover:text-subtle"
        aria-label="Dismiss shortcut hints"
      >
        ✕
      </Button>
    </div>
  )
}
