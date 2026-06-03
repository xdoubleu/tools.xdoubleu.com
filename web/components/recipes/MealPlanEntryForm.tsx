'use client'

import { useState } from 'react'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'
import RecipeCombobox from './RecipeCombobox'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogClose
} from '@/components/ui/dialog'

type Tab = 'recipe' | 'custom'

interface MealPlanEntryFormProps {
  open: boolean
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
  open,
  title,
  recipes,
  initialRecipeId = '',
  initialCustomName = '',
  initialServings = 1,
  saveLabel = 'Save',
  onSave,
  onCancel
}: MealPlanEntryFormProps) {
  const initialTab: Tab = initialRecipeId ? 'recipe' : 'custom'
  const [tab, setTab] = useState<Tab>(initialTab)
  const [recipeId, setRecipeId] = useState(initialRecipeId)
  const [servings, setServings] = useState(initialServings)
  const [customItems, setCustomItems] = useState<string[]>(
    initialCustomName ? initialCustomName.split('\n').filter(Boolean) : ['']
  )

  const handleSave = () => {
    if (tab === 'recipe') {
      if (!recipeId) return
      onSave(recipeId, '', servings)
    } else {
      const joined = customItems.filter((s) => s.trim()).join('\n')
      if (!joined) return
      onSave('', joined, 1)
    }
  }

  const addCustomItem = () => setCustomItems((prev) => [...prev, ''])
  const updateCustomItem = (i: number, val: string) =>
    setCustomItems((prev) => prev.map((item, idx) => (idx === i ? val : item)))
  const removeCustomItem = (i: number) =>
    setCustomItems((prev) => prev.filter((_, idx) => idx !== i))

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (!o) onCancel()
      }}
    >
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogClose aria-label="Close">×</DialogClose>
        </DialogHeader>

        <div className="flex gap-1 rounded-xl bg-surface p-1 mb-4">
          {(['recipe', 'custom'] as Tab[]).map((t) => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={`flex-1 rounded-lg py-1.5 text-sm font-medium transition-colors ${
                tab === t ? 'bg-card text-fg shadow-sm' : 'text-muted hover:text-fg'
              }`}
            >
              {t.charAt(0).toUpperCase() + t.slice(1)}
            </button>
          ))}
        </div>

        {tab === 'recipe' ? (
          <div className="space-y-3">
            <RecipeCombobox
              recipes={recipes}
              initialValue={recipes.find((r) => r.id === recipeId)?.name || ''}
              onSelect={(id) => setRecipeId(id)}
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
          </div>
        ) : (
          <div className="space-y-2">
            {customItems.map((item, i) => (
              <div key={i} className="flex gap-2">
                <input
                  type="text"
                  value={item}
                  onChange={(e) => updateCustomItem(i, e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      e.preventDefault()
                      if (i === customItems.length - 1) addCustomItem()
                    }
                  }}
                  placeholder={`Item ${i + 1}`}
                  autoFocus={i === 0}
                  className="flex h-11 flex-1 rounded-xl border border-input-border bg-input px-3 py-2 text-sm text-input-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
                />
                {customItems.length > 1 && (
                  <button
                    onClick={() => removeCustomItem(i)}
                    className="flex h-11 w-11 items-center justify-center rounded-xl text-lg font-bold text-danger hover:bg-danger/10"
                  >
                    ×
                  </button>
                )}
              </div>
            ))}
            <button onClick={addCustomItem} className="text-sm text-accent hover:underline">
              + Add item
            </button>
          </div>
        )}

        <div className="mt-4 flex gap-2">
          <Button onClick={handleSave} className="flex-1">
            {saveLabel}
          </Button>
          <Button variant="secondary" onClick={onCancel} className="flex-1">
            Cancel
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
