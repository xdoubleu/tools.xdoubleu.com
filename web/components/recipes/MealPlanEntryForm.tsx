'use client'

import { useMemo, useState } from 'react'
import { useSWRConfig } from 'swr'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'
import RecipeCombobox from './RecipeCombobox'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { cn } from '@/lib/cn'
import { parseCustomItems, encodeCustomItems, type CustomItem } from '@/lib/customItems'
import { useCategories, useItemCategories } from '@/hooks/useShoppingList'
import { createServiceClient } from '@/lib/client'
import { ShoppingListService } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'
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
  initialExcludeFromShoppingList?: boolean
  saveLabel?: string
  onSave: (
    recipeId: string,
    customName: string,
    servings: number,
    excludeFromShoppingList: boolean
  ) => void
  onCancel: () => void
}

export default function MealPlanEntryForm({
  open,
  title,
  recipes,
  initialRecipeId = '',
  initialCustomName = '',
  initialServings = 1,
  initialExcludeFromShoppingList = false,
  saveLabel = 'Save',
  onSave,
  onCancel
}: MealPlanEntryFormProps) {
  const initialTab: Tab = initialRecipeId ? 'recipe' : 'custom'
  const [tab, setTab] = useState<Tab>(initialTab)
  const [recipeId, setRecipeId] = useState(initialRecipeId)
  const [servings, setServings] = useState(initialServings)
  const [customItems, setCustomItems] = useState<CustomItem[]>(() => {
    const parsed = initialCustomName ? parseCustomItems(initialCustomName) : []
    return parsed.length > 0 ? parsed : [{ name: '', amount: '' }]
  })
  const [excludeFromShoppingList, setExcludeFromShoppingList] = useState(
    initialExcludeFromShoppingList
  )

  const { data: categoriesData } = useCategories()
  const { data: itemCategoriesData } = useItemCategories()
  const categories = categoriesData?.categories ?? []
  const { mutate: globalMutate } = useSWRConfig()

  // Current name->category catalog assignments, keyed by normalized name. Used
  // to pre-fill each row's category and to avoid redundant catalog writes.
  const nameToCategoryId = useMemo(() => {
    const map: Record<string, string> = {}
    for (const entry of itemCategoriesData?.items ?? []) {
      map[entry.name] = entry.categoryId
    }
    return map
  }, [itemCategoriesData])

  // The category shown for a row: an explicit choice wins, otherwise fall back
  // to the name's existing catalog assignment.
  const effectiveCategoryId = (item: CustomItem) =>
    item.categoryId ?? nameToCategoryId[item.name.trim().toLowerCase()] ?? ''

  const persistCategories = async () => {
    const client = createServiceClient(ShoppingListService)
    const seen = new Set<string>()
    let wrote = false
    for (const item of customItems) {
      const name = item.name.trim()
      const categoryId = effectiveCategoryId(item)
      const key = name.toLowerCase()
      // Skip blanks, dupes (last write wins is handled by order), and rows
      // already matching the catalog.
      if (!name || !categoryId || seen.has(key)) continue
      seen.add(key)
      if (nameToCategoryId[key] === categoryId) continue
      await client.setItemCategory({ name, categoryId })
      wrote = true
    }
    if (wrote) {
      await globalMutate('/shoppinglist/item-categories')
      await globalMutate('/shoppinglist/item-names')
    }
  }

  const handleSave = async () => {
    if (tab === 'recipe') {
      if (!recipeId) return
      onSave(recipeId, '', servings, false)
    } else {
      const joined = encodeCustomItems(customItems)
      if (!joined) return
      await persistCategories()
      onSave('', joined, 1, excludeFromShoppingList)
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
          {(['recipe', 'custom'] as Tab[]).map((t) => (
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
        ) : (
          <div className="space-y-2">
            {customItems.map((item, i) => (
              <div key={i} className="space-y-2 rounded-xl border border-border p-2">
                <div className="flex gap-2">
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
                {categories.length > 0 && (
                  <Select
                    aria-label={`Category for item ${i + 1}`}
                    value={effectiveCategoryId(item)}
                    onChange={(e) => updateCustomItem(i, { categoryId: e.target.value })}
                    className="h-9"
                  >
                    <option value="">-- Category --</option>
                    {categories.map((category) => (
                      <option key={category.id} value={category.id}>
                        {category.name}
                      </option>
                    ))}
                  </Select>
                )}
              </div>
            ))}
            <Button variant="link" size="sm" className="self-start" onClick={addCustomItem}>
              + Add item
            </Button>
            <label className="flex items-center gap-2 pt-1 text-xs text-muted">
              <input
                type="checkbox"
                checked={excludeFromShoppingList}
                onChange={(e) => setExcludeFromShoppingList(e.target.checked)}
                className="size-4 rounded accent-accent"
              />
              Keep off the shopping list
            </label>
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
