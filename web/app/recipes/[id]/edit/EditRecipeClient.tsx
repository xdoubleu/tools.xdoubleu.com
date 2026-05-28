'use client'

import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { useRecipe } from '@/hooks/useRecipes'
import RecipeForm from '@/components/recipes/RecipeForm'

export default function EditRecipeClient({ id }: { id: string }) {
  const { data, isLoading, error } = useRecipe(id)
  const router = useRouter()
  const recipe = data?.recipe

  return (
    <main className="max-w-2xl mx-auto p-6">
      <div className="flex items-center gap-4 mb-6">
        <Link href={`/recipes/${id}`} className="text-sm text-accent hover:underline">
          &larr; Back to Recipe
        </Link>
        <h1 className="text-3xl font-bold">Edit Recipe</h1>
      </div>
      {isLoading && <p>Loading recipe...</p>}
      {error && <p className="text-danger">Failed to load recipe.</p>}
      {recipe && (
        <RecipeForm
          recipe={recipe}
          onSave={(savedId) => router.push(`/recipes/${savedId}`)}
          onCancel={() => router.push(`/recipes/${id}`)}
        />
      )}
    </main>
  )
}
