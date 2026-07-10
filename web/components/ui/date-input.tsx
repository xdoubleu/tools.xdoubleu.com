'use client'

import { useEffect, useState } from 'react'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/cn'

interface DateInputProps {
  /** 'YYYY-MM-DD' or '' */
  value: string
  /** Always receives 'YYYY-MM-DD' or '' — never a partial date. */
  onChange: (value: string) => void
  onBlur?: () => void
  id?: string
  className?: string
  'aria-label'?: string
}

function isoToDisplay(iso: string): string {
  if (!iso) return ''
  const [y, m, d] = iso.split('-')
  return `${d}/${m}/${y}`
}

function displayToIso(text: string): string | null {
  const match = /^(\d{1,2})\/(\d{1,2})\/(\d{4})$/.exec(text)
  if (!match) return null
  const [d, m, y] = [Number(match[1]), Number(match[2]), Number(match[3])]
  const date = new Date(Date.UTC(y, m - 1, d))
  const valid =
    date.getUTCFullYear() === y && date.getUTCMonth() === m - 1 && date.getUTCDate() === d
  if (!valid) return null
  return `${y}-${String(m).padStart(2, '0')}-${String(d).padStart(2, '0')}`
}

/**
 * dd/MM/yyyy text field with a native date picker behind the calendar button.
 * Native date inputs render in the browser locale (Safari shows MM/DD for
 * en-US languages), so the visible field is a text input we format ourselves.
 */
export function DateInput({
  value,
  onChange,
  onBlur,
  id,
  className,
  'aria-label': ariaLabel
}: DateInputProps) {
  const [text, setText] = useState(isoToDisplay(value))

  useEffect(() => {
    setText((current) =>
      displayToIso(current) === (value || null) ? current : isoToDisplay(value)
    )
  }, [value])

  const handleTextChange = (next: string) => {
    setText(next)
    if (next === '') {
      onChange('')
      return
    }
    const iso = displayToIso(next)
    if (iso) onChange(iso)
  }

  const handleBlur = () => {
    const iso = displayToIso(text)
    if (iso) setText(isoToDisplay(iso))
    else if (text !== '') setText(isoToDisplay(value))
    onBlur?.()
  }

  return (
    <div className={cn('relative h-11 w-full', className)}>
      <Input
        id={id}
        inputMode="numeric"
        placeholder="dd/mm/yyyy"
        aria-label={ariaLabel}
        value={text}
        onChange={(e) => handleTextChange(e.target.value)}
        onBlur={handleBlur}
        className="h-full w-full pr-9"
      />
      <span className="absolute inset-y-0 right-0 flex w-9 items-center justify-center text-muted">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          width={16}
          height={16}
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          aria-hidden="true"
        >
          <rect x="3" y="4" width="18" height="18" rx="2" ry="2" />
          <line x1="16" y1="2" x2="16" y2="6" />
          <line x1="8" y1="2" x2="8" y2="6" />
          <line x1="3" y1="10" x2="21" y2="10" />
        </svg>
        <input
          type="date"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onClick={(e) => {
            try {
              e.currentTarget.showPicker()
            } catch {
              // showPicker unsupported: focus still opens the picker on mobile
            }
          }}
          tabIndex={-1}
          aria-hidden="true"
          className="absolute inset-0 h-full w-full cursor-pointer opacity-0"
        />
      </span>
    </div>
  )
}
