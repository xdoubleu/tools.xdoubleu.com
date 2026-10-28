import { useState, useRef, useEffect, useId } from 'react'
import { cn } from '@/lib/cn'
import { Input } from '@/components/ui/input'

interface ComboboxProps {
  value: string
  onChange: (value: string) => void
  /** Called when the user picks a suggestion (click, keyboard, or blur snap). */
  onSelect?: (value: string) => void
  suggestions: string[]
  placeholder?: string
  className?: string
  /** The input's `id`, for a `Field` label. */
  id?: string
  autoFocus?: boolean
  /** Called when Enter is pressed and no suggestion is highlighted. */
  onEnter?: () => void
  'aria-label'?: string
  /** `false` shows `suggestions` as given, for results already filtered elsewhere (e.g. a server search). */
  filterSuggestions?: boolean
}

export function Combobox({
  value,
  onChange,
  onSelect,
  suggestions,
  placeholder,
  className,
  id,
  autoFocus,
  onEnter,
  'aria-label': ariaLabel,
  filterSuggestions = true
}: ComboboxProps) {
  const [open, setOpen] = useState(false)
  const [highlightedIndex, setHighlightedIndex] = useState(-1)
  const inputRef = useRef<HTMLInputElement>(null)
  const listboxId = useId()

  useEffect(() => {
    if (autoFocus) inputRef.current?.focus()
  }, [autoFocus])

  const filtered =
    value && filterSuggestions
      ? suggestions.filter(
          (s) =>
            s.toLowerCase().includes(value.toLowerCase()) && s.toLowerCase() !== value.toLowerCase()
        )
      : suggestions
  const expanded = open && filtered.length > 0
  const optionId = (i: number) => `${listboxId}-${i}`

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
      // Commits only an explicit highlight; focus still moves on.
      const target = filtered[highlightedIndex]
      if (open && target) select(target)
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
        id={id}
        type="text"
        value={value}
        onChange={handleChange}
        onKeyDown={handleKeyDown}
        onFocus={() => setOpen(value.length > 0)}
        onBlur={handleBlur}
        placeholder={placeholder}
        aria-label={ariaLabel}
        role="combobox"
        aria-autocomplete="list"
        aria-expanded={expanded}
        aria-controls={expanded ? listboxId : undefined}
        aria-activedescendant={
          expanded && filtered[highlightedIndex] ? optionId(highlightedIndex) : undefined
        }
      />
      {expanded && (
        <ul
          id={listboxId}
          role="listbox"
          aria-label={ariaLabel ?? 'Suggestions'}
          className="absolute z-10 w-full mt-1 bg-card border border-border rounded-2xl shadow-elevated max-h-48 overflow-y-auto"
        >
          {filtered.map((s, i) => (
            <li
              key={`${s}-${i}`}
              id={optionId(i)}
              role="option"
              aria-selected={i === highlightedIndex}
              onMouseDown={() => select(s)}
              className={cn(
                'flex min-h-11 items-center px-3 py-2 cursor-pointer text-sm transition-colors sm:min-h-9',
                i === highlightedIndex
                  ? 'bg-accent text-white'
                  : 'text-fg hover:bg-hover active:bg-hover'
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
