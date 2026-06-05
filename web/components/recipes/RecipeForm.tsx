'use client'

import { useState } from 'react'
import { useCreateRecipe, useUpdateRecipe } from '@/hooks/useRecipes'
import type { CreateRecipeInput, UpdateRecipeInput } from '@/hooks/useRecipes'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'
import { parseFraction } from '@/lib/recipes/parseFraction'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'

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
}

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
      group: ing.groupName ?? ''
    })) || [{ name: '', amount: '', unit: '', group: '' }]
  )

  const createRecipe = useCreateRecipe()
  const updateRecipe = useUpdateRecipe()

  const addIngredient = () =>
    setIngredients([...ingredients, { amount: '', unit: '', name: '', group: '' }])
  const removeIngredient = (index: number) =>
    setIngredients(ingredients.filter((_, i) => i !== index))
  const updateIngredient = (index: number, field: string, value: string) => {
    const updated = [...ingredients]
    updated[index] = { ...updated[index], [field]: value }
    setIngredients(updated)
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    try {
      const parsedBatchServings = batchServings ? parseInt(batchServings, 10) : undefined
      if (recipe?.id) {
        const req: UpdateRecipeInput = {
          id: recipe.id,
          name,
          baseServings: parseInt(servings, 10),
          batchServings: parsedBatchServings,
          steps: steps.split('\n').filter((s) => s.trim()),
          ingredientNames: ingredients.map((ing) => ing.name),
          ingredientAmounts: ingredients.map((ing) => parseFraction(ing.amount)),
          ingredientUnits: ingredients.map((ing) => ing.unit),
          ingredientGroupNames: ingredients.map((ing) => ing.group)
        }
        await updateRecipe(req)
        onSave(recipe.id)
      } else {
        const req: CreateRecipeInput = {
          name,
          baseServings: parseInt(servings, 10),
          batchServings: parsedBatchServings,
          steps: steps.split('\n').filter((s) => s.trim()),
          ingredientNames: ingredients.map((ing) => ing.name),
          ingredientAmounts: ingredients.map((ing) => parseFraction(ing.amount)),
          ingredientUnits: ingredients.map((ing) => ing.unit),
          ingredientGroupNames: ingredients.map((ing) => ing.group)
        }
        const result = await createRecipe(req)
        onSave(result.recipe?.id || '')
      }
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
            <div key={idx} className="flex gap-2">
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
              <Input
                type="text"
                placeholder="Name"
                value={ing.name}
                onChange={(e) => updateIngredient(idx, 'name', e.target.value)}
                className="flex-1"
              />
              <Input
                type="text"
                placeholder="Group (optional)"
                value={ing.group}
                onChange={(e) => updateIngredient(idx, 'group', e.target.value)}
                className="w-28"
              />
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
