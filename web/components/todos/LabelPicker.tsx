'use client'

import { useState } from 'react'
import { filterLabels } from '@/lib/todos/labelFilter'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'

interface LabelPickerProps {
  value: string[]
  onChange: (labels: string[]) => void
  presets: string[]
  placeholder?: string
}

export function LabelPicker({
  value,
  onChange,
  presets,
  placeholder = 'Search labels…'
}: LabelPickerProps) {
  const [query, setQuery] = useState('')
  const [open, setOpen] = useState(false)

  const filtered = filterLabels(presets, query)

  function toggleLabel(label: string) {
    if (value.includes(label)) {
      onChange(value.filter((l) => l !== label))
    } else {
      onChange([...value, label])
    }
  }

  return (
    <div className="relative">
      <Input
        type="text"
        value={query}
        onChange={(e) => {
          setQuery(e.target.value)
          setOpen(true)
        }}
        onFocus={() => setOpen(true)}
        onBlur={() => setTimeout(() => setOpen(false), 150)}
        placeholder={placeholder}
        aria-label="Label search"
      />
      {open && filtered.length > 0 && (
        <ul
          role="listbox"
          className="absolute z-10 mt-1 w-full rounded-2xl border border-border bg-card shadow-elevated"
        >
          {filtered.map((label) => (
            <li key={label} role="option" aria-selected={value.includes(label)}>
              <label className="flex cursor-pointer items-center gap-2 px-3 py-2 text-sm hover:bg-hover">
                <input
                  type="checkbox"
                  checked={value.includes(label)}
                  onChange={() => toggleLabel(label)}
                  className="accent-accent"
                />
                {label}
              </label>
            </li>
          ))}
        </ul>
      )}
      {value.length > 0 && (
        <div className="mt-1.5 flex flex-wrap gap-1">
          {value.map((label) => (
            <span
              key={label}
              className="inline-flex items-center gap-1 rounded-full border border-accent/20 bg-accent/10 px-2 py-0.5 text-xs text-accent"
            >
              {label}
              <Button
                type="button"
                variant="ghost"
                size="iconSm"
                onClick={() => toggleLabel(label)}
                aria-label={`Remove ${label}`}
                className="h-4 w-4 rounded-sm text-accent hover:bg-transparent hover:text-accent-hover focus-visible:ring-1 focus-visible:ring-accent"
              >
                ×
              </Button>
            </span>
          ))}
        </div>
      )}
    </div>
  )
}
