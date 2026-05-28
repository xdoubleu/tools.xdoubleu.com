'use client'

import Link from 'next/link'
import { useMealPlans } from '@/hooks/useMealPlans'
import type { Plan } from '@/lib/gen/mealplans/v1/mealplans_pb'

function PlanCard({ plan }: { plan: Plan }) {
  return (
    <Link
      href={`/mealplans/${plan.id}`}
      className="block rounded-2xl border border-border bg-card p-4 shadow-card transition-colors hover:shadow-elevated"
    >
      <h2 className="font-semibold text-lg">{plan.name}</h2>
      <p className="text-sm text-muted mt-1">
        {plan.meals.length} meal{plan.meals.length !== 1 ? 's' : ''}
      </p>
      <p className="text-xs text-muted mt-1">
        Created {new Date(plan.createdAt).toLocaleDateString()}
      </p>
    </Link>
  )
}

export default function PlansPage() {
  const { data, error, isLoading } = useMealPlans()

  return (
    <main className="max-w-4xl mx-auto p-6">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-3xl font-bold">Meal Plans</h1>
        <div className="flex gap-2">
          <Link
            href="/recipes/list"
            className="px-4 py-2 bg-surface border border-border rounded hover:bg-border text-sm"
          >
            Recipes
          </Link>
          <Link
            href="/mealplans/new"
            className="rounded-xl bg-accent px-4 py-2 text-sm text-white hover:bg-accent-hover"
          >
            New Plan
          </Link>
        </div>
      </div>

      {isLoading && <p>Loading plans...</p>}
      {error && <p className="text-danger">Failed to load meal plans.</p>}
      {data && data.plans.length === 0 && (
        <p className="text-muted">No meal plans yet. Create your first one!</p>
      )}
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
