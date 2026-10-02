'use client'

import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { enqueueWrite } from '@/lib/offline/outbox'
import { hasName } from '@/lib/shoppinglist/names'
import {
  createCategoryWrite,
  createShoppingItemWrite,
  setItemCategoryWrite
} from '@/lib/shoppinglist/offlineWrites'
import type { Category } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'

// Sentinel select value that reveals the new-category name input.
const NEW_CATEGORY = '__new__'

interface AddItemFormProps {
  categories: Category[]
}

export default function AddItemForm({ categories }: AddItemFormProps) {
  const [newName, setNewName] = useState('')
  const [newAmount, setNewAmount] = useState('')
  const [newUnit, setNewUnit] = useState('')
  const [newCategoryId, setNewCategoryId] = useState('')
  const [newCategoryName, setNewCategoryName] = useState('')
  const [adding, setAdding] = useState(false)

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault()
    const name = newName.trim()
    if (!name) return
    setAdding(true)
    try {
      await enqueueWrite(createShoppingItemWrite, {
        id: crypto.randomUUID(),
        amount: newAmount || '0',
        unit: newUnit.trim(),
        name
      })
      let categoryId = newCategoryId
      if (newCategoryId === NEW_CATEGORY) {
        const trimmed = newCategoryName.trim()
        // Reuse a same-named category rather than queue a create the API rejects.
        const existing = categories.find((c) => hasName([c], trimmed))
        categoryId = existing?.id ?? ''
        if (trimmed && !existing) {
          categoryId = crypto.randomUUID()
          await enqueueWrite(createCategoryWrite, { id: categoryId, name: trimmed })
        }
      }
      // Categories live in the name->category catalog, so this persists by name.
      if (categoryId) {
        await enqueueWrite(setItemCategoryWrite, { name, categoryId })
      }
      setNewName('')
      setNewAmount('')
      setNewUnit('')
      setNewCategoryId('')
      setNewCategoryName('')
    } finally {
      setAdding(false)
    }
  }

  return (
    <form onSubmit={handleAdd} className="flex flex-wrap gap-2 mb-6">
      <Input
        type="number"
        inputMode="decimal"
        placeholder="Amount"
        value={newAmount}
        onChange={(e) => setNewAmount(e.target.value)}
        min="0"
        step="any"
        className="min-w-0 flex-1 sm:w-24 sm:flex-none"
      />
      <Input
        type="text"
        placeholder="Unit"
        value={newUnit}
        onChange={(e) => setNewUnit(e.target.value)}
        className="min-w-0 flex-1 sm:w-24 sm:flex-none"
      />
      <Input
        type="text"
        placeholder="Item name"
        value={newName}
        onChange={(e) => setNewName(e.target.value)}
        required
        className="min-w-32 flex-1"
      />
      <Select
        aria-label="Category"
        value={newCategoryId}
        onChange={(e) => setNewCategoryId(e.target.value)}
        className="w-auto"
      >
        <option value="">-- Category --</option>
        {categories.map((category) => (
          <option key={category.id} value={category.id}>
            {category.name}
          </option>
        ))}
        <option value={NEW_CATEGORY}>+ New category</option>
      </Select>
      {newCategoryId === NEW_CATEGORY && (
        <Input
          type="text"
          placeholder="New category"
          aria-label="New category name"
          value={newCategoryName}
          onChange={(e) => setNewCategoryName(e.target.value)}
          className="w-full sm:w-32"
        />
      )}
      <Button type="submit" disabled={adding || !newName.trim()}>
        Add
      </Button>
    </form>
  )
}
