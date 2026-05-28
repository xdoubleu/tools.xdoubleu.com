'use client'

import { useState } from 'react'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'
import RecipeCombobox from './RecipeCombobox'
import { Button } from '@/components/ui/button'

interface MealPlanEntryFormProps {
  title: string
  recipes: Recipe[]
  initialRecipeId?: string
  initialCustomName?: string
  initialServings?: number
  saveLabel?: string
  onSave: (recipeId: string, customName: string, servings: number) => void
  onCancel: () => void
}

export default function MealPlanEntryForm({
  title,
  recipes,
  initialRecipeId = '',
  initialCustomName = '',
  initialServings = 1,
  saveLabel = 'Save',
  onSave,
  onCancel
}: MealPlanEntryFormProps) {
  const [recipeId, setRecipeId] = useState(initialRecipeId)
  const [customName, setCustomName] = useState(initialCustomName)
  const [servings, setServings] = useState(initialServings)

  const handleSave = () => {
    if (!recipeId && !customName.trim()) return
    onSave(recipeId, customName, servings)
  }

  return (
    <div className="rounded-2xl border border-border bg-card p-4 shadow-card space-y-3">
      <h3 className="text-sm font-semibold text-fg">{title}</h3>
      <RecipeCombobox
        recipes={recipes}
        initialValue={
          initialCustomName || recipes.find((r) => r.id === initialRecipeId)?.name || ''
        }
        onSelect={(id, name) => {
          setRecipeId(id)
          setCustomName(name)
        }}
        autoFocus
        onEnter={handleSave}
      />
      <input
        type="number"
        min="1"
        value={servings}
        onChange={(e) => setServings(parseInt(e.target.value, 10))}
        onKeyDown={(e) => e.key === 'Enter' && handleSave()}
        placeholder="Servings"
        className="flex h-11 w-full rounded-xl border border-input-border bg-input px-3 py-2 text-sm text-input-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
      />
      <div className="flex gap-2">
        <Button onClick={handleSave} className="flex-1">
          {saveLabel}
        </Button>
        <Button variant="secondary" onClick={onCancel} className="flex-1">
          Cancel
        </Button>
      </div>
    </div>
  )
}
