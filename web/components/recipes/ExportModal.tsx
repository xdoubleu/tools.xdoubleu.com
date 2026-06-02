'use client'

import { useMemo, useState } from 'react'
import { useMealPlans } from '@/hooks/useMealPlans'
import {
  useMealPlanExportItems,
  useStores,
  useStoreCategories,
  useItemCategories
} from '@/hooks/useShoppingList'
import {
  formatForClipboard,
  formatForAppleNotes,
  formatAsTxt,
  groupByStore,
  formatGroupedForClipboard,
  formatGroupedForAppleNotes,
  formatGroupedAsTxt
} from '@/lib/recipes/shoppingExport'
import type { ShoppingItem, CategoryGroup } from '@/lib/recipes/shoppingExport'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogClose
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'

interface ExportModalProps {
  customItems: ShoppingItem[]
  onClose: () => void
}

export default function ExportModal({ customItems, onClose }: ExportModalProps) {
  const [selectedPlanId, setSelectedPlanId] = useState('')
  const [selectedStoreId, setSelectedStoreId] = useState('')
  const [copyFeedback, setCopyFeedback] = useState('')

  const { data: plansData, isLoading: plansLoading } = useMealPlans()
  const { data: exportData, isLoading: exportLoading } = useMealPlanExportItems(selectedPlanId)
  const { data: storesData } = useStores()
  const { data: storeCategoriesData } = useStoreCategories(selectedStoreId)
  const { data: itemCategoriesData } = useItemCategories()

  const mealItems: ShoppingItem[] | undefined =
    selectedPlanId && exportData
      ? exportData.items.map((item) => ({
          name: item.name,
          amount: item.amount,
          unit: item.unit
        }))
      : undefined

  const nameToCategoryId = useMemo(() => {
    const map: Record<string, string> = {}
    for (const entry of itemCategoriesData?.items ?? []) {
      map[entry.name] = entry.categoryId
    }
    return map
  }, [itemCategoriesData])

  const groups: CategoryGroup[] | undefined = useMemo(() => {
    if (!selectedStoreId || !storeCategoriesData) return undefined
    return groupByStore(customItems, mealItems, storeCategoriesData.categories, nameToCategoryId)
  }, [selectedStoreId, storeCategoriesData, customItems, mealItems, nameToCategoryId])

  const showFeedback = (msg: string) => {
    setCopyFeedback(msg)
    setTimeout(() => setCopyFeedback(''), 2000)
  }

  const clipboardText = () =>
    groups ? formatGroupedForClipboard(groups) : formatForClipboard(customItems, mealItems)

  const appleNotesText = () =>
    groups ? formatGroupedForAppleNotes(groups) : formatForAppleNotes(customItems, mealItems)

  const txtText = () => (groups ? formatGroupedAsTxt(groups) : formatAsTxt(customItems, mealItems))

  const handleExportClipboard = async () => {
    await navigator.clipboard.writeText(clipboardText())
    showFeedback('Copied!')
  }

  const handleExportAppleNotes = async () => {
    const text = appleNotesText()
    if (navigator.share) {
      await navigator.share({ text })
    } else {
      await navigator.clipboard.writeText(text)
      showFeedback('Copied (Apple Notes format)!')
    }
  }

  const handleExportTxt = () => {
    const text = txtText()
    const element = document.createElement('a')
    element.setAttribute('href', 'data:text/plain;charset=utf-8,' + encodeURIComponent(text))
    element.setAttribute('download', 'shopping-list.txt')
    element.style.display = 'none'
    document.body.appendChild(element)
    element.click()
    document.body.removeChild(element)
  }

  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Export Shopping List</DialogTitle>
          <DialogClose aria-label="Close">×</DialogClose>
        </DialogHeader>

        <div className="space-y-5">
          <div className="space-y-1.5">
            <Label htmlFor="export-plan-select">Add meal plan ingredients (optional)</Label>
            {plansLoading ? (
              <p className="text-sm text-muted">Loading plans...</p>
            ) : (
              <select
                id="export-plan-select"
                value={selectedPlanId}
                onChange={(e) => setSelectedPlanId(e.target.value)}
                className="flex h-11 w-full rounded-xl border border-input-border bg-input px-3 py-2 text-sm text-input-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
              >
                <option value="">-- None --</option>
                {(plansData?.plans ?? []).map((plan) => (
                  <option key={plan.id} value={plan.id}>
                    {plan.name}
                  </option>
                ))}
              </select>
            )}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="export-store-select">Order by store (optional)</Label>
            <select
              id="export-store-select"
              value={selectedStoreId}
              onChange={(e) => setSelectedStoreId(e.target.value)}
              className="flex h-11 w-full rounded-xl border border-input-border bg-input px-3 py-2 text-sm text-input-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
            >
              <option value="">-- No store (flat list) --</option>
              {(storesData?.stores ?? []).map((store) => (
                <option key={store.id} value={store.id}>
                  {store.name}
                </option>
              ))}
            </select>
          </div>

          {selectedStoreId && groups && (
            <div>
              <h3 className="mb-2 text-xs font-semibold uppercase tracking-widest text-muted">
                Grouped by store aisle
              </h3>
              {groups.length === 0 ? (
                <p className="text-sm text-muted">No items to export.</p>
              ) : (
                <div className="space-y-3">
                  {groups.map((group) => (
                    <div key={group.category}>
                      <p className="text-sm font-semibold text-fg">{group.category}</p>
                      <ul className="space-y-1">
                        {group.items.map((item, i) => (
                          <li key={i} className="text-sm text-subtle">
                            {item.amount} {item.unit} — {item.name}
                          </li>
                        ))}
                      </ul>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          {selectedPlanId && !selectedStoreId && (
            <div>
              <h3 className="mb-2 text-xs font-semibold uppercase tracking-widest text-muted">
                Meal plan — next 7 days
              </h3>
              {exportLoading && <p className="text-sm text-muted">Loading...</p>}
              {!exportLoading && mealItems && mealItems.length === 0 && (
                <p className="text-sm text-muted">No meals with recipes in the next 7 days.</p>
              )}
              {!exportLoading && mealItems && mealItems.length > 0 && (
                <ul className="space-y-1">
                  {mealItems.map((item, i) => (
                    <li key={i} className="text-sm text-subtle">
                      {item.amount} {item.unit} — {item.name}
                    </li>
                  ))}
                </ul>
              )}
            </div>
          )}

          <div className="flex flex-wrap items-center gap-2 border-t border-border pt-4">
            <Button size="sm" onClick={handleExportClipboard}>
              Copy to Clipboard
            </Button>
            <Button size="sm" onClick={handleExportAppleNotes}>
              Share to Apple Notes
            </Button>
            <Button size="sm" variant="secondary" onClick={handleExportTxt}>
              Download .txt
            </Button>
            {copyFeedback && <span className="text-sm text-success">{copyFeedback}</span>}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
