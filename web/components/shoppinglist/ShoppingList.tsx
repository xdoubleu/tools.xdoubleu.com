import { useState } from 'react'
import { useLocalStorage } from '@/hooks/useLocalStorage'
import type { ShoppingItem } from '@/lib/shoppinglist/shoppingExport'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Checkbox } from '@/components/ui/checkbox'
import { ConfirmDialog } from '@/components/ui/dialog'
import { RemoveButton } from '@/components/ui/remove-button'
import { cn } from '@/lib/cn'
import { focusById } from '@/lib/focusById'
import { EmptyState } from '@/components/ui/states'
import { formatQuantity } from '@/lib/format'

export interface ShoppingItemEdit {
  name: string
  amount: string
  unit: string
}

interface ShoppingListProps {
  items: ShoppingItem[]
  onDelete?: (itemId: string) => Promise<void>
  onEdit?: (itemId: string, values: ShoppingItemEdit) => Promise<void>
}

// One list per family, so one key.
const TICKS_KEY = 'shoppinglist:ticks'

const itemKey = (item: ShoppingItem, idx: number) => item.id ?? `item-${idx}-${item.name}`

export default function ShoppingList({ items, onDelete, onEdit }: ShoppingListProps) {
  const [storedTicks, setStoredTicks] = useLocalStorage<string[]>(TICKS_KEY, [])
  const [editingId, setEditingId] = useState<string | null>(null)
  const [draft, setDraft] = useState<ShoppingItemEdit>({ name: '', amount: '', unit: '' })
  const [saving, setSaving] = useState(false)
  // Kept after the confirm closes so its title doesn't change mid-animation.
  const [removing, setRemoving] = useState<ShoppingItem | null>(null)
  const [confirmingRemove, setConfirmingRemove] = useState(false)

  // Ticks of items no longer on the list are ignored, and dropped on the next write.
  const keys = new Set(items.map(itemKey))
  const checkedItems = new Set(
    (Array.isArray(storedTicks) ? storedTicks : []).filter((key) => keys.has(key))
  )

  const toggleItem = (key: string) => {
    const newChecked = new Set(checkedItems)
    if (newChecked.has(key)) newChecked.delete(key)
    else newChecked.add(key)
    setStoredTicks([...newChecked])
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
      {items.length === 0 ? (
        <EmptyState
          placement="inline"
          action={
            <Button onClick={() => focusById('shopping-item-name')}>Add your first item</Button>
          }
        >
          No items yet.
        </EmptyState>
      ) : (
        <div className="space-y-2">
          {items.map((item, idx) => {
            const key = itemKey(item, idx)
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
                    onClick={() => saveEdit(item.id!)}
                    disabled={saving || !draft.name.trim()}
                  >
                    Save
                  </Button>
                  <Button variant="ghost" onClick={cancelEdit} disabled={saving}>
                    Cancel
                  </Button>
                </Card>
              )
            }

            return (
              <Card key={key} variant="inset" padding="none">
                <div className="flex flex-wrap items-center gap-x-3 pl-3 pr-1">
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
                        {formatQuantity(item.amount, item.unit, item.name)}
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
                    <RemoveButton
                      onClick={() => {
                        setRemoving(item)
                        setConfirmingRemove(true)
                      }}
                      aria-label={`Remove ${item.name}`}
                    />
                  )}
                </div>
              </Card>
            )
          })}
        </div>
      )}

      <ConfirmDialog
        open={confirmingRemove}
        onOpenChange={setConfirmingRemove}
        title={`Remove ${removing?.name ?? 'item'}?`}
        description="It's taken off the shopping list."
        confirmLabel="Remove"
        destructive
        onConfirm={() => {
          setConfirmingRemove(false)
          if (removing?.id && onDelete) void onDelete(removing.id)
        }}
      />
    </div>
  )
}
