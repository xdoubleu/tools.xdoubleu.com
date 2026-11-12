import { useState } from 'react'
import { getApiUrl } from '@/lib/env'
import { useCopyFeedback } from '@/hooks/useCopyFeedback'
import { useMealPlan } from '@/hooks/useMealPlans'
import { useAllRecipes } from '@/hooks/useRecipes'
import MealPlanCalendar from '@/components/mealplans/MealPlanCalendar'
import { Button } from '@/components/ui/button'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader, PageHeaderLink } from '@/components/ui/page-header'
import { Skeleton } from '@/components/ui/skeleton'
import { ErrorState } from '@/components/ui/states'

// Matches the loaded page: title row, week navigation, and a week of day cards.
function MealPlanSkeleton() {
  return (
    <div role="status" aria-label="Loading meal plan" className="space-y-4">
      <Skeleton className="h-10 w-1/2" />
      <Skeleton className="h-9 w-full" />
      <div className="space-y-3">
        {Array.from({ length: 7 }, (_, i) => (
          <Skeleton key={i} className="h-48" />
        ))}
      </div>
    </div>
  )
}

export default function MealPlanClient({ id }: { id: string }) {
  const [offset, setOffset] = useState(0)
  const { data, error, isLoading, mutate } = useMealPlan(id, offset)
  const { data: recipesData } = useAllRecipes()

  const { copy, copied: icalCopied } = useCopyFeedback()

  const handleCopyIcal = () => {
    if (!data?.icalUrl) return
    void copy(`${getApiUrl()}${data.icalUrl}`)
  }

  const plan = data?.plan
  const recipes = recipesData ?? []

  return (
    <PageContainer size="wide">
      {isLoading && <MealPlanSkeleton />}
      {error && <ErrorState placement="page" what="meal plan" />}

      {plan && (
        <>
          <PageHeader
            title={plan.name}
            breadcrumb={[{ label: 'Meal plans', href: '/mealplans' }, { label: plan.name }]}
            actions={
              <>
                {data?.icalUrl && (
                  <Button variant="secondary" onClick={handleCopyIcal}>
                    {icalCopied ? 'Copied!' : 'iCal link'}
                  </Button>
                )}
                <PageHeaderLink href={`/mealplans/${plan.id}/edit`}>Edit plan</PageHeaderLink>
              </>
            }
          />

          <MealPlanCalendar
            plan={plan}
            recipes={recipes}
            weekOffset={offset}
            onPrevWeek={() => setOffset(data?.prevOffset ?? offset - 1)}
            onNextWeek={() => setOffset(data?.nextOffset ?? offset + 1)}
            onThisWeek={() => setOffset(0)}
            onMutate={() => mutate()}
          />
        </>
      )}
    </PageContainer>
  )
}
