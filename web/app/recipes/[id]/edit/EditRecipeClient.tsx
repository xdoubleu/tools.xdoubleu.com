'use client'

import { useRouter } from 'next/navigation'
import { useRecipe } from '@/hooks/useRecipes'
import RecipeForm from '@/components/recipes/RecipeForm'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { ErrorState, LoadingState } from '@/components/ui/states'

export default function EditRecipeClient({ id }: { id: string }) {
  const { data, isLoading, error } = useRecipe(id)
  const router = useRouter()
  const recipe = data?.recipe

  return (
    <PageContainer size="form">
      <PageHeader
        title="Edit Recipe"
        breadcrumb={[
          { label: 'Recipes', href: '/recipes/list' },
          { label: recipe?.name ?? 'Recipe', href: `/recipes/${id}` },
          { label: 'Edit' }
        ]}
      />
      {isLoading && <LoadingState label="recipe" />}
      {error && <ErrorState what="recipe" />}
      {recipe && (
        <RecipeForm
          recipe={recipe}
          onSave={(savedId) => router.push(`/recipes/${savedId}`)}
          onCancel={() => router.push(`/recipes/${id}`)}
        />
      )}
    </PageContainer>
  )
}
