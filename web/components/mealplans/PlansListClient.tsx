'use client'

import { useEffect } from 'react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { useMealPlans } from '@/hooks/useMealPlans'
import { Button } from '@/components/ui/button'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'

export default function PlansListClient() {
  const { data, error, isLoading } = useMealPlans()
  const router = useRouter()

  useEffect(() => {
    if (data?.plans && data.plans.length > 0) {
      router.replace(`/mealplans/${data.plans[0].id}`)
    }
  }, [data, router])

  return (
    <PageContainer>
      <PageHeader title="Meal Plan" />

      {isLoading && <LoadingState />}
      {error && <ErrorState what="meal plan" />}
      {data && data.plans.length === 0 && (
        <EmptyState
          action={
            <Button asChild>
              <Link href="/mealplans/new">Create Meal Plan</Link>
            </Button>
          }
        >
          You don&apos;t have a meal plan yet.
        </EmptyState>
      )}
    </PageContainer>
  )
}
