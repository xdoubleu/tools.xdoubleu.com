'use client'

import { useState } from 'react'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'
import RecipeCombobox from './RecipeCombobox'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/cn'
import { parseCustomItems, encodeCustomItems, type CustomItem } from '@/lib/customItems'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogClose
} from '@/components/ui/dialog'

type Tab = 'recipe' | 'custom' | 'event'

interface MealPlanEntryFormProps {
  open: boolean
  title: string
  recipes: Recipe[]
  initialRecipeId?: string
  initialCustomName?: string
  initialServings?: number
  initialIsEvent?: boolean
  saveLabel?: string
  onSave: (recipeId: string, customName: string, servings: number, isEvent: boolean) => void
  onCancel: () => void
}

export default function MealPlanEntryForm({
  open,
  title,
  recipes,
  initialRecipeId = '',
  initialCustomName = '',
  initialServings = 1,
  initialIsEvent = false,
  saveLabel = 'Save',
  onSave,
  onCancel
}: MealPlanEntryFormProps) {
  const initialTab: Tab = initialRecipeId ? 'recipe' : initialIsEvent ? 'event' : 'custom'
  const [tab, setTab] = useState<Tab>(initialTab)
  const [recipeId, setRecipeId] = useState(initialRecipeId)
  const [servings, setServings] = useState(initialServings)
  const [customItems, setCustomItems] = useState<CustomItem[]>(() => {
    const parsed = !initialIsEvent && initialCustomName ? parseCustomItems(initialCustomName) : []
    return parsed.length > 0 ? parsed : [{ name: '', amount: '' }]
  })
  const [eventName, setEventName] = useState(initialIsEvent ? initialCustomName : '')

  const handleSave = () => {
    if (tab === 'recipe') {
      if (!recipeId) return
      onSave(recipeId, '', servings, false)
    } else if (tab === 'event') {
      const trimmed = eventName.trim()
      if (!trimmed) return
      onSave('', trimmed, 1, true)
    } else {
      const joined = encodeCustomItems(customItems)
      if (!joined) return
      onSave('', joined, 1, false)
    }
  }

  const addCustomItem = () => setCustomItems((prev) => [...prev, { name: '', amount: '' }])
  const updateCustomItem = (i: number, patch: Partial<CustomItem>) =>
    setCustomItems((prev) => prev.map((item, idx) => (idx === i ? { ...item, ...patch } : item)))
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
          {(['recipe', 'custom', 'event'] as Tab[]).map((t) => (
            <Button
              key={t}
              variant="ghost"
              size="sm"
              onClick={() => setTab(t)}
              className={cn(
                'flex-1 rounded-lg',
                tab === t
                  ? 'bg-card text-fg shadow-sm'
                  : 'text-muted hover:bg-transparent hover:text-fg'
              )}
            >
              {t.charAt(0).toUpperCase() + t.slice(1)}
            </Button>
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
            <Input
              type="number"
              min="1"
              value={servings}
              onChange={(e) => setServings(parseInt(e.target.value, 10))}
              onKeyDown={(e) => e.key === 'Enter' && handleSave()}
              placeholder="Servings"
            />
          </div>
        ) : tab === 'event' ? (
          <div className="space-y-2">
            <Input
              type="text"
              value={eventName}
              onChange={(e) => setEventName(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleSave()}
              placeholder="Event name"
              autoFocus
            />
            <p className="text-xs text-muted">Events stay on the calendar but aren’t exported.</p>
          </div>
        ) : (
          <div className="space-y-2">
            {customItems.map((item, i) => (
              <div key={i} className="flex gap-2">
                <Input
                  type="text"
                  value={item.name}
                  onChange={(e) => updateCustomItem(i, { name: e.target.value })}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      e.preventDefault()
                      if (i === customItems.length - 1) addCustomItem()
                    }
                  }}
                  placeholder={`Item ${i + 1}`}
                  autoFocus={i === 0}
                  className="flex-1"
                />
                <Input
                  type="number"
                  min="0"
                  step="any"
                  value={item.amount}
                  onChange={(e) => updateCustomItem(i, { amount: e.target.value })}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      e.preventDefault()
                      if (i === customItems.length - 1) addCustomItem()
                    }
                  }}
                  placeholder="Qty"
                  aria-label={`Amount for item ${i + 1}`}
                  className="w-20"
                />
                {customItems.length > 1 && (
                  <Button
                    variant="destructive"
                    size="icon"
                    aria-label="Remove item"
                    onClick={() => removeCustomItem(i)}
                  >
                    ×
                  </Button>
                )}
              </div>
            ))}
            <Button variant="link" size="sm" className="self-start" onClick={addCustomItem}>
              + Add item
            </Button>
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
