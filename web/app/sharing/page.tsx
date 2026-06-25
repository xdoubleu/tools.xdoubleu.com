'use client'

import { useState } from 'react'
import useSWR from 'swr'
import { useRecipeBookShares, useShareRecipeBook, useUnshareRecipeBook } from '@/hooks/useRecipes'
import {
  useShoppingListShares,
  useShareShoppingList,
  useUnshareShoppingList
} from '@/hooks/useShoppingList'
import { useSharePlan, useUnsharePlan } from '@/hooks/useMealPlans'
import { createServiceClient } from '@/lib/client'
import { MealPlansService } from '@/lib/gen/mealplans/v1/mealplans_pb'
import ShareModal, { type ShareEntry } from '@/components/recipes/ShareModal'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { PageContainer } from '@/components/ui/page-container'

function SharesPreview({ shares }: { shares: ShareEntry[] }) {
  if (shares.length === 0) {
    return <p className="text-sm text-muted">Not shared with anyone yet.</p>
  }
  return (
    <ul className="space-y-1">
      {shares.map((s) => (
        <li key={s.userId} className="flex items-center gap-2 text-sm">
          {s.displayName}
          <Badge variant={s.canEdit ? 'success' : 'secondary'}>
            {s.canEdit ? 'Can edit' : 'View only'}
          </Badge>
        </li>
      ))}
    </ul>
  )
}

function SharingCard({
  title,
  shares,
  onManage
}: {
  title: string
  shares: ShareEntry[]
  onManage: () => void
}) {
  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between gap-2">
          <CardTitle>{title}</CardTitle>
          <Button variant="secondary" size="sm" onClick={onManage}>
            Manage
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        <SharesPreview shares={shares} />
      </CardContent>
    </Card>
  )
}

// Owned meal plans plus the users each is shared with. GetPlan returns
// sharedWith and isOwner regardless of the week offset, so offset 0 suffices.
function useOwnedPlanShares() {
  const client = createServiceClient(MealPlansService)
  return useSWR('/sharing/mealplans', async () => {
    const { plans } = await client.listPlans({})
    const detailed = await Promise.all(plans.map((p) => client.getPlan({ id: p.id, offset: 0 })))
    return detailed
      .filter((d) => d.isOwner && d.plan)
      .map((d) => ({
        id: d.plan!.id,
        name: d.plan!.name,
        shares: d.sharedWith.map((u) => ({
          userId: u.userId,
          displayName: u.displayName,
          canEdit: u.canEdit
        }))
      }))
  })
}

export default function SharingPage() {
  const { data: bookShares, mutate: mutateBook } = useRecipeBookShares()
  const shareBook = useShareRecipeBook()
  const unshareBook = useUnshareRecipeBook()

  const { data: listShares, mutate: mutateList } = useShoppingListShares()
  const shareList = useShareShoppingList()
  const unshareList = useUnshareShoppingList()

  const { data: planShares, mutate: mutatePlans } = useOwnedPlanShares()
  const sharePlan = useSharePlan()
  const unsharePlan = useUnsharePlan()

  const [open, setOpen] = useState<string | null>(null)

  const bookEntries: ShareEntry[] = bookShares?.shares ?? []
  const listEntries: ShareEntry[] = listShares?.shares ?? []

  return (
    <PageContainer className="p-6">
      <h1 className="mb-6 text-3xl font-bold">Sharing</h1>

      <div className="space-y-6">
        <SharingCard title="Recipe book" shares={bookEntries} onManage={() => setOpen('book')} />
        <SharingCard title="Shopping list" shares={listEntries} onManage={() => setOpen('list')} />
        {(planShares ?? []).map((plan) => (
          <SharingCard
            key={plan.id}
            title={`Meal plan: ${plan.name}`}
            shares={plan.shares}
            onManage={() => setOpen(`plan:${plan.id}`)}
          />
        ))}
      </div>

      {open === 'book' && (
        <ShareModal
          title="Share recipe book"
          shares={bookEntries}
          onShare={async (id, canEdit) => {
            await shareBook(id, canEdit)
            await mutateBook()
          }}
          onUnshare={async (id) => {
            await unshareBook(id)
            await mutateBook()
          }}
          onClose={() => setOpen(null)}
        />
      )}

      {open === 'list' && (
        <ShareModal
          title="Share shopping list"
          shares={listEntries}
          onShare={async (id, canEdit) => {
            await shareList(id, canEdit)
            await mutateList()
          }}
          onUnshare={async (id) => {
            await unshareList(id)
            await mutateList()
          }}
          onClose={() => setOpen(null)}
        />
      )}

      {(planShares ?? []).map(
        (plan) =>
          open === `plan:${plan.id}` && (
            <ShareModal
              key={plan.id}
              title={`Share meal plan: ${plan.name}`}
              shares={plan.shares}
              onShare={async (id, canEdit) => {
                await sharePlan({ planId: plan.id, contactUserId: id, canEdit })
                await mutatePlans()
              }}
              onUnshare={async (id) => {
                await unsharePlan({ planId: plan.id, targetUserId: id })
                await mutatePlans()
              }}
              onClose={() => setOpen(null)}
            />
          )
      )}
    </PageContainer>
  )
}
