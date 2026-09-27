'use client'

import { useRouter } from 'next/navigation'
import RecipeForm from '@/components/recipes/RecipeForm'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'

export default function NewRecipePage() {
  const router = useRouter()

  return (
    <PageContainer size="form">
      <PageHeader
        title="New Recipe"
        breadcrumb={[{ label: 'Recipes', href: '/recipes/list' }, { label: 'New' }]}
      />
      <RecipeForm
        onSave={(id) => router.push(`/recipes/${id}`)}
        onCancel={() => router.push('/recipes/list')}
      />
    </PageContainer>
  )
}
