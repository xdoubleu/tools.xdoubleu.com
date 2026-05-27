'use client'

import { useState } from 'react'
import { useCreateRecipe, useUpdateRecipe } from '@/hooks/useRecipes'
import type { CreateRecipeInput, UpdateRecipeInput } from '@/hooks/useRecipes'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'
import { parseFraction } from '@/lib/recipes/parseFraction'

interface RecipeFormProps {
  recipe?: Recipe
  onSave: (id: string) => void
  onCancel: () => void
}

interface IngredientRow {
  name: string
  amount: string
  unit: string
}

export default function RecipeForm({ recipe, onSave, onCancel }: RecipeFormProps) {
  const [name, setName] = useState(recipe?.name || '')
  const [servings, setServings] = useState(recipe?.baseServings?.toString() || '1')
  const [steps, setSteps] = useState(recipe?.instructions || '')
  const [ingredients, setIngredients] = useState<IngredientRow[]>(
    recipe?.ingredients?.map((ing) => ({
      name: ing.name,
      amount: ing.amount.toString(),
      unit: ing.unit
    })) || [{ name: '', amount: '', unit: '' }]
  )

  const createRecipe = useCreateRecipe()
  const updateRecipe = useUpdateRecipe()

  const addIngredient = () => {
    setIngredients([...ingredients, { amount: '', unit: '', name: '' }])
  }

  const removeIngredient = (index: number) => {
    setIngredients(ingredients.filter((_, i) => i !== index))
  }

  const updateIngredient = (index: number, field: string, value: string) => {
    const updated = [...ingredients]
    updated[index] = { ...updated[index], [field]: value }
    setIngredients(updated)
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()

    try {
      if (recipe?.id) {
        const req: UpdateRecipeInput = {
          id: recipe.id,
          name,
          baseServings: parseInt(servings, 10),
          steps: steps.split('\n').filter((s) => s.trim()),
          ingredientNames: ingredients.map((ing) => ing.name),
          ingredientAmounts: ingredients.map((ing) => parseFraction(ing.amount)),
          ingredientUnits: ingredients.map((ing) => ing.unit)
        }
        await updateRecipe(req)
        onSave(recipe.id)
      } else {
        const req: CreateRecipeInput = {
          name,
          baseServings: parseInt(servings, 10),
          steps: steps.split('\n').filter((s) => s.trim()),
          ingredientNames: ingredients.map((ing) => ing.name),
          ingredientAmounts: ingredients.map((ing) => parseFraction(ing.amount)),
          ingredientUnits: ingredients.map((ing) => ing.unit)
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
      <div>
        <label className="block text-sm font-medium text-subtle mb-1">Recipe Name</label>
        <input
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
          className="w-full px-3 py-2 rounded border border-input-border bg-input text-input-text focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
      </div>

      <div>
        <label className="block text-sm font-medium text-subtle mb-1">Servings</label>
        <input
          type="number"
          value={servings}
          onChange={(e) => setServings(e.target.value)}
          min="1"
          className="w-full px-3 py-2 rounded border border-input-border bg-input text-input-text focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
      </div>

      <div>
        <label className="block text-sm font-medium text-subtle mb-1">Ingredients</label>
        <div className="space-y-2">
          {ingredients.map((ing, idx) => (
            <div key={idx} className="flex gap-2">
              <input
                type="text"
                placeholder="e.g. 1/3"
                value={ing.amount}
                onChange={(e) => updateIngredient(idx, 'amount', e.target.value)}
                className="w-16 px-2 py-1 rounded border border-input-border bg-input text-input-text"
              />
              <input
                type="text"
                placeholder="Unit"
                value={ing.unit}
                onChange={(e) => updateIngredient(idx, 'unit', e.target.value)}
                className="w-20 px-2 py-1 rounded border border-input-border bg-input text-input-text"
              />
              <input
                type="text"
                placeholder="Name"
                value={ing.name}
                onChange={(e) => updateIngredient(idx, 'name', e.target.value)}
                className="flex-1 px-2 py-1 rounded border border-input-border bg-input text-input-text"
              />
              {ingredients.length > 1 && (
                <button
                  type="button"
                  onClick={() => removeIngredient(idx)}
                  className="px-2 py-1 bg-red-600 text-white text-sm rounded hover:bg-red-700"
                >
                  Remove
                </button>
              )}
            </div>
          ))}
        </div>
        <button
          type="button"
          onClick={addIngredient}
          className="mt-2 px-3 py-1 bg-subtle text-bg text-sm rounded hover:bg-fg"
        >
          Add Ingredient
        </button>
      </div>

      <div>
        <label className="block text-sm font-medium text-subtle mb-1">Steps</label>
        <textarea
          value={steps}
          onChange={(e) => setSteps(e.target.value)}
          rows={6}
          className="w-full px-3 py-2 rounded border border-input-border bg-input text-input-text focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
      </div>

      <div className="flex gap-2">
        <button
          type="submit"
          className="flex-1 px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700"
        >
          Save Recipe
        </button>
        <button
          type="button"
          onClick={onCancel}
          className="flex-1 px-4 py-2 bg-subtle text-bg rounded hover:bg-fg"
        >
          Cancel
        </button>
      </div>
    </form>
  )
}
