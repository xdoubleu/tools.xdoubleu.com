'use client'

import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { useMealPlan } from '@/hooks/useMealPlans'
import PlanForm from '@/components/recipes/PlanForm'

export default function EditPlanClient({ id }: { id: string }) {
  const { data, isLoading, error } = useMealPlan(id)
  const router = useRouter()
  const plan = data?.plan

  return (
    <main className="max-w-2xl mx-auto p-6">
      <div className="flex items-center gap-4 mb-6">
        <Link href={`/mealplans/${id}`} className="text-sm text-accent hover:underline">
          &larr; Back to Plan
        </Link>
        <h1 className="text-3xl font-bold">Settings</h1>
      </div>

      {isLoading && <p>Loading plan...</p>}
      {error && <p className="text-danger">Failed to load plan.</p>}
      {plan && (
        <PlanForm
          plan={plan}
          onSave={(planId) => router.push(`/mealplans/${planId}`)}
          onCancel={() => router.push(`/mealplans/${id}`)}
        />
      )}
    </main>
  )
}
