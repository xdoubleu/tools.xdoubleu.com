'use client'

import Link from 'next/link'
import { useRecipe } from '@/hooks/useRecipes'
import type { Ingredient } from '@/lib/gen/recipes/v1/recipes_pb'

function IngredientRow({ ingredient }: { ingredient: Ingredient }) {
  return (
    <li className="flex gap-2 py-1 border-b last:border-0 border-border">
      <span className="font-medium">
        {ingredient.amount} {ingredient.unit}
      </span>
      <span>{ingredient.name}</span>
    </li>
  )
}

export default function RecipeClient({ id }: { id: string }) {
  const { data, error, isLoading } = useRecipe(id)

  return (
    <main className="max-w-2xl mx-auto p-6">
      <Link href="/recipes" className="text-blue-600 hover:underline text-sm mb-4 block">
        &larr; Back to recipes
      </Link>

      {isLoading && <p>Loading recipe...</p>}
      {error && <p className="text-red-600">Failed to load recipe.</p>}
      {data?.recipe && (
        <>
          <h1 className="text-3xl font-bold mb-2">{data.recipe.name}</h1>
          <p className="text-muted text-sm mb-6">Serves {data.recipe.baseServings}</p>

          {data.recipe.ingredients.length > 0 && (
            <section className="mb-6">
              <h2 className="text-xl font-semibold mb-3">Ingredients</h2>
              <ul>
                {data.recipe.ingredients
                  .slice()
                  .sort((a, b) => a.sortOrder - b.sortOrder)
                  .map((ing) => (
                    <IngredientRow key={ing.id} ingredient={ing} />
                  ))}
              </ul>
            </section>
          )}

          {data.recipe.instructions && (
            <section>
              <h2 className="text-xl font-semibold mb-3">Instructions</h2>
              <div className="prose max-w-none whitespace-pre-wrap text-subtle">
                {data.recipe.instructions}
              </div>
            </section>
          )}
        </>
      )}
    </main>
  )
}
