'use client'

import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { createServiceClient } from '@/lib/client'
import { ShoppingListService } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'
import type { Category } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'

// Sentinel category id that switches the category select into "create a new
// category" mode, revealing the name input below it.
const NEW_CATEGORY = '__new__'

interface AddItemFormProps {
  ownerUserId: string
  categories: Category[]
  onAdded: () => Promise<unknown>
  onCategoriesChanged: () => Promise<unknown>
}

export default function AddItemForm({
  ownerUserId,
  categories,
  onAdded,
  onCategoriesChanged
}: AddItemFormProps) {
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
      const client = createServiceClient(ShoppingListService)
      await client.createShoppingItem({
        amount: newAmount || '0',
        unit: newUnit.trim(),
        name,
        ownerUserId
      })
      // Resolve the effective category id: either an existing selection or a
      // brand-new category created inline from the add form.
      let categoryId = newCategoryId
      if (newCategoryId === NEW_CATEGORY) {
        const trimmed = newCategoryName.trim()
        categoryId = ''
        if (trimmed) {
          const resp = await client.createCategory({ name: trimmed, ownerUserId })
          categoryId = resp.category?.id ?? ''
          await onCategoriesChanged()
        }
      }
      // The category lives in the name->category catalog, not on the item, so
      // assigning it here makes it persist by name across every list and export.
      if (categoryId) {
        await client.setItemCategory({ name, categoryId, ownerUserId })
      }
      setNewName('')
      setNewAmount('')
      setNewUnit('')
      setNewCategoryId('')
      setNewCategoryName('')
      await onAdded()
    } finally {
      setAdding(false)
    }
  }

  return (
    <form onSubmit={handleAdd} className="flex flex-wrap gap-2 mb-6">
      <Input
        type="number"
        placeholder="Amount"
        value={newAmount}
        onChange={(e) => setNewAmount(e.target.value)}
        min="0"
        step="any"
        className="w-24"
      />
      <Input
        type="text"
        placeholder="Unit"
        value={newUnit}
        onChange={(e) => setNewUnit(e.target.value)}
        className="w-24"
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
          className="w-32"
        />
      )}
      <Button type="submit" disabled={adding || !newName.trim()}>
        Add
      </Button>
    </form>
  )
}
