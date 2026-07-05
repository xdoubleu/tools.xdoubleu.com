'use client'

import { useRouter } from 'next/navigation'
import RecipeForm from '@/components/recipes/RecipeForm'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { PageContainer } from '@/components/ui/page-container'

export default function NewRecipePage() {
  const router = useRouter()

  return (
    <PageContainer className="max-w-2xl p-6">
      <Breadcrumb
        className="mb-4"
        items={[{ label: 'Recipes', href: '/recipes/list' }, { label: 'New' }]}
      />
      <h1 className="text-3xl font-bold mb-6">New Recipe</h1>
      <RecipeForm
        onSave={(id) => router.push(`/recipes/${id}`)}
        onCancel={() => router.push('/recipes/list')}
      />
    </PageContainer>
  )
}
