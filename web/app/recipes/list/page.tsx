'use client'

import Link from 'next/link'
import { useRecipes } from '@/hooks/useRecipes'
import type { Recipe } from '@/lib/gen/recipes/v1/recipes_pb'
import { cn } from '@/lib/cn'
import { Button } from '@/components/ui/button'
import { interactiveCardClass } from '@/components/ui/card'

function RecipeCard({ recipe }: { recipe: Recipe }) {
  return (
    <Link href={`/recipes/${recipe.id}`} className={cn(interactiveCardClass, 'block p-4')}>
      <h2 className="font-semibold text-lg">{recipe.name}</h2>
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
        <Button asChild>
          <Link href="/recipes/new">New Recipe</Link>
        </Button>
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
