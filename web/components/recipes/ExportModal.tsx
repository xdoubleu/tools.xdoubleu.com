'use client'

import { useState } from 'react'
import { useMealPlans } from '@/hooks/useMealPlans'
import { useMealPlanExportItems } from '@/hooks/useShoppingList'
import { formatForClipboard, formatForAppleNotes, formatAsTxt } from '@/lib/recipes/shoppingExport'
import type { ShoppingItem, DayItems } from '@/lib/recipes/shoppingExport'
import type { DayShoppingItems } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'
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

function toDayItems(raw: DayShoppingItems[]): DayItems[] {
  return raw.map((day) => ({
    date: day.date,
    items: day.items.map((item) => ({
      name: item.name,
      amount: item.amount,
      unit: item.unit
    }))
  }))
}

export default function ExportModal({ customItems, onClose }: ExportModalProps) {
  const [selectedPlanId, setSelectedPlanId] = useState('')
  const [copyFeedback, setCopyFeedback] = useState('')

  const { data: plansData, isLoading: plansLoading } = useMealPlans()
  const { data: exportData, isLoading: exportLoading } = useMealPlanExportItems(selectedPlanId)

  const dayItems = selectedPlanId && exportData ? toDayItems(exportData.dayItems) : undefined

  const showFeedback = (msg: string) => {
    setCopyFeedback(msg)
    setTimeout(() => setCopyFeedback(''), 2000)
  }

  const handleExportClipboard = async () => {
    const text = formatForClipboard(customItems, dayItems)
    await navigator.clipboard.writeText(text)
    showFeedback('Copied!')
  }

  const handleExportAppleNotes = async () => {
    const text = formatForAppleNotes(customItems, dayItems)
    if (navigator.share) {
      await navigator.share({ text })
    } else {
      await navigator.clipboard.writeText(text)
      showFeedback('Copied (Apple Notes format)!')
    }
  }

  const handleExportTxt = () => {
    const text = formatAsTxt(customItems, dayItems)
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

          {selectedPlanId && (
            <div>
              <h3 className="mb-2 text-xs font-semibold uppercase tracking-widest text-muted">
                Meal plan — next 7 days
              </h3>
              {exportLoading && <p className="text-sm text-muted">Loading...</p>}
              {!exportLoading && dayItems && dayItems.length === 0 && (
                <p className="text-sm text-muted">No meals with recipes in the next 7 days.</p>
              )}
              {!exportLoading && dayItems && dayItems.length > 0 && (
                <div className="space-y-3">
                  {dayItems.map((day) => (
                    <div key={day.date}>
                      <p className="text-sm font-medium text-fg">{day.date}</p>
                      <ul className="mt-1 space-y-1 pl-3">
                        {day.items.map((item, i) => (
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
