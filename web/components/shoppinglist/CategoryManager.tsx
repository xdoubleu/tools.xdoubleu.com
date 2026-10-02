'use client'

import { useState } from 'react'
import { useCategories } from '@/hooks/useShoppingList'
import { enqueueWrite } from '@/lib/offline/outbox'
import { hasName } from '@/lib/shoppinglist/names'
import {
  createCategoryWrite,
  deleteCategoryWrite,
  renameCategoryWrite
} from '@/lib/shoppinglist/offlineWrites'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { EmptyState, LoadingState } from '@/components/ui/states'

export default function CategoryManager() {
  const { data, isLoading } = useCategories()
  const [newName, setNewName] = useState('')
  const [busy, setBusy] = useState(false)
  const [editingId, setEditingId] = useState('')
  const [editName, setEditName] = useState('')
  const [error, setError] = useState('')

  const categories = data?.categories ?? []

  const run = async (fn: () => Promise<unknown>) => {
    setBusy(true)
    setError('')
    try {
      await fn()
    } finally {
      setBusy(false)
    }
  }

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault()
    const name = newName.trim()
    if (!name) return
    if (hasName(categories, name)) {
      setError('That name is already in use.')
      return
    }
    await run(async () => {
      await enqueueWrite(createCategoryWrite, { id: crypto.randomUUID(), name })
      setNewName('')
    })
  }

  const handleRename = async (id: string) => {
    const name = editName.trim()
    if (!name) return
    if (hasName(categories, name, id)) {
      setError('That name is already in use.')
      return
    }
    await run(async () => {
      await enqueueWrite(renameCategoryWrite, { id, name })
      setEditingId('')
      setEditName('')
    })
  }

  const handleDelete = async (id: string) => {
    await run(() => enqueueWrite(deleteCategoryWrite, { id }))
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
      {isLoading && <LoadingState className="text-sm" />}
      {!isLoading && categories.length === 0 && <EmptyState>No categories yet.</EmptyState>}

      <ul className="space-y-2">
        {categories.map((category) => (
          <li key={category.id}>
            <Card variant="inset" className="flex flex-wrap items-center gap-2 p-2">
              {editingId === category.id ? (
                <>
                  <Input
                    aria-label="Category name"
                    value={editName}
                    onChange={(e) => setEditName(e.target.value)}
                    className="min-w-0 flex-1 basis-40"
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
                  <span className="min-w-0 flex-1 break-words text-sm text-fg">
                    {category.name}
                  </span>
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
            </Card>
          </li>
        ))}
      </ul>
    </div>
  )
}
