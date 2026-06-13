'use client'

import { useState, useMemo, useEffect, useRef } from 'react'
import { useCreateRecipe, useUpdateRecipe } from '@/hooks/useRecipes'
import type { CreateRecipeInput, UpdateRecipeInput } from '@/hooks/useRecipes'
import { useItemNames, useCategories } from '@/hooks/useShoppingList'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'
import { parseFraction } from '@/lib/recipes/parseFraction'
import { createServiceClient } from '@/lib/client'
import { ShoppingListService } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Select } from '@/components/ui/select'
import { Combobox } from '@/components/ui/combobox'

interface RecipeFormProps {
  recipe?: Recipe
  onSave: (id: string) => void
  onCancel: () => void
}

interface IngredientRow {
  name: string
  amount: string
  unit: string
  group: string
  categoryId: string
  newCategoryName: string
}

// Sentinel category id that switches the category select into "create a new
// category" mode, revealing the name input next to it.
const NEW_CATEGORY = '__new__'

const normalizeName = (name: string) => name.toLowerCase().trim()

export default function RecipeForm({ recipe, onSave, onCancel }: RecipeFormProps) {
  const [name, setName] = useState(recipe?.name || '')
  const [servings, setServings] = useState(recipe?.baseServings?.toString() || '1')
  const [batchServings, setBatchServings] = useState(recipe?.batchServings?.toString() || '')
  const [steps, setSteps] = useState(recipe?.instructions || '')
  const [ingredients, setIngredients] = useState<IngredientRow[]>(
    recipe?.ingredients?.map((ing) => ({
      name: ing.name,
      amount: ing.amount.toString(),
      unit: ing.unit,
      group: ing.groupName ?? '',
      categoryId: '',
      newCategoryName: ''
    })) || [{ name: '', amount: '', unit: '', group: '', categoryId: '', newCategoryName: '' }]
  )

  const createRecipe = useCreateRecipe()
  const updateRecipe = useUpdateRecipe()
  const { data: itemNamesData } = useItemNames()
  const { data: categoriesData, mutate: mutateCategories } = useCategories()
  const categories = categoriesData?.categories ?? []

  // Catalog of known ingredient names (for autocomplete) and their current
  // name -> category mapping (for auto-fill and skipping redundant writes).
  const itemNameSuggestions = useMemo(
    () => (itemNamesData?.names ?? []).map((n) => n.name),
    [itemNamesData]
  )
  const nameToCategory = useMemo(() => {
    const map = new Map<string, string>()
    for (const n of itemNamesData?.names ?? []) {
      if (n.categoryId) map.set(normalizeName(n.name), n.categoryId)
    }
    return map
  }, [itemNamesData])

  // When editing, pre-fill each ingredient's category from the catalog once the
  // name list has loaded. Runs a single time so it never clobbers user edits.
  const categoriesPrefilled = useRef(false)
  useEffect(() => {
    if (categoriesPrefilled.current || !recipe?.id || nameToCategory.size === 0) return
    setIngredients((prev) =>
      prev.map((ing) =>
        ing.categoryId
          ? ing
          : { ...ing, categoryId: nameToCategory.get(normalizeName(ing.name)) ?? '' }
      )
    )
    categoriesPrefilled.current = true
  }, [nameToCategory, recipe?.id])

  const addIngredient = () =>
    setIngredients([
      ...ingredients,
      { amount: '', unit: '', name: '', group: '', categoryId: '', newCategoryName: '' }
    ])
  const removeIngredient = (index: number) =>
    setIngredients(ingredients.filter((_, i) => i !== index))
  const updateIngredient = (index: number, field: keyof IngredientRow, value: string) => {
    const updated = [...ingredients]
    updated[index] = { ...updated[index], [field]: value }
    setIngredients(updated)
  }

  // Picking a known ingredient name reuses its canonical spelling (preventing
  // near-duplicates) and auto-fills its category when one is already known.
  const selectIngredientName = (index: number, value: string) => {
    const known = nameToCategory.get(normalizeName(value))
    const updated = [...ingredients]
    updated[index] = {
      ...updated[index],
      name: value,
      categoryId: known ?? updated[index].categoryId
    }
    setIngredients(updated)
  }

  // Persist the chosen categories into the name->category catalog (shared across
  // every list and export), creating any new categories first. Mirrors the
  // shopping list add-form.
  const syncIngredientCategories = async () => {
    const client = createServiceClient(ShoppingListService)
    const createdCategories = new Map<string, string>()
    let categoryCreated = false

    const writes: Promise<unknown>[] = []
    for (const ing of ingredients) {
      const itemName = ing.name.trim()
      if (!itemName) continue

      let categoryId = ing.categoryId
      if (categoryId === NEW_CATEGORY) {
        const newName = ing.newCategoryName.trim()
        if (!newName) continue
        const key = newName.toLowerCase()
        let id = createdCategories.get(key)
        if (!id) {
          const resp = await client.createCategory({ name: newName, ownerUserId: '' })
          id = resp.category?.id ?? ''
          createdCategories.set(key, id)
          categoryCreated = true
        }
        categoryId = id
      }
      if (!categoryId) continue

      // Skip when the catalog already maps this name to the same category.
      if (nameToCategory.get(normalizeName(itemName)) === categoryId) continue
      writes.push(client.setItemCategory({ name: itemName, categoryId, ownerUserId: '' }))
    }

    await Promise.all(writes)
    if (categoryCreated) await mutateCategories()
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      const parsedBatchServings = batchServings ? parseInt(batchServings, 10) : undefined
      const ingredientPayload = {
        ingredientNames: ingredients.map((ing) => ing.name),
        ingredientAmounts: ingredients.map((ing) => parseFraction(ing.amount)),
        ingredientUnits: ingredients.map((ing) => ing.unit),
        ingredientGroupNames: ingredients.map((ing) => ing.group)
      }
      const base = {
        name,
        baseServings: parseInt(servings, 10),
        batchServings: parsedBatchServings,
        steps: steps.split('\n').filter((s) => s.trim()),
        ...ingredientPayload
      }

      let savedId: string
      if (recipe?.id) {
        const req: UpdateRecipeInput = { id: recipe.id, ...base }
        await updateRecipe(req)
        savedId = recipe.id
      } else {
        const req: CreateRecipeInput = base
        const result = await createRecipe(req)
        savedId = result.recipe?.id || ''
      }

      await syncIngredientCategories()
      onSave(savedId)
    } catch (err) {
      console.error('Failed to save recipe:', err)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div className="space-y-1.5">
        <Label>Recipe Name</Label>
        <Input type="text" value={name} onChange={(e) => setName(e.target.value)} required />
      </div>

      <div className="space-y-1.5">
        <Label>Servings</Label>
        <Input
          type="number"
          value={servings}
          onChange={(e) => setServings(e.target.value)}
          min="1"
          className="max-w-24"
        />
      </div>

      <div className="space-y-1.5">
        <Label>Batch prep servings</Label>
        <p className="text-xs text-muted-foreground">
          When set, the shopping list buys for this many servings instead of summing scheduled
          occurrences. Leave empty to use the scheduled total.
        </p>
        <Input
          type="number"
          value={batchServings}
          onChange={(e) => setBatchServings(e.target.value)}
          min="1"
          placeholder="e.g. 10"
          className="max-w-24"
        />
      </div>

      <div className="space-y-1.5">
        <Label>Ingredients</Label>
        <div className="space-y-2">
          {ingredients.map((ing, idx) => (
            <div key={idx} className="flex flex-wrap gap-2">
              <Input
                type="text"
                placeholder="e.g. 1/3"
                value={ing.amount}
                onChange={(e) => updateIngredient(idx, 'amount', e.target.value)}
                className="w-16"
              />
              <Input
                type="text"
                placeholder="Unit"
                value={ing.unit}
                onChange={(e) => updateIngredient(idx, 'unit', e.target.value)}
                className="w-20"
              />
              <Combobox
                value={ing.name}
                onChange={(value) => updateIngredient(idx, 'name', value)}
                onSelect={(value) => selectIngredientName(idx, value)}
                suggestions={itemNameSuggestions}
                placeholder="Name"
                aria-label="Name"
                className="flex-1 min-w-32"
              />
              <Input
                type="text"
                placeholder="Group (optional)"
                value={ing.group}
                onChange={(e) => updateIngredient(idx, 'group', e.target.value)}
                className="w-28"
              />
              <Select
                aria-label="Category"
                value={ing.categoryId}
                onChange={(e) => updateIngredient(idx, 'categoryId', e.target.value)}
                className="w-auto"
              >
                <option value="">-- Category --</option>
                {categories.map((category) => (
                  <option key={category.id} value={category.id}>
                    {category.name}
                  </option>
                ))}
                <option value={NEW_CATEGORY}>+ New category</option>
              </Select>
              {ing.categoryId === NEW_CATEGORY && (
                <Input
                  type="text"
                  placeholder="New category"
                  aria-label="New category name"
                  value={ing.newCategoryName}
                  onChange={(e) => updateIngredient(idx, 'newCategoryName', e.target.value)}
                  className="w-32"
                />
              )}
              {ingredients.length > 1 && (
                <Button
                  type="button"
                  variant="destructive"
                  size="sm"
                  aria-label="Remove"
                  onClick={() => removeIngredient(idx)}
                >
                  ×
                </Button>
              )}
            </div>
          ))}
        </div>
        <Button type="button" variant="secondary" size="sm" onClick={addIngredient}>
          Add Ingredient
        </Button>
      </div>

      <div className="space-y-1.5">
        <Label>Steps</Label>
        <Textarea value={steps} onChange={(e) => setSteps(e.target.value)} rows={6} />
      </div>

      <div className="flex gap-2">
        <Button type="submit" className="flex-1">
          Save Recipe
        </Button>
        <Button type="button" variant="secondary" onClick={onCancel} className="flex-1">
          Cancel
        </Button>
      </div>
    </form>
  )
}
