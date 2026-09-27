'use client'

import { useMemo, useState } from 'react'
import { useStores, useStoreCategories, useItemCategories } from '@/hooks/useShoppingList'
import {
  formatForClipboard,
  formatForAppleNotes,
  formatAsTxt,
  groupByStore,
  formatGroupedForClipboard,
  formatGroupedForAppleNotes,
  formatGroupedAsTxt,
  toExportGroups,
  prepareForExport,
  formatOrigins
} from '@/lib/shoppinglist/shoppingExport'
import type { ShoppingItem, StoreGrouping } from '@/lib/shoppinglist/shoppingExport'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogClose,
  DialogFooter
} from '@/components/ui/dialog'
import { Alert } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Select } from '@/components/ui/select'

interface ExportDialogProps {
  customItems: ShoppingItem[]
  // Fetched and group-filtered on the landing page.
  mealItems: ShoppingItem[]
  onClose: () => void
}

export default function ExportDialog({ customItems, mealItems, onClose }: ExportDialogProps) {
  const [selectedStoreId, setSelectedStoreId] = useState('')
  const [copyFeedback, setCopyFeedback] = useState('')

  const { data: storesData } = useStores()
  const { data: storeCategoriesData } = useStoreCategories(selectedStoreId)
  const { data: itemCategoriesData } = useItemCategories()

  const nameToCategoryId = useMemo(() => {
    const map: Record<string, string> = {}
    for (const entry of itemCategoriesData?.items ?? []) {
      map[entry.name] = entry.categoryId
    }
    return map
  }, [itemCategoriesData])

  const grouping: StoreGrouping | undefined = useMemo(() => {
    if (!selectedStoreId || !storeCategoriesData) return undefined
    return groupByStore(customItems, mealItems, storeCategoriesData.categories, nameToCategoryId)
  }, [selectedStoreId, storeCategoriesData, customItems, mealItems, nameToCategoryId])

  const exportGroups = useMemo(() => (grouping ? toExportGroups(grouping) : undefined), [grouping])

  const storeHasNoCategories =
    selectedStoreId && storeCategoriesData && storeCategoriesData.categories.length === 0

  const uncategorizedCount = grouping?.uncategorized.length ?? 0
  const unorderedCount = grouping?.unordered.length ?? 0

  const showFeedback = (msg: string) => {
    setCopyFeedback(msg)
    setTimeout(() => setCopyFeedback(''), 2000)
  }

  const clipboardText = () =>
    exportGroups
      ? formatGroupedForClipboard(exportGroups)
      : formatForClipboard(customItems, mealItems)

  const appleNotesText = () =>
    exportGroups
      ? formatGroupedForAppleNotes(exportGroups)
      : formatForAppleNotes(customItems, mealItems)

  const txtText = () =>
    exportGroups ? formatGroupedAsTxt(exportGroups) : formatAsTxt(customItems, mealItems)

  const handleExportClipboard = async () => {
    await navigator.clipboard.writeText(clipboardText())
    showFeedback('Copied!')
  }

  const handleExportAppleNotes = async () => {
    const text = appleNotesText()
    if (navigator.share) {
      try {
        await navigator.share({ text })
      } catch (e) {
        if (e instanceof DOMException && e.name === 'AbortError') return
        throw e
      }
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
            <Label htmlFor="export-store-select">Order by store (optional)</Label>
            <Select
              id="export-store-select"
              value={selectedStoreId}
              onChange={(e) => setSelectedStoreId(e.target.value)}
            >
              <option value="">-- No store (flat list) --</option>
              {(storesData?.stores ?? []).map((store) => (
                <option key={store.id} value={store.id}>
                  {store.name}
                </option>
              ))}
            </Select>
          </div>

          {storeHasNoCategories && (
            <Alert tone="warn">
              This store has no categories configured. Items will be exported as a flat list.
            </Alert>
          )}

          {!storeHasNoCategories && uncategorizedCount > 0 && (
            <Alert tone="warn">
              {uncategorizedCount === 1
                ? '1 item has no category assigned and will appear under "Other".'
                : `${uncategorizedCount} items have no category assigned and will appear under "Other".`}
            </Alert>
          )}

          {!storeHasNoCategories && unorderedCount > 0 && (
            <Alert tone="warn">
              {unorderedCount === 1
                ? '1 item has a category that this store doesn\'t order, so its place in the aisle order is unknown. It will appear under "Other".'
                : `${unorderedCount} items have a category that this store doesn't order, so their place in the aisle order is unknown. They will appear under "Other".`}
            </Alert>
          )}

          {selectedStoreId && exportGroups && (
            <div>
              <h3 className="mb-2 text-xs font-semibold uppercase tracking-widest text-muted">
                Grouped by store aisle
              </h3>
              {exportGroups.length === 0 ? (
                <p className="text-sm text-muted">No items to export.</p>
              ) : (
                <div className="space-y-3">
                  {exportGroups.map((group) => (
                    <div key={group.category}>
                      <p className="text-sm font-semibold text-fg">{group.category}</p>
                      <ul className="space-y-1">
                        {group.items.map((item, i) => (
                          <li key={i} className="text-sm text-subtle">
                            {item.amount} {item.unit} — {item.name}
                            {item.origins && item.origins.length > 0 && (
                              <span className="text-muted">{formatOrigins(item.origins)}</span>
                            )}
                          </li>
                        ))}
                      </ul>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          {!selectedStoreId && (
            <div>
              <h3 className="mb-2 text-xs font-semibold uppercase tracking-widest text-muted">
                Export preview
              </h3>
              <ul className="space-y-1">
                {prepareForExport(customItems, mealItems).map((item, i) => (
                  <li key={i} className="text-sm text-subtle">
                    {item.amount} {item.unit} — {item.name}
                    {item.origins && item.origins.length > 0 && (
                      <span className="text-muted">{formatOrigins(item.origins)}</span>
                    )}
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>

        <DialogFooter>
          {copyFeedback && (
            <span role="status" className="mr-auto text-sm text-success">
              {copyFeedback}
            </span>
          )}
          <Button size="sm" variant="secondary" onClick={handleExportTxt}>
            Download .txt
          </Button>
          <Button size="sm" onClick={handleExportAppleNotes}>
            Share to Apple Notes
          </Button>
          <Button size="sm" onClick={handleExportClipboard}>
            Copy to Clipboard
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
