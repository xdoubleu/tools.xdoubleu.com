'use client'

import { useEffect } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { useMealPlans } from '@/hooks/useMealPlans'
import { PageContainer } from '@/components/ui/page-container'

export default function PlansListClient() {
  const { data, error, isLoading } = useMealPlans()
  const router = useRouter()

  useEffect(() => {
    if (data?.plans && data.plans.length > 0) {
      router.replace(`/mealplans/${data.plans[0].id}`)
    }
  }, [data, router])

  return (
    <PageContainer className="p-6">
      <h1 className="text-3xl font-bold mb-6">Meal Plan</h1>

      {isLoading && <p>Loading...</p>}
      {error && <p className="text-danger">Failed to load meal plan.</p>}
      {data && data.plans.length === 0 && (
        <div>
          <p className="text-muted mb-4">You don&apos;t have a meal plan yet.</p>
          <Link
            href="/mealplans/new"
            className="rounded-xl bg-accent px-4 py-2 text-sm text-white hover:bg-accent-hover"
          >
            Create Meal Plan
          </Link>
        </div>
      )}
    </PageContainer>
  )
}
