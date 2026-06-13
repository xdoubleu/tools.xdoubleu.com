'use client'

import { useState } from 'react'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'
import { Combobox } from '@/components/ui/combobox'

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

  const handleChange = (value: string) => {
    setInputValue(value)
    onSelect('', value)
  }

  const handleSelect = (name: string) => {
    setInputValue(name)
    const match = recipes.find((r) => r.name.toLowerCase() === name.toLowerCase())
    onSelect(match?.id ?? '', '')
  }

  return (
    <Combobox
      value={inputValue}
      onChange={handleChange}
      onSelect={handleSelect}
      suggestions={recipes.map((r) => r.name)}
      placeholder="Recipe name or custom meal..."
      autoFocus={autoFocus}
      onEnter={onEnter}
    />
  )
}
