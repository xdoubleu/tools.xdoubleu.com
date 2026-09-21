'use client'

import { useState } from 'react'
import { useCategories } from '@/hooks/useShoppingList'
import { createServiceClient } from '@/lib/client'
import { ShoppingListService } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

export default function CategoryManager() {
  const { data, isLoading, mutate } = useCategories()
  const [newName, setNewName] = useState('')
  const [busy, setBusy] = useState(false)
  const [editingId, setEditingId] = useState('')
  const [editName, setEditName] = useState('')
  const [error, setError] = useState('')

  const client = createServiceClient(ShoppingListService)
  const categories = data?.categories ?? []

  const run = async (fn: () => Promise<unknown>) => {
    setBusy(true)
    setError('')
    try {
      await fn()
      await mutate()
    } catch {
      setError('Something went wrong. The name may already be in use.')
    } finally {
      setBusy(false)
    }
  }

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!newName.trim()) return
    await run(async () => {
      await client.createCategory({ name: newName.trim() })
      setNewName('')
    })
  }

  const handleRename = async (id: string) => {
    if (!editName.trim()) return
    await run(async () => {
      await client.renameCategory({ id, name: editName.trim() })
      setEditingId('')
      setEditName('')
    })
  }

  const handleDelete = async (id: string) => {
    await run(() => client.deleteCategory({ id }))
  }

  return (
    <div className="space-y-4">
      <form onSubmit={handleCreate} className="flex gap-2">
        <Input
          placeholder="New category (e.g. Produce)"
          value={newName}
          onChange={(e) => setNewName(e.target.value)}
        />
        <Button type="submit" disabled={busy || !newName.trim()}>
          Add
        </Button>
      </form>

      {error && <p className="text-sm text-danger">{error}</p>}
      {isLoading && <p className="text-sm text-muted">Loading…</p>}
      {!isLoading && categories.length === 0 && (
        <p className="text-sm text-muted">No categories yet.</p>
      )}

      <ul className="space-y-2">
        {categories.map((category) => (
          <li
            key={category.id}
            className="flex items-center gap-2 rounded-2xl border border-border bg-surface p-2"
          >
            {editingId === category.id ? (
              <>
                <Input
                  value={editName}
                  onChange={(e) => setEditName(e.target.value)}
                  className="h-9"
                />
                <Button size="sm" disabled={busy} onClick={() => handleRename(category.id)}>
                  Save
                </Button>
                <Button size="sm" variant="ghost" onClick={() => setEditingId('')}>
                  Cancel
                </Button>
              </>
            ) : (
              <>
                <span className="flex-1 text-sm text-fg">{category.name}</span>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => {
                    setEditingId(category.id)
                    setEditName(category.name)
                  }}
                >
                  Rename
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  disabled={busy}
                  onClick={() => handleDelete(category.id)}
                  aria-label={`Delete ${category.name}`}
                >
                  Delete
                </Button>
              </>
            )}
          </li>
        ))}
      </ul>
    </div>
  )
}
