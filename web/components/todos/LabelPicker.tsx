'use client'

import { useState } from 'react'
import { filterLabels } from '@/lib/todos/labelFilter'

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
      <input
        type="text"
        value={query}
        onChange={(e) => {
          setQuery(e.target.value)
          setOpen(true)
        }}
        onFocus={() => setOpen(true)}
        onBlur={() => setTimeout(() => setOpen(false), 150)}
        placeholder={placeholder}
        className="w-full rounded border border-gray-300 px-3 py-1.5 text-sm"
        aria-label="Label search"
      />
      {open && filtered.length > 0 && (
        <ul
          role="listbox"
          className="absolute z-10 mt-1 w-full rounded border border-gray-200 bg-white shadow-sm"
        >
          {filtered.map((label) => (
            <li key={label} role="option" aria-selected={value.includes(label)}>
              <label className="flex cursor-pointer items-center gap-2 px-3 py-1.5 text-sm hover:bg-gray-50">
                <input
                  type="checkbox"
                  checked={value.includes(label)}
                  onChange={() => toggleLabel(label)}
                  className="accent-blue-600"
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
              className="inline-flex items-center gap-1 rounded bg-blue-100 px-2 py-0.5 text-xs text-blue-800"
            >
              {label}
              <button
                type="button"
                onClick={() => toggleLabel(label)}
                aria-label={`Remove ${label}`}
                className="hover:text-blue-600"
              >
                ×
              </button>
            </span>
          ))}
        </div>
      )}
    </div>
  )
}
