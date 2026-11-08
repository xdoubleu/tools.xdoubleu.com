import { useState } from 'react'
import { getApiUrl } from '@/lib/env'
import { useCopyFeedback } from '@/hooks/useCopyFeedback'
import { useMealPlan } from '@/hooks/useMealPlans'
import { useAllRecipes } from '@/hooks/useRecipes'
import MealPlanCalendar from '@/components/mealplans/MealPlanCalendar'
import { Button } from '@/components/ui/button'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader, PageHeaderLink } from '@/components/ui/page-header'
import { ErrorState, LoadingState } from '@/components/ui/states'

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
    <PageContainer>
      {isLoading && <LoadingState placement="page" label="meal plan" />}
      {error && <ErrorState placement="page" what="meal plan" />}

      {plan && (
        <>
          <PageHeader
            title={plan.name}
            breadcrumb={[{ label: 'Meal Plans', href: '/mealplans' }, { label: plan.name }]}
            actions={
              <>
                {data?.icalUrl && (
                  <Button variant="secondary" onClick={handleCopyIcal}>
                    {icalCopied ? 'Copied!' : 'iCal Link'}
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
            onMutate={() => mutate()}
          />
        </>
      )}
    </PageContainer>
  )
}
