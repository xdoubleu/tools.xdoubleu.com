'use client'

import { useState, useRef, useEffect } from 'react'
import { cn } from '@/lib/cn'
import { Input } from '@/components/ui/input'

interface ComboboxProps {
  value: string
  /** Called when the user types free text. */
  onChange: (value: string) => void
  /** Called when the user picks a suggestion (click, keyboard, or blur snap). */
  onSelect?: (value: string) => void
  suggestions: string[]
  placeholder?: string
  className?: string
  autoFocus?: boolean
  /** Called when Enter is pressed and no suggestion is highlighted. */
  onEnter?: () => void
  'aria-label'?: string
}

export function Combobox({
  value,
  onChange,
  onSelect,
  suggestions,
  placeholder,
  className,
  autoFocus,
  onEnter,
  'aria-label': ariaLabel
}: ComboboxProps) {
  const [open, setOpen] = useState(false)
  const [highlightedIndex, setHighlightedIndex] = useState(-1)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (autoFocus) inputRef.current?.focus()
  }, [autoFocus])

  const filtered = value
    ? suggestions.filter(
        (s) =>
          s.toLowerCase().includes(value.toLowerCase()) && s.toLowerCase() !== value.toLowerCase()
      )
    : suggestions

  const select = (suggestion: string) => {
    setOpen(false)
    setHighlightedIndex(-1)
    onSelect?.(suggestion)
  }

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const val = e.target.value
    setHighlightedIndex(-1)
    setOpen(val.length > 0)
    onChange(val)
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      if (!open) setOpen(true)
      setHighlightedIndex((i) => Math.min(i + 1, filtered.length - 1))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setHighlightedIndex((i) => Math.max(i - 1, -1))
    } else if (e.key === 'Tab') {
      const target = highlightedIndex >= 0 ? filtered[highlightedIndex] : filtered[0]
      if (open && target) {
        e.preventDefault()
        select(target)
      }
    } else if (e.key === 'Enter') {
      e.preventDefault()
      if (highlightedIndex >= 0 && filtered[highlightedIndex]) {
        select(filtered[highlightedIndex])
      } else {
        setOpen(false)
        onEnter?.()
      }
    } else if (e.key === 'Escape') {
      setOpen(false)
      setHighlightedIndex(-1)
    }
  }

  const handleBlur = () => {
    setTimeout(() => {
      setOpen(false)
      const exact = suggestions.find((s) => s.toLowerCase() === value.toLowerCase())
      if (exact) select(exact)
    }, 100)
  }

  return (
    <div className={cn('relative', className)}>
      <Input
        ref={inputRef}
        type="text"
        value={value}
        onChange={handleChange}
        onKeyDown={handleKeyDown}
        onFocus={() => setOpen(value.length > 0)}
        onBlur={handleBlur}
        placeholder={placeholder}
        aria-label={ariaLabel}
      />
      {open && filtered.length > 0 && (
        <ul className="absolute z-10 w-full mt-1 bg-card border border-border rounded-2xl shadow-elevated max-h-48 overflow-y-auto">
          {filtered.map((s, i) => (
            <li
              key={`${s}-${i}`}
              onMouseDown={() => select(s)}
              className={cn(
                'px-3 py-2 cursor-pointer text-sm transition-colors',
                i === highlightedIndex ? 'bg-accent text-white' : 'text-fg hover:bg-hover'
              )}
            >
              {s}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
