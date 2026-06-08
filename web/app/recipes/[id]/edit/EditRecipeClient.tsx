'use client'

import { useRouter } from 'next/navigation'
import { useRecipe } from '@/hooks/useRecipes'
import RecipeForm from '@/components/recipes/RecipeForm'
import { Breadcrumb } from '@/components/ui/breadcrumb'

export default function EditRecipeClient({ id }: { id: string }) {
  const { data, isLoading, error } = useRecipe(id)
  const router = useRouter()
  const recipe = data?.recipe

  return (
    <main className="max-w-2xl mx-auto p-6">
      <Breadcrumb
        className="mb-4"
        items={[
          { label: 'Recipes', href: '/recipes/list' },
          { label: recipe?.name ?? 'Recipe', href: `/recipes/${id}` },
          { label: 'Edit' }
        ]}
      />
      <h1 className="text-3xl font-bold mb-6">Edit Recipe</h1>
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
