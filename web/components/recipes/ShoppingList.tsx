'use client'

import { useState } from 'react'
import { formatForClipboard, formatForAppleNotes, formatAsTxt } from '@/lib/recipes/shoppingExport'
import type { ShoppingItem } from '@/lib/recipes/shoppingExport'

interface ShoppingListProps {
  items: ShoppingItem[]
}

export default function ShoppingList({ items }: ShoppingListProps) {
  const [checkedItems, setCheckedItems] = useState<Set<string>>(new Set())

  const toggleItem = (index: number) => {
    const newChecked = new Set(checkedItems)
    const key = `${index}-${items[index].name}`
    if (newChecked.has(key)) {
      newChecked.delete(key)
    } else {
      newChecked.add(key)
    }
    setCheckedItems(newChecked)
  }

  const handleExportClipboard = () => {
    const text = formatForClipboard(items)
    navigator.clipboard.writeText(text)
    alert('Copied to clipboard!')
  }

  const handleExportAppleNotes = () => {
    const text = formatForAppleNotes(items)
    navigator.clipboard.writeText(text)
    alert('Copied to clipboard (Apple Notes format)!')
  }

  const handleExportTxt = () => {
    const text = formatAsTxt(items)
    const element = document.createElement('a')
    element.setAttribute('href', 'data:text/plain;charset=utf-8,' + encodeURIComponent(text))
    element.setAttribute('download', 'shopping-list.txt')
    element.style.display = 'none'
    document.body.appendChild(element)
    element.click()
    document.body.removeChild(element)
  }

  if (!items || items.length === 0) {
    return <p className="text-muted">No shopping items.</p>
  }

  return (
    <div className="space-y-4">
      <div className="flex gap-2">
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
          Copy (Apple Notes)
        </button>
        <button
          onClick={handleExportTxt}
          className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 text-sm"
        >
          Download .txt
        </button>
      </div>

      <div className="space-y-2">
        {items.map((item, idx) => {
          const key = `${idx}-${item.name}`
          const isChecked = checkedItems.has(key)
          return (
            <div key={key} className="flex items-center gap-3 p-3 border border-border rounded">
              <input
                type="checkbox"
                checked={isChecked}
                onChange={() => toggleItem(idx)}
                className="w-4 h-4 rounded"
              />
              <span className={isChecked ? 'line-through text-muted' : ''}>
                {item.amount} {item.unit} - {item.name}
              </span>
            </div>
          )
        })}
      </div>
    </div>
  )
}
