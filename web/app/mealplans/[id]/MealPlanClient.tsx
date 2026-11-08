import { useState } from 'react'
import { getApiUrl } from '@/lib/env'
import { useCopyFeedback } from '@/hooks/useCopyFeedback'
import { useMealPlan } from '@/hooks/useMealPlans'
import { useAllRecipes } from '@/hooks/useRecipes'
import MealPlanCalendar from '@/components/mealplans/MealPlanCalendar'
import { Button } from '@/components/ui/button'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader, PageHeaderSettingsLink } from '@/components/ui/page-header'
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
      {isLoading && <LoadingState label="meal plan" />}
      {error && <ErrorState what="meal plan" />}

      {plan && (
        <>
          <PageHeader
            title={plan.name}
            actions={
              <>
                {data?.icalUrl && (
                  <Button variant="secondary" size="sm" onClick={handleCopyIcal}>
                    {icalCopied ? 'Copied!' : 'iCal Link'}
                  </Button>
                )}
                <PageHeaderSettingsLink href={`/mealplans/${plan.id}/edit`} />
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
