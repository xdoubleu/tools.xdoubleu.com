'use client'

import { useState } from 'react'
import { useMealPlans } from '@/hooks/useMealPlans'
import { useMealPlanExportItems } from '@/hooks/useShoppingList'
import { formatForClipboard, formatForAppleNotes, formatAsTxt } from '@/lib/recipes/shoppingExport'
import type { ShoppingItem, DayItems } from '@/lib/recipes/shoppingExport'
import type { DayShoppingItems } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'

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
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose()
      }}
    >
      <div className="bg-surface w-full max-w-lg mx-4 rounded-lg shadow-xl p-6 space-y-6 max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between">
          <h2 className="text-lg font-semibold">Export Shopping List</h2>
          <button
            onClick={onClose}
            aria-label="Close"
            className="text-muted hover:text-foreground text-xl leading-none"
          >
            ×
          </button>
        </div>

        <div>
          <label
            htmlFor="export-plan-select"
            className="block text-sm font-medium text-subtle mb-1"
          >
            Add meal plan ingredients (optional)
          </label>
          {plansLoading ? (
            <p className="text-sm text-muted">Loading plans...</p>
          ) : (
            <select
              id="export-plan-select"
              value={selectedPlanId}
              onChange={(e) => setSelectedPlanId(e.target.value)}
              className="w-full px-3 py-2 border border-input-border bg-input text-input-text rounded text-sm"
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
            <h3 className="text-sm font-semibold text-subtle uppercase tracking-wide mb-2">
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
                    <p className="text-sm font-medium">{day.date}</p>
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

        <div className="flex flex-wrap gap-2 items-center pt-2 border-t border-border">
          <button
            onClick={handleExportClipboard}
            className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 text-sm"
          >
            Copy to Clipboard
          </button>
          <button
            onClick={handleExportAppleNotes}
            className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 text-sm"
          >
            Share to Apple Notes
          </button>
          <button
            onClick={handleExportTxt}
            className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 text-sm"
          >
            Download .txt
          </button>
          {copyFeedback && <span className="text-sm text-green-600">{copyFeedback}</span>}
        </div>
      </div>
    </div>
  )
}
