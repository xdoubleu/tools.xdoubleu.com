'use client'

import Link from 'next/link'
import { useMealPlans } from '@/hooks/useRecipes'
import type { Plan } from '@/lib/gen/recipes/v1/mealplans_pb'

function PlanCard({ plan }: { plan: Plan }) {
  return (
    <div className="border border-border rounded p-4">
      <h2 className="font-semibold text-lg">{plan.name}</h2>
      <p className="text-sm text-muted mt-1">
        {plan.meals.length} meal{plan.meals.length !== 1 ? 's' : ''}
      </p>
      <p className="text-xs text-muted mt-1">
        Created {new Date(plan.createdAt).toLocaleDateString()}
      </p>
    </div>
  )
}

export default function PlansPage() {
  const { data, error, isLoading } = useMealPlans()

  return (
    <main className="max-w-4xl mx-auto p-6">
      <div className="flex items-center gap-4 mb-6">
        <Link href="/recipes" className="text-blue-600 hover:underline text-sm">
          &larr; Recipes
        </Link>
        <h1 className="text-3xl font-bold">Meal Plans</h1>
      </div>

      {isLoading && <p>Loading plans...</p>}
      {error && <p className="text-red-600">Failed to load meal plans.</p>}
      {data && data.plans.length === 0 && <p className="text-muted">No meal plans yet.</p>}
      {data && data.plans.length > 0 && (
        <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {data.plans.map((plan) => (
            <PlanCard key={plan.id} plan={plan} />
          ))}
        </div>
      )}
    </main>
  )
}
