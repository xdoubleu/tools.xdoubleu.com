'use client'

import { useMemo, useState } from 'react'
import Link from 'next/link'
import {
  useCustomList,
  useCategories,
  useAccessibleLists,
  useShoppingListShares,
  useShareShoppingList,
  useUnshareShoppingList,
  useAllMealPlanExportItems,
  useAllPlanIngredientGroups
} from '@/hooks/useShoppingList'
import ShoppingList from '@/components/recipes/ShoppingList'
import ExportModal from '@/components/recipes/ExportModal'
import ShareModal from '@/components/recipes/ShareModal'
import AddItemForm from '@/components/shoppinglist/AddItemForm'
import MealPlanGroupFilter from '@/components/shoppinglist/MealPlanGroupFilter'
import MealPlanItemsPreview from '@/components/shoppinglist/MealPlanItemsPreview'
import { Button } from '@/components/ui/button'
import { PageContainer } from '@/components/ui/page-container'
import { Select } from '@/components/ui/select'
import { createServiceClient } from '@/lib/client'
import { ShoppingListService } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'
import type { ShoppingItem as ShoppingItemExport } from '@/lib/recipes/shoppingExport'
import type { ShoppingItem } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'

function toExportItem(item: ShoppingItem): ShoppingItemExport {
  return {
    id: item.id || undefined,
    amount: item.amount,
    unit: item.unit,
    name: item.name
  }
}

export default function ShoppingListPageClient() {
  const [showExport, setShowExport] = useState(false)
  const [showShare, setShowShare] = useState(false)
  const [ownerUserId, setOwnerUserId] = useState('')
  const [excludedGroups, setExcludedGroups] = useState<Set<string>>(new Set())

  const { data: accessibleData } = useAccessibleLists()
  const owners = accessibleData?.owners ?? []
  const selectedOwner = owners.find((o) =>
    ownerUserId === '' ? o.isSelf : o.userId === ownerUserId
  )
  const isSelf = !selectedOwner || selectedOwner.isSelf
  const canEdit = isSelf || (selectedOwner?.canEdit ?? false)

  const { data, isLoading, mutate } = useCustomList(ownerUserId)
  const { data: categoriesData, mutate: mutateCategories } = useCategories(ownerUserId)
  const categories = categoriesData?.categories ?? []

  const { data: sharesData, mutate: mutateShares } = useShoppingListShares()
  const shareList = useShareShoppingList()
  const unshareList = useUnshareShoppingList()

  const handleShare = async (contactUserId: string, edit: boolean) => {
    await shareList(contactUserId, edit)
    await mutateShares()
  }

  const handleUnshare = async (userId: string) => {
    await unshareList(userId)
    await mutateShares()
  }

  const items = (data?.items ?? []).map(toExportItem)

  const { data: groupsData } = useAllPlanIngredientGroups()
  const { data: mealExportData, isLoading: mealLoading } = useAllMealPlanExportItems(
    Array.from(excludedGroups)
  )

  // Map the aggregated meal-plan export items into the shared ShoppingItem shape
  // once, so both the read-only landing preview and the ExportModal work off a
  // single source of truth (and a single SWR fetch).
  const mealItems: ShoppingItemExport[] = useMemo(
    () =>
      (mealExportData?.items ?? []).map((item) => ({
        name: item.name,
        amount: item.amount,
        unit: item.unit,
        recipeName: item.recipeName,
        groupName: item.groupName || undefined
      })),
    [mealExportData]
  )

  const toggleGroup = (groupName: string) =>
    setExcludedGroups((prev) => {
      const next = new Set(prev)
      if (next.has(groupName)) next.delete(groupName)
      else next.add(groupName)
      return next
    })

  const handleDelete = async (itemId: string) => {
    const client = createServiceClient(ShoppingListService)
    await client.deleteShoppingItem({ itemId, ownerUserId })
    await mutate()
  }

  const handleEdit = async (
    itemId: string,
    values: { name: string; amount: string; unit: string }
  ) => {
    const client = createServiceClient(ShoppingListService)
    await client.updateShoppingItem({
      itemId,
      name: values.name,
      amount: values.amount || '0',
      unit: values.unit,
      ownerUserId
    })
    await mutate()
  }

  return (
    <PageContainer className="p-6">
      <div className="mb-6 flex items-center justify-between gap-2">
        <h1 className="text-3xl font-bold">Shopping List</h1>
        <div className="flex items-center gap-3">
          {owners.length > 1 && (
            <Select
              aria-label="Viewing list"
              value={ownerUserId}
              onChange={(e) => setOwnerUserId(e.target.value)}
              className="h-9 w-auto"
            >
              {owners.map((o) => (
                <option key={o.userId} value={o.isSelf ? '' : o.userId}>
                  {o.isSelf ? 'My list' : `${o.displayName}'s list`}
                </option>
              ))}
            </Select>
          )}
          {isSelf && (
            <Button variant="secondary" size="sm" onClick={() => setShowShare(true)}>
              Share
            </Button>
          )}
          <Link href="/shoppinglist/settings" className="text-sm text-accent hover:underline">
            Settings
          </Link>
        </div>
      </div>

      {canEdit && (
        <AddItemForm
          ownerUserId={ownerUserId}
          categories={categories}
          onAdded={mutate}
          onCategoriesChanged={mutateCategories}
        />
      )}

      {isLoading && <p className="text-muted">Loading…</p>}
      {!isLoading && (
        <ShoppingList
          items={items}
          onDelete={canEdit ? handleDelete : undefined}
          onEdit={canEdit ? handleEdit : undefined}
          onExport={() => setShowExport(true)}
        />
      )}

      <div className="mt-8 space-y-6">
        <MealPlanGroupFilter
          groups={groupsData?.groups ?? []}
          excludedGroups={excludedGroups}
          onToggle={toggleGroup}
        />
        <MealPlanItemsPreview mealItems={mealItems} isLoading={mealLoading} />
      </div>

      {showExport && (
        <ExportModal
          customItems={items}
          mealItems={mealItems}
          onClose={() => setShowExport(false)}
        />
      )}

      {showShare && (
        <ShareModal
          title="Share shopping list"
          shares={(sharesData?.shares ?? []).map((s) => ({
            userId: s.userId,
            displayName: s.displayName,
            canEdit: s.canEdit
          }))}
          onShare={handleShare}
          onUnshare={handleUnshare}
          onClose={() => setShowShare(false)}
        />
      )}
    </PageContainer>
  )
}
