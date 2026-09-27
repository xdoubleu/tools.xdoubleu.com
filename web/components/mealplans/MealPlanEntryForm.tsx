'use client'

import { useMemo, useState } from 'react'
import { useSWRConfig } from 'swr'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'
import type { MealSuggestion } from './useMealCalendarState'
import RecipeCombobox from '@/components/mealplans/RecipeCombobox'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { Card } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { SegmentedTabs } from '@/components/ui/segmented-tabs'
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
  DialogClose,
  DialogFooter
} from '@/components/ui/dialog'

type Tab = 'recipe' | 'custom'

interface MealPlanEntryFormProps {
  open: boolean
  title: string
  recipes: Recipe[]
  suggestedRecipes?: MealSuggestion[]
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
  suggestedRecipes = [],
  initialRecipeId = '',
  initialCustomName = '',
  initialServings = 1,
  initialExcludeFromShoppingList = false,
  saveLabel = 'Save',
  onSave,
  onCancel
}: MealPlanEntryFormProps) {
  const initialTab: Tab = initialCustomName ? 'custom' : 'recipe'
  const [tab, setTab] = useState<Tab>(initialTab)
  const [recipeId, setRecipeId] = useState(initialRecipeId)
  const [servings, setServings] = useState(initialServings)
  // Remounts RecipeCombobox so a suggestion chip's name shows in the input.
  const [comboboxKey, setComboboxKey] = useState(0)
  const [customItems, setCustomItems] = useState<CustomItem[]>(() => {
    const parsed = initialCustomName ? parseCustomItems(initialCustomName) : []
    return parsed.length > 0 ? parsed : [{ name: '', amount: '', unit: '' }]
  })
  const [excludeFromShoppingList, setExcludeFromShoppingList] = useState(
    initialExcludeFromShoppingList
  )

  const { data: categoriesData } = useCategories()
  const { data: itemCategoriesData } = useItemCategories()
  const categories = categoriesData?.categories ?? []
  const { mutate: globalMutate } = useSWRConfig()

  // Catalog assignments by normalized name, for pre-fill and skipping writes.
  const nameToCategoryId = useMemo(() => {
    const map: Record<string, string> = {}
    for (const entry of itemCategoriesData?.items ?? []) {
      map[entry.name] = entry.categoryId
    }
    return map
  }, [itemCategoriesData])

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
      // Skip blanks, dupes, and rows already matching the catalog.
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

  const selectSuggestion = (suggestion: MealSuggestion) => {
    setRecipeId(suggestion.recipe.id)
    setServings(suggestion.servings)
    setComboboxKey((k) => k + 1)
  }

  const addCustomItem = () =>
    setCustomItems((prev) => [...prev, { name: '', amount: '', unit: '' }])
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
      <DialogContent side="sheet">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogClose aria-label="Close">×</DialogClose>
        </DialogHeader>

        <SegmentedTabs<Tab>
          aria-label="Meal type"
          className="mb-4"
          value={tab}
          onChange={setTab}
          options={[
            { value: 'recipe', label: 'Recipe' },
            { value: 'custom', label: 'Custom' }
          ]}
        />

        {tab === 'recipe' ? (
          <div className="space-y-3">
            {suggestedRecipes.length > 0 && (
              <div className="space-y-1.5">
                <span className="text-xs font-medium text-muted">Suggestions</span>
                <div className="flex flex-wrap gap-1.5">
                  {suggestedRecipes.map((suggestion) => (
                    <Button
                      key={suggestion.recipe.id}
                      variant={suggestion.recipe.id === recipeId ? 'default' : 'secondary'}
                      size="sm"
                      className="rounded-lg"
                      onClick={() => selectSuggestion(suggestion)}
                    >
                      {suggestion.recipe.name}
                    </Button>
                  ))}
                </div>
              </div>
            )}
            <RecipeCombobox
              key={comboboxKey}
              recipes={recipes}
              initialValue={recipes.find((r) => r.id === recipeId)?.name || ''}
              onSelect={(id) => setRecipeId(id)}
              autoFocus
              onEnter={handleSave}
            />
            <Input
              type="number"
              inputMode="numeric"
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
              <Card key={i} variant="inset" className="space-y-2 p-2">
                <div className="flex flex-wrap gap-2 sm:flex-nowrap">
                  {!excludeFromShoppingList && (
                    <>
                      <Input
                        type="number"
                        inputMode="decimal"
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
                        className="min-w-0 flex-1 sm:w-20 sm:flex-none"
                      />
                      <Input
                        type="text"
                        value={item.unit ?? ''}
                        onChange={(e) => updateCustomItem(i, { unit: e.target.value })}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter') {
                            e.preventDefault()
                            if (i === customItems.length - 1) addCustomItem()
                          }
                        }}
                        placeholder="Unit"
                        aria-label={`Unit for item ${i + 1}`}
                        className="min-w-0 flex-1 sm:w-20 sm:flex-none"
                      />
                    </>
                  )}
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
                    className={cn(
                      'min-w-0 flex-1',
                      !excludeFromShoppingList && 'order-last basis-full sm:order-none sm:basis-0'
                    )}
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
                {categories.length > 0 && !excludeFromShoppingList && (
                  <Select
                    aria-label={`Category for item ${i + 1}`}
                    value={effectiveCategoryId(item)}
                    onChange={(e) => updateCustomItem(i, { categoryId: e.target.value })}
                  >
                    <option value="">-- Category --</option>
                    {categories.map((category) => (
                      <option key={category.id} value={category.id}>
                        {category.name}
                      </option>
                    ))}
                  </Select>
                )}
              </Card>
            ))}
            <Button variant="link" size="sm" className="self-start" onClick={addCustomItem}>
              + Add item
            </Button>
            <Checkbox
              checked={excludeFromShoppingList}
              onChange={(e) => setExcludeFromShoppingList(e.target.checked)}
              label={<span className="text-xs text-muted">Keep off the shopping list</span>}
            />
          </div>
        )}

        <DialogFooter>
          <Button variant="secondary" onClick={onCancel} className="flex-1 sm:flex-none">
            Cancel
          </Button>
          <Button onClick={handleSave} className="flex-1 sm:flex-none">
            {saveLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
