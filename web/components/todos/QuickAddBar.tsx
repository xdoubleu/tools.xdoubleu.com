'use client'

import { useState, useRef, useImperativeHandle, forwardRef, useEffect } from 'react'

interface QuickAddBarProps {
  sections: Array<{ id: string; name: string }>
  labelPresets: Array<{ value: string; color: string }>
  onAdded: () => void
}

interface QuickAddBarHandle {
  focus: () => void
}

const QuickAddBar = forwardRef<QuickAddBarHandle, QuickAddBarProps>(
  ({ sections, labelPresets, onAdded }, ref) => {
    const inputRef = useRef<HTMLInputElement>(null)
    const [input, setInput] = useState('')
    const [showDropdown, setShowDropdown] = useState(false)
    const [dropdownType, setDropdownType] = useState<'label' | 'section' | null>(null)

    useImperativeHandle(ref, () => ({
      focus: () => {
        inputRef.current?.focus()
      }
    }))

    useEffect(() => {
      const handleKeyDown = (e: KeyboardEvent) => {
        if (e.key === '/') {
          const target = e.target
          if (
            !(target instanceof HTMLInputElement) &&
            !(target instanceof HTMLTextAreaElement)
          ) {
            e.preventDefault()
            inputRef.current?.focus()
          }
        }
      }

      window.addEventListener('keydown', handleKeyDown)
      return () => window.removeEventListener('keydown', handleKeyDown)
    }, [])

    const handleInputChange = (value: string) => {
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

    const handleSubmit = async (e: React.FormEvent) => {
      e.preventDefault()
      if (!input.trim()) return

      // For now, just clear and call callback
      // In a real implementation, this would call the API
      setInput('')
      setShowDropdown(false)
      onAdded()
    }

    const filteredLabels =
      dropdownType === 'label'
        ? labelPresets.filter((l) => l.value.toLowerCase().includes(input.split('@')[1] || ''))
        : []

    const filteredSections =
      dropdownType === 'section'
        ? sections.filter((s) => s.name.toLowerCase().includes(input.split('#')[1] || ''))
        : []

    return (
      <form onSubmit={handleSubmit} className="relative mb-4">
        <input
          ref={inputRef}
          type="text"
          value={input}
          onChange={(e) => handleInputChange(e.target.value)}
          placeholder="Add task... use @label #section p1-3 !date"
          className="w-full px-4 py-2 rounded border border-input-border bg-input text-input-text focus:outline-none focus:ring-2 focus:ring-blue-500"
        />

        {showDropdown && dropdownType === 'label' && filteredLabels.length > 0 && (
          <div className="absolute top-full left-0 right-0 mt-1 bg-card border border-border rounded shadow-lg z-10">
            {filteredLabels.map((label) => (
              <button
                key={label.value}
                type="button"
                onClick={() => {
                  setInput(input.replace(/@\S*$/, `@${label.value}`))
                  setShowDropdown(false)
                }}
                className="w-full text-left px-4 py-2 hover:bg-surface flex items-center gap-2"
              >
                <div
                  className="w-3 h-3 rounded"
                  style={{ backgroundColor: label.color }}
                />
                {label.value}
              </button>
            ))}
          </div>
        )}

        {showDropdown && dropdownType === 'section' && filteredSections.length > 0 && (
          <div className="absolute top-full left-0 right-0 mt-1 bg-card border border-border rounded shadow-lg z-10">
            {filteredSections.map((section) => (
              <button
                key={section.id}
                type="button"
                onClick={() => {
                  setInput(input.replace(/#\S*$/, `#${section.name}`))
                  setShowDropdown(false)
                }}
                className="w-full text-left px-4 py-2 hover:bg-surface"
              >
                {section.name}
              </button>
            ))}
          </div>
        )}

        <button
          type="submit"
          className="mt-2 w-full px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700"
        >
          Add Task
        </button>
      </form>
    )
  }
)

QuickAddBar.displayName = 'QuickAddBar'

export default QuickAddBar
