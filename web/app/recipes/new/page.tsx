'use client'

import { useRouter } from 'next/navigation'
import RecipeForm from '@/components/recipes/RecipeForm'
import { Breadcrumb } from '@/components/ui/breadcrumb'

export default function NewRecipePage() {
  const router = useRouter()

  return (
    <main className="max-w-2xl mx-auto p-6">
      <Breadcrumb
        className="mb-4"
        items={[{ label: 'Recipes', href: '/recipes/list' }, { label: 'New' }]}
      />
      <h1 className="text-3xl font-bold mb-6">New Recipe</h1>
      <RecipeForm
        onSave={(id) => router.push(`/recipes/${id}`)}
        onCancel={() => router.push('/recipes/list')}
      />
    </main>
  )
}
