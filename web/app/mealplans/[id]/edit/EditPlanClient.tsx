'use client'

import { useRouter } from 'next/navigation'
import { useMealPlan } from '@/hooks/useMealPlans'
import PlanForm from '@/components/recipes/PlanForm'
import { Breadcrumb } from '@/components/ui/breadcrumb'

export default function EditPlanClient({ id }: { id: string }) {
  const { data, isLoading, error } = useMealPlan(id)
  const router = useRouter()
  const plan = data?.plan

  return (
    <main className="max-w-2xl mx-auto p-6">
      <Breadcrumb
        className="mb-4"
        items={[
          { label: 'Meal Plans', href: '/mealplans' },
          { label: plan?.name ?? 'Plan', href: `/mealplans/${id}` },
          { label: 'Settings' }
        ]}
      />
      <h1 className="text-3xl font-bold mb-6">Settings</h1>

      {isLoading && <p>Loading plan…</p>}
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
