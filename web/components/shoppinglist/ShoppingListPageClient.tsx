'use client'

import { useMemo, useState } from 'react'
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
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader, PageHeaderSettingsLink } from '@/components/ui/page-header'
import { LoadingState } from '@/components/ui/states'
import { enqueueWrite } from '@/lib/offline/outbox'
import { deleteShoppingItemWrite, updateShoppingItemWrite } from '@/lib/shoppinglist/offlineWrites'
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

  const { data, isLoading } = useCustomList()
  const { data: categoriesData } = useCategories()
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

  const handleDelete = (itemId: string) => enqueueWrite(deleteShoppingItemWrite, { itemId })

  const handleEdit = (itemId: string, values: { name: string; amount: string; unit: string }) =>
    enqueueWrite(updateShoppingItemWrite, {
      itemId,
      name: values.name,
      amount: values.amount || '0',
      unit: values.unit
    })

  return (
    <PageContainer>
      <PageHeader
        title="Shopping List"
        actions={<PageHeaderSettingsLink href="/shoppinglist/settings" />}
      />

      <AddItemForm categories={categories} />

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
