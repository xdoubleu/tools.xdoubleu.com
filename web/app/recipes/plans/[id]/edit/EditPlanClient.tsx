'use client'

import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { useMealPlan } from '@/hooks/useRecipes'
import PlanForm from '@/components/recipes/PlanForm'

export default function EditPlanClient({ id }: { id: string }) {
  const { data, isLoading, error } = useMealPlan(id)
  const router = useRouter()
  const plan = data?.plan

  return (
    <main className="max-w-2xl mx-auto p-6">
      <div className="flex items-center gap-4 mb-6">
        <Link href={`/recipes/plans/${id}`} className="text-blue-600 hover:underline text-sm">
          &larr; Back to Plan
        </Link>
        <h1 className="text-3xl font-bold">Edit Meal Plan</h1>
      </div>

      {isLoading && <p>Loading plan...</p>}
      {error && <p className="text-red-600">Failed to load plan.</p>}
      {plan && (
        <PlanForm
          plan={plan}
          onSave={(planId) => router.push(`/recipes/plans/${planId}`)}
          onCancel={() => router.push(`/recipes/plans/${id}`)}
        />
      )}
    </main>
  )
}
