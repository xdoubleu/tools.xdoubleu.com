'use client'

import { useState } from 'react'
import Link from 'next/link'
import { getApiUrl } from '@/lib/env'
import { useMealPlan, useSharePlan, useUnsharePlan } from '@/hooks/useMealPlans'
import type { SharePlanInput, UnsharePlanInput } from '@/hooks/useMealPlans'
import { useRecipes } from '@/hooks/useRecipes'
import MealPlanCalendar from '@/components/recipes/MealPlanCalendar'
import ShareModal from '@/components/recipes/ShareModal'
import { Button } from '@/components/ui/button'
import { PageContainer } from '@/components/ui/page-container'

export default function MealPlanClient({ id }: { id: string }) {
  const [offset, setOffset] = useState(0)
  const { data, error, isLoading, mutate } = useMealPlan(id, offset)
  const { data: recipesData } = useRecipes()
  const sharePlan = useSharePlan()
  const unsharePlan = useUnsharePlan()

  const [showShareModal, setShowShareModal] = useState(false)
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
  const isOwner = data?.isOwner ?? false

  const handleShare = async (userId: string, canEdit: boolean) => {
    if (!plan) return
    const req: SharePlanInput = { planId: plan.id, contactUserId: userId, canEdit }
    await sharePlan(req)
    await mutate()
  }

  const handleUnshare = async (userId: string) => {
    if (!plan) return
    const req: UnsharePlanInput = { planId: plan.id, targetUserId: userId }
    await unsharePlan(req)
    await mutate()
  }

  return (
    <PageContainer className="p-6">
      {isLoading && <p>Loading meal plan…</p>}
      {error && <p className="text-danger">Failed to load meal plan.</p>}

      {plan && (
        <>
          <div className="flex items-center justify-between mb-6">
            <h1 className="text-3xl font-bold">{plan.name}</h1>
            <div className="flex gap-2">
              {data?.icalUrl && (
                <Button variant="secondary" size="sm" onClick={handleCopyIcal}>
                  {icalCopied ? 'Copied!' : 'iCal Link'}
                </Button>
              )}
              {isOwner && (
                <>
                  <Button variant="secondary" size="sm" onClick={() => setShowShareModal(true)}>
                    Share
                  </Button>
                  <Button asChild variant="secondary" size="sm">
                    <Link href={`/mealplans/${plan.id}/edit`}>Settings</Link>
                  </Button>
                </>
              )}
            </div>
          </div>

          <MealPlanCalendar
            plan={plan}
            recipes={recipes}
            weekOffset={offset}
            onPrevWeek={() => setOffset(data?.prevOffset ?? offset - 1)}
            onNextWeek={() => setOffset(data?.nextOffset ?? offset + 1)}
            onMutate={() => mutate()}
          />

          {showShareModal && (
            <ShareModal
              title="Share meal plan"
              shares={(data?.sharedWith ?? []).map((u) => ({
                userId: u.userId,
                displayName: u.displayName,
                canEdit: u.canEdit
              }))}
              onShare={handleShare}
              onUnshare={handleUnshare}
              onClose={() => setShowShareModal(false)}
            />
          )}
        </>
      )}
    </PageContainer>
  )
}
