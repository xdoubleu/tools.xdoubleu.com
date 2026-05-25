'use client'

import { useRouter } from 'next/navigation'
import Link from 'next/link'
import RecipeForm from '@/components/recipes/RecipeForm'

export default function NewRecipePage() {
  const router = useRouter()

  return (
    <main className="max-w-2xl mx-auto p-6">
      <div className="flex items-center gap-4 mb-6">
        <Link href="/recipes/list" className="text-blue-600 hover:underline text-sm">
          &larr; Recipes
        </Link>
        <h1 className="text-3xl font-bold">New Recipe</h1>
      </div>
      <RecipeForm
        onSave={(id) => router.push(`/recipes/${id}`)}
        onCancel={() => router.push('/recipes/list')}
      />
    </main>
  )
}
