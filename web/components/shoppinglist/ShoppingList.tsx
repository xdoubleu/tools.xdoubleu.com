'use client'

import { useState } from 'react'
import type { ShoppingItem } from '@/lib/shoppinglist/shoppingExport'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import { cn } from '@/lib/cn'

export interface ShoppingItemEdit {
  name: string
  amount: string
  unit: string
}

interface ShoppingListProps {
  items: ShoppingItem[]
  onDelete?: (itemId: string) => Promise<void>
  onEdit?: (itemId: string, values: ShoppingItemEdit) => Promise<void>
  onExport?: () => void
}

export default function ShoppingList({ items, onDelete, onEdit, onExport }: ShoppingListProps) {
  const [checkedItems, setCheckedItems] = useState<Set<string>>(new Set())
  const [editingId, setEditingId] = useState<string | null>(null)
  const [draft, setDraft] = useState<ShoppingItemEdit>({ name: '', amount: '', unit: '' })
  const [saving, setSaving] = useState(false)

  const toggleItem = (key: string) => {
    const newChecked = new Set(checkedItems)
    if (newChecked.has(key)) newChecked.delete(key)
    else newChecked.add(key)
    setCheckedItems(newChecked)
  }

  const startEdit = (item: ShoppingItem) => {
    setEditingId(item.id ?? null)
    setDraft({ name: item.name, amount: item.amount, unit: item.unit })
  }

  const cancelEdit = () => {
    setEditingId(null)
    setDraft({ name: '', amount: '', unit: '' })
  }

  const saveEdit = async (itemId: string) => {
    if (!onEdit || !draft.name.trim()) return
    setSaving(true)
    try {
      await onEdit(itemId, {
        name: draft.name.trim(),
        amount: draft.amount,
        unit: draft.unit.trim()
      })
      cancelEdit()
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="space-y-4">
      {onExport && (
        <Button variant="secondary" size="sm" onClick={onExport}>
          Export
        </Button>
      )}

      {items.length === 0 ? (
        <p className="text-muted">No items yet. Add something above.</p>
      ) : (
        <div className="space-y-2">
          {items.map((item, idx) => {
            const key = `item-${idx}-${item.name}`
            const isChecked = checkedItems.has(key)
            const isEditing = editingId != null && item.id === editingId

            if (isEditing && item.id) {
              return (
                <Card key={key} variant="inset" className="flex flex-wrap items-center gap-2">
                  <Input
                    type="number"
                    inputMode="decimal"
                    aria-label="Amount"
                    placeholder="Amount"
                    value={draft.amount}
                    onChange={(e) => setDraft({ ...draft, amount: e.target.value })}
                    min="0"
                    step="any"
                    className="min-w-0 flex-1 sm:w-24 sm:flex-none"
                  />
                  <Input
                    type="text"
                    aria-label="Unit"
                    placeholder="Unit"
                    value={draft.unit}
                    onChange={(e) => setDraft({ ...draft, unit: e.target.value })}
                    className="min-w-0 flex-1 sm:w-24 sm:flex-none"
                  />
                  <Input
                    type="text"
                    aria-label="Item name"
                    placeholder="Item name"
                    value={draft.name}
                    onChange={(e) => setDraft({ ...draft, name: e.target.value })}
                    className="min-w-0 flex-1 basis-full sm:basis-32"
                  />
                  <Button
                    size="sm"
                    onClick={() => saveEdit(item.id!)}
                    disabled={saving || !draft.name.trim()}
                  >
                    Save
                  </Button>
                  <Button variant="ghost" size="sm" onClick={cancelEdit} disabled={saving}>
                    Cancel
                  </Button>
                </Card>
              )
            }

            return (
              <Card key={key} variant="inset" className="flex flex-wrap items-center gap-x-3 py-0">
                <Checkbox
                  checked={isChecked}
                  onChange={() => toggleItem(key)}
                  labelClassName="min-w-0 flex-1"
                  label={
                    <span
                      className={cn(
                        'min-w-0 break-words text-sm',
                        isChecked ? 'text-muted line-through' : 'text-fg'
                      )}
                    >
                      {item.amount} {item.unit} - {item.name}
                    </span>
                  }
                />
                {item.id && onEdit && (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => startEdit(item)}
                    aria-label={`Edit ${item.name}`}
                    className="text-muted hover:text-accent"
                  >
                    Edit
                  </Button>
                )}
                {item.id && onDelete && (
                  <Button
                    variant="ghost"
                    size="iconSm"
                    onClick={() => onDelete(item.id!)}
                    aria-label={`Remove ${item.name}`}
                    className="rounded-full text-muted hover:bg-transparent hover:text-danger focus-visible:ring-danger/50"
                  >
                    ×
                  </Button>
                )}
              </Card>
            )
          })}
        </div>
      )}
    </div>
  )
}
