import { useState } from 'react'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'
import { FieldError } from '@/components/ui/field-error'
import { Combobox } from '@/components/ui/combobox'

interface RecipeComboboxProps {
  recipes: Recipe[]
  onSelect: (recipeId: string, customName: string) => void
  autoFocus?: boolean
  onEnter?: () => void
  /** Called with the raw text on every keystroke. */
  onTextChange?: (text: string) => void
  error?: string
  initialValue?: string
}

export default function RecipeCombobox({
  recipes,
  onSelect,
  autoFocus,
  onEnter,
  onTextChange,
  error,
  initialValue = ''
}: RecipeComboboxProps) {
  const [inputValue, setInputValue] = useState(initialValue)

  const handleChange = (value: string) => {
    setInputValue(value)
    onSelect('', value)
    onTextChange?.(value)
  }

  const handleSelect = (name: string) => {
    setInputValue(name)
    const match = recipes.find((r) => r.name.toLowerCase() === name.toLowerCase())
    onSelect(match?.id ?? '', '')
    onTextChange?.(name)
  }

  return (
    <>
      <Combobox
        value={inputValue}
        onChange={handleChange}
        onSelect={handleSelect}
        suggestions={recipes.map((r) => r.name)}
        placeholder="Recipe name…"
        aria-label="Recipe"
        autoFocus={autoFocus}
        onEnter={onEnter}
        listInFlow
      />
      {error && <FieldError id="recipe-error">{error}</FieldError>}
    </>
  )
}
