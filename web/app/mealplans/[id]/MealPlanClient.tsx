'use client'

import { useState } from 'react'
import Link from 'next/link'
import { getApiUrl } from '@/lib/env'
import { useMealPlan } from '@/hooks/useMealPlans'
import { useRecipes } from '@/hooks/useRecipes'
import MealPlanCalendar from '@/components/mealplans/MealPlanCalendar'
import { Button } from '@/components/ui/button'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { ErrorState, LoadingState } from '@/components/ui/states'

export default function MealPlanClient({ id }: { id: string }) {
  const [offset, setOffset] = useState(0)
  const { data, error, isLoading, mutate } = useMealPlan(id, offset)
  const { data: recipesData } = useRecipes()

  const [icalCopied, setIcalCopied] = useState(false)

  const handleCopyIcal = () => {
    if (!data?.icalUrl) return
    const url = `${getApiUrl()}${data.icalUrl}`
    navigator.clipboard.writeText(url).then(() => {
      setIcalCopied(true)
      setTimeout(() => setIcalCopied(false), 2000)
    })
  }

  const plan = data?.plan
  const recipes = recipesData?.recipes ?? []

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
                <Button asChild variant="secondary" size="sm">
                  <Link href={`/mealplans/${plan.id}/edit`}>Settings</Link>
                </Button>
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
