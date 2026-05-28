'use client'

import Link from 'next/link'
import { useRecipes } from '@/hooks/useRecipes'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'

function RecipeCard({ recipe }: { recipe: Recipe }) {
  return (
    <Link
      href={`/recipes/${recipe.id}`}
      className="block border border-border rounded p-4 hover:bg-surface transition-colors"
    >
      <h2 className="font-semibold text-lg">{recipe.name}</h2>
      <p className="text-sm text-muted mt-1">
        {recipe.ingredients.length} ingredient
        {recipe.ingredients.length !== 1 ? 's' : ''}
      </p>
      <p className="text-sm text-muted mt-1">Serves {recipe.baseServings}</p>
    </Link>
  )
}

export default function RecipesListPage() {
  const { data, error, isLoading } = useRecipes()

  return (
    <main className="max-w-4xl mx-auto p-6">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-3xl font-bold">Recipes</h1>
        <div className="flex gap-2">
          <Link
            href="/recipes/plans"
            className="px-4 py-2 bg-surface border border-border rounded hover:bg-border text-sm"
          >
            Meal Plans
          </Link>
          <Link
            href="/recipes/new"
            className="rounded-xl bg-accent px-4 py-2 text-sm text-white hover:bg-accent-hover"
          >
            New Recipe
          </Link>
        </div>
      </div>

      {isLoading && <p>Loading recipes...</p>}
      {error && <p className="text-danger">Failed to load recipes.</p>}
      {data && data.recipes.length === 0 && (
        <p className="text-muted">No recipes yet. Create your first one!</p>
      )}
      {data && data.recipes.length > 0 && (
        <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {data.recipes.map((recipe) => (
            <RecipeCard key={recipe.id} recipe={recipe} />
          ))}
        </div>
      )}
    </main>
  )
}
