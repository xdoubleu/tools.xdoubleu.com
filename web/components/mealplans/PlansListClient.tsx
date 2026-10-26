import { useEffect } from 'react'
import { useRouter } from '@/hooks/navigation'
import { useMealPlans } from '@/hooks/useMealPlans'
import { Button } from '@/components/ui/button'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'

export default function PlansListClient() {
  const { data, error, isLoading, mutate } = useMealPlans()
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
      {/* Listing creates the first plan, so a retry fills an empty list. */}
      {data && data.plans.length === 0 && (
        <EmptyState action={<Button onClick={() => void mutate()}>Create Meal Plan</Button>}>
          You don&apos;t have a meal plan yet.
        </EmptyState>
      )}
    </PageContainer>
  )
}
