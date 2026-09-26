'use client'

import { useMemo, useState } from 'react'
import Link from 'next/link'
import {
  useCustomList,
  useCategories,
  useAllMealPlanExportItems,
  useAllPlanIngredientGroups
} from '@/hooks/useShoppingList'
import ShoppingList from '@/components/shoppinglist/ShoppingList'
import ExportDialog from '@/components/shoppinglist/ExportDialog'
import AddItemForm from '@/components/shoppinglist/AddItemForm'
import MealPlanGroupFilter from '@/components/shoppinglist/MealPlanGroupFilter'
import MealPlanItemsPreview from '@/components/shoppinglist/MealPlanItemsPreview'
import { Button } from '@/components/ui/button'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { LoadingState } from '@/components/ui/states'
import { createServiceClient } from '@/lib/client'
import { ShoppingListService } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'
import type { ShoppingItem as ShoppingItemExport } from '@/lib/shoppinglist/shoppingExport'
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
  const [excludedGroups, setExcludedGroups] = useState<Set<string>>(new Set())

  const { data, isLoading, mutate } = useCustomList()
  const { data: categoriesData, mutate: mutateCategories } = useCategories()
  const categories = categoriesData?.categories ?? []

  const items = (data?.items ?? []).map(toExportItem)

  const { data: groupsData } = useAllPlanIngredientGroups()
  const { data: mealExportData, isLoading: mealLoading } = useAllMealPlanExportItems(
    Array.from(excludedGroups)
  )

  // Mapped once so the preview and ExportDialog share one source and fetch.
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
    await client.deleteShoppingItem({ itemId })
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
      unit: values.unit
    })
    await mutate()
  }

  return (
    <PageContainer>
      <PageHeader
        title="Shopping List"
        actions={
          <Button asChild variant="link" size="sm">
            <Link href="/shoppinglist/settings">Settings</Link>
          </Button>
        }
      />

      <AddItemForm
        categories={categories}
        onAdded={mutate}
        onCategoriesChanged={mutateCategories}
      />

      {isLoading && <LoadingState />}
      {!isLoading && (
        <ShoppingList
          items={items}
          onDelete={handleDelete}
          onEdit={handleEdit}
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
        <ExportDialog
          customItems={items}
          mealItems={mealItems}
          onClose={() => setShowExport(false)}
        />
      )}
    </PageContainer>
  )
}
