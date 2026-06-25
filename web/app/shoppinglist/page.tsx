'use client'

import { useState } from 'react'
import Link from 'next/link'
import {
  useCustomList,
  useCategories,
  useAccessibleLists,
  useShoppingListShares,
  useShareShoppingList,
  useUnshareShoppingList
} from '@/hooks/useShoppingList'
import ShoppingList from '@/components/recipes/ShoppingList'
import ExportModal from '@/components/recipes/ExportModal'
import ShareModal from '@/components/recipes/ShareModal'
import { Button } from '@/components/ui/button'
import { PageContainer } from '@/components/ui/page-container'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { createServiceClient } from '@/lib/client'
import { ShoppingListService } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'
import type { ShoppingItem as ShoppingItemExport } from '@/lib/recipes/shoppingExport'
import type { ShoppingItem } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'

function toExportItem(item: ShoppingItem): ShoppingItemExport {
  return {
    id: item.id || undefined,
    amount: item.amount,
    unit: item.unit,
    name: item.name
  }
}

// Sentinel category id that switches the category select into "create a new
// category" mode, revealing the name input below it.
const NEW_CATEGORY = '__new__'

export default function ShoppingPage() {
  const [newName, setNewName] = useState('')
  const [newAmount, setNewAmount] = useState('')
  const [newUnit, setNewUnit] = useState('')
  const [newCategoryId, setNewCategoryId] = useState('')
  const [newCategoryName, setNewCategoryName] = useState('')
  const [adding, setAdding] = useState(false)
  const [showExport, setShowExport] = useState(false)
  const [showShare, setShowShare] = useState(false)
  const [ownerUserId, setOwnerUserId] = useState('')

  const { data: accessibleData } = useAccessibleLists()
  const owners = accessibleData?.owners ?? []
  const selectedOwner = owners.find((o) =>
    ownerUserId === '' ? o.isSelf : o.userId === ownerUserId
  )
  const isSelf = !selectedOwner || selectedOwner.isSelf
  const canEdit = isSelf || (selectedOwner?.canEdit ?? false)

  const { data, isLoading, mutate } = useCustomList(ownerUserId)
  const { data: categoriesData, mutate: mutateCategories } = useCategories(ownerUserId)
  const categories = categoriesData?.categories ?? []

  const { data: sharesData, mutate: mutateShares } = useShoppingListShares()
  const shareList = useShareShoppingList()
  const unshareList = useUnshareShoppingList()

  const handleShare = async (contactUserId: string, edit: boolean) => {
    await shareList(contactUserId, edit)
    await mutateShares()
  }

  const handleUnshare = async (userId: string) => {
    await unshareList(userId)
    await mutateShares()
  }

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault()
    const name = newName.trim()
    if (!name) return
    setAdding(true)
    try {
      const client = createServiceClient(ShoppingListService)
      await client.addShoppingItem({
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
          await mutateCategories()
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
      await mutate()
    } finally {
      setAdding(false)
    }
  }

  const items = (data?.items ?? []).map(toExportItem)

  const handleDelete = async (itemId: string) => {
    const client = createServiceClient(ShoppingListService)
    await client.deleteShoppingItem({ itemId, ownerUserId })
    await mutate()
  }

  const handleEdit = async (
    itemId: string,
    values: { name: string; amount: string; unit: string }
  ) => {
    const client = createServiceClient(ShoppingListService)
    await client.updateShoppingItem({
      itemId,
      name: values.name,
      amount: values.amount || '0',
      unit: values.unit,
      ownerUserId
    })
    await mutate()
  }

  return (
    <PageContainer className="p-6">
      <div className="mb-6 flex items-center justify-between gap-2">
        <h1 className="text-3xl font-bold">Shopping List</h1>
        <div className="flex items-center gap-3">
          {owners.length > 1 && (
            <Select
              aria-label="Viewing list"
              value={ownerUserId}
              onChange={(e) => setOwnerUserId(e.target.value)}
              className="h-9 w-auto"
            >
              {owners.map((o) => (
                <option key={o.userId} value={o.isSelf ? '' : o.userId}>
                  {o.isSelf ? 'My list' : `${o.displayName}'s list`}
                </option>
              ))}
            </Select>
          )}
          {isSelf && (
            <Button variant="secondary" size="sm" onClick={() => setShowShare(true)}>
              Share
            </Button>
          )}
          <Link href="/shoppinglist/settings" className="text-sm text-accent hover:underline">
            Settings
          </Link>
        </div>
      </div>

      {canEdit && (
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
      )}

      {isLoading && <p>Loading...</p>}
      {!isLoading && (
        <ShoppingList
          items={items}
          onDelete={canEdit ? handleDelete : undefined}
          onEdit={canEdit ? handleEdit : undefined}
          onExport={() => setShowExport(true)}
        />
      )}

      {showExport && <ExportModal customItems={items} onClose={() => setShowExport(false)} />}

      {showShare && (
        <ShareModal
          title="Share shopping list"
          shares={(sharesData?.shares ?? []).map((s) => ({
            userId: s.userId,
            displayName: s.displayName,
            canEdit: s.canEdit
          }))}
          onShare={handleShare}
          onUnshare={handleUnshare}
          onClose={() => setShowShare(false)}
        />
      )}
    </PageContainer>
  )
}
