'use client'

import { useState, useRef, useImperativeHandle, forwardRef, useEffect } from 'react'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { createServiceClient } from '@/lib/client'
import { TaskService } from '@/lib/gen/todos/v1/tasks_pb'

interface QuickAddBarProps {
  sections: Array<{ id: string; name: string }>
  labelPresets: Array<{ value: string; color: string }>
  sectionId?: string
  onAdded: () => void
  onClose: () => void
}

export interface QuickAddBarHandle {
  focus: () => void
}

const QuickAddBar = forwardRef<QuickAddBarHandle, QuickAddBarProps>(
  ({ sections, labelPresets, sectionId, onAdded, onClose }, ref) => {
    const inputRef = useRef<HTMLInputElement>(null)
    const formRef = useRef<HTMLFormElement>(null)
    const [input, setInput] = useState('')
    const [showDropdown, setShowDropdown] = useState(false)
    const [dropdownType, setDropdownType] = useState<'label' | 'section' | null>(null)

    useImperativeHandle(ref, () => ({
      focus: () => inputRef.current?.focus()
    }))

    useEffect(() => {
      const handleKeyDown = (e: KeyboardEvent) => {
        if (e.key === 'Escape') {
          onClose()
        }
      }
      window.addEventListener('keydown', handleKeyDown)
      return () => window.removeEventListener('keydown', handleKeyDown)
    }, [onClose])

    function handleInputChange(value: string) {
      setInput(value)
      if (value.includes('@')) {
        setDropdownType('label')
        setShowDropdown(true)
      } else if (value.includes('#')) {
        setDropdownType('section')
        setShowDropdown(true)
      } else {
        setShowDropdown(false)
      }
    }

    async function handleSubmit(e: React.FormEvent) {
      e.preventDefault()
      const trimmed = input.trim()
      if (!trimmed) return
      setInput('')
      setShowDropdown(false)
      const client = createServiceClient(TaskService)
      await client.quickAddTask({ input: trimmed, sectionId: sectionId ?? '' })
      onAdded()
      onClose()
    }

    function handleBlur(e: React.FocusEvent) {
      // Stay open if focus moves within the form (e.g. clicking a dropdown option)
      if (e.relatedTarget instanceof Node && formRef.current?.contains(e.relatedTarget)) return
      if (!showDropdown) onClose()
    }

    const filteredLabels =
      dropdownType === 'label'
        ? labelPresets.filter((l) => l.value.toLowerCase().includes(input.split('@')[1] || ''))
        : []

    const filteredSections =
      dropdownType === 'section'
        ? sections.filter((s) => s.name.toLowerCase().includes(input.split('#')[1] || ''))
        : []

    const MAX = 80
    const showCounter = input.length > 60

    return (
      <form ref={formRef} onSubmit={handleSubmit} onBlur={handleBlur} className="relative mb-4">
        <div className="relative">
          <Input
            ref={inputRef}
            type="text"
            value={input}
            onChange={(e) => handleInputChange(e.target.value)}
            placeholder="Add task... use @label #section p1-3 !date"
            maxLength={MAX}
            autoFocus
          />
          {showCounter && (
            <span
              className={`pointer-events-none absolute right-2 top-1/2 -translate-y-1/2 text-xs tabular-nums ${input.length >= MAX ? 'text-danger' : 'text-muted'}`}
            >
              {input.length}/{MAX}
            </span>
          )}
        </div>

        {showDropdown && dropdownType === 'label' && filteredLabels.length > 0 && (
          <div className="absolute top-full left-0 right-0 mt-1 bg-card border border-border rounded-2xl shadow-elevated z-10">
            {filteredLabels.map((label) => (
              <button
                key={label.value}
                type="button"
                onMouseDown={(e) => e.preventDefault()}
                onClick={() => {
                  setInput(input.replace(/@\S*$/, `@${label.value}`))
                  setShowDropdown(false)
                  inputRef.current?.focus()
                }}
                className="w-full text-left px-4 py-2 hover:bg-surface flex items-center gap-2 text-sm"
              >
                <div className="w-3 h-3 rounded-full" style={{ backgroundColor: label.color }} />
                {label.value}
              </button>
            ))}
          </div>
        )}

        {showDropdown && dropdownType === 'section' && filteredSections.length > 0 && (
          <div className="absolute top-full left-0 right-0 mt-1 bg-card border border-border rounded-2xl shadow-elevated z-10">
            {filteredSections.map((section) => (
              <button
                key={section.id}
                type="button"
                onMouseDown={(e) => e.preventDefault()}
                onClick={() => {
                  setInput(input.replace(/#\S*$/, `#${section.name}`))
                  setShowDropdown(false)
                  inputRef.current?.focus()
                }}
                className="w-full text-left px-4 py-2 hover:bg-surface text-sm"
              >
                {section.name}
              </button>
            ))}
          </div>
        )}

        <Button type="submit" className="mt-2 w-full">
          Add Task
        </Button>
      </form>
    )
  }
)

QuickAddBar.displayName = 'QuickAddBar'

export default QuickAddBar
