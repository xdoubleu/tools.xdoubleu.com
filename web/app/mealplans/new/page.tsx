'use client'

import { useRouter } from 'next/navigation'
import Link from 'next/link'
import PlanForm from '@/components/recipes/PlanForm'

export default function NewPlanPage() {
  const router = useRouter()

  return (
    <main className="max-w-2xl mx-auto p-6">
      <div className="flex items-center gap-4 mb-6">
        <Link href="/mealplans" className="text-blue-600 hover:underline text-sm">
          &larr; Meal Plans
        </Link>
        <h1 className="text-3xl font-bold">New Meal Plan</h1>
      </div>
      <PlanForm
        onSave={(id) => router.push(`/mealplans/${id}`)}
        onCancel={() => router.push('/mealplans')}
      />
    </main>
  )
}
