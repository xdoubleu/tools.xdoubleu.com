'use client'

import { useState, useRef, useEffect } from 'react'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'

interface RecipeComboboxProps {
  recipes: Recipe[]
  onSelect: (recipeId: string, customName: string) => void
  autoFocus?: boolean
  onEnter?: () => void
  initialValue?: string
}

export default function RecipeCombobox({
  recipes,
  onSelect,
  autoFocus,
  onEnter,
  initialValue = ''
}: RecipeComboboxProps) {
  const [inputValue, setInputValue] = useState(initialValue)
  const [open, setOpen] = useState(false)
  const [highlightedIndex, setHighlightedIndex] = useState(-1)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (autoFocus) inputRef.current?.focus()
  }, [autoFocus])

  const filtered = inputValue
    ? recipes.filter((r) => r.name.toLowerCase().includes(inputValue.toLowerCase()))
    : recipes

  const selectRecipe = (recipe: Recipe) => {
    setInputValue(recipe.name)
    setOpen(false)
    setHighlightedIndex(-1)
    onSelect(recipe.id, '')
  }

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const val = e.target.value
    setInputValue(val)
    setHighlightedIndex(-1)
    setOpen(val.length > 0)
    onSelect('', val)
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      if (!open) setOpen(true)
      setHighlightedIndex((i) => Math.min(i + 1, filtered.length - 1))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setHighlightedIndex((i) => Math.max(i - 1, -1))
    } else if (e.key === 'Enter') {
      e.preventDefault()
      if (highlightedIndex >= 0 && filtered[highlightedIndex]) {
        selectRecipe(filtered[highlightedIndex])
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
      const exact = recipes.find((r) => r.name.toLowerCase() === inputValue.toLowerCase())
      if (exact) {
        setInputValue(exact.name)
        onSelect(exact.id, '')
      }
    }, 100)
  }

  return (
    <div className="relative">
      <input
        ref={inputRef}
        type="text"
        value={inputValue}
        onChange={handleChange}
        onKeyDown={handleKeyDown}
        onFocus={() => setOpen(inputValue.length > 0)}
        onBlur={handleBlur}
        placeholder="Recipe name or custom meal..."
        className="w-full px-3 py-2 rounded border border-input-border bg-input text-input-text"
      />
      {open && filtered.length > 0 && (
        <ul className="absolute z-10 w-full mt-1 bg-card border border-border rounded shadow-md max-h-48 overflow-y-auto">
          {filtered.map((r, i) => (
            <li
              key={r.id}
              onMouseDown={() => selectRecipe(r)}
              className={`px-3 py-2 cursor-pointer text-sm ${
                i === highlightedIndex ? 'bg-blue-600 text-white' : 'hover:bg-surface'
              }`}
            >
              {r.name}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
