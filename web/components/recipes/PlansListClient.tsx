'use client'

import { useEffect } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { useMealPlans } from '@/hooks/useMealPlans'
import { Button } from '@/components/ui/button'
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

      {isLoading && <p className="text-muted">Loading…</p>}
      {error && <p className="text-danger">Failed to load meal plan.</p>}
      {data && data.plans.length === 0 && (
        <div>
          <p className="text-muted mb-4">You don&apos;t have a meal plan yet.</p>
          <Button asChild>
            <Link href="/mealplans/new">Create Meal Plan</Link>
          </Button>
        </div>
      )}
    </PageContainer>
  )
}
