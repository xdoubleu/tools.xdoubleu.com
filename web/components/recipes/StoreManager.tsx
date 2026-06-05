'use client'

import { useEffect, useRef, useState } from 'react'
import { useStores, useStoreCategories, useCategories } from '@/hooks/useShoppingList'
import { createServiceClient } from '@/lib/client'
import { ShoppingListService } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'

interface OrderedCategory {
  id: string
  name: string
}

export default function StoreManager() {
  const { data: storesData, isLoading, mutate: mutateStores } = useStores()
  const [selectedStoreId, setSelectedStoreId] = useState('')
  const [newName, setNewName] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const client = createServiceClient(ShoppingListService)
  const stores = storesData?.stores ?? []

  const run = async (fn: () => Promise<unknown>) => {
    setBusy(true)
    setError('')
    try {
      await fn()
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
      await client.createStore({ name: newName.trim() })
      setNewName('')
      await mutateStores()
    })
  }

  const handleDelete = async (id: string) => {
    await run(async () => {
      await client.deleteStore({ id })
      if (selectedStoreId === id) setSelectedStoreId('')
      await mutateStores()
    })
  }

  return (
    <div className="space-y-4">
      <form onSubmit={handleCreate} className="flex gap-2">
        <Input
          placeholder="New store (e.g. Colruyt)"
          value={newName}
          onChange={(e) => setNewName(e.target.value)}
        />
        <Button type="submit" disabled={busy || !newName.trim()}>
          Add
        </Button>
      </form>

      {error && <p className="text-sm text-danger">{error}</p>}
      {isLoading && <p className="text-sm text-muted">Loading…</p>}
      {!isLoading && stores.length === 0 && <p className="text-sm text-muted">No stores yet.</p>}

      <ul className="space-y-2">
        {stores.map((store) => (
          <li
            key={store.id}
            className="flex items-center gap-2 rounded-2xl border border-border bg-surface p-2"
          >
            <span className="flex-1 text-sm text-fg">{store.name}</span>
            <Button
              size="sm"
              variant={selectedStoreId === store.id ? 'default' : 'ghost'}
              onClick={() => setSelectedStoreId(selectedStoreId === store.id ? '' : store.id)}
            >
              {selectedStoreId === store.id ? 'Editing' : 'Edit order'}
            </Button>
            <Button
              size="sm"
              variant="ghost"
              disabled={busy}
              onClick={() => handleDelete(store.id)}
              aria-label={`Delete ${store.name}`}
            >
              Delete
            </Button>
          </li>
        ))}
      </ul>

      {selectedStoreId && <StoreCategoryOrder storeId={selectedStoreId} />}
    </div>
  )
}

function StoreCategoryOrder({ storeId }: { storeId: string }) {
  const { data: storeCategoriesData, mutate } = useStoreCategories(storeId)
  const { data: categoriesData } = useCategories()
  const [order, setOrder] = useState<OrderedCategory[]>([])
  const [saved, setSaved] = useState(false)
  const [busy, setBusy] = useState(false)

  const client = createServiceClient(ShoppingListService)

  // Initialize order from server data, but only once per storeId so that local
  // reordering state isn't wiped on every SWR re-fetch (which may return a new
  // object reference each time).
  const lastInitializedStoreId = useRef<string>('')
  useEffect(() => {
    if (storeCategoriesData && storeId !== lastInitializedStoreId.current) {
      setOrder(storeCategoriesData.categories.map((c) => ({ id: c.id, name: c.name })))
      lastInitializedStoreId.current = storeId
    }
  }, [storeCategoriesData, storeId])

  const allCategories = categoriesData?.categories ?? []
  const available = allCategories.filter((c) => !order.some((o) => o.id === c.id))

  const move = (index: number, delta: number) => {
    const target = index + delta
    if (target < 0 || target >= order.length) return
    const next = [...order]
    ;[next[index], next[target]] = [next[target], next[index]]
    setOrder(next)
    setSaved(false)
  }

  const remove = (id: string) => {
    setOrder(order.filter((o) => o.id !== id))
    setSaved(false)
  }

  const add = (id: string) => {
    const category = allCategories.find((c) => c.id === id)
    if (!category) return
    setOrder([...order, { id: category.id, name: category.name }])
    setSaved(false)
  }

  const save = async () => {
    setBusy(true)
    setSaved(false)
    try {
      await client.setStoreCategories({ storeId, categoryIds: order.map((o) => o.id) })
      await mutate()
      setSaved(true)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-3 rounded-2xl border border-border bg-card p-3">
      <h3 className="text-sm font-semibold text-fg">Aisle order</h3>
      <p className="text-xs text-muted">
        Arrange categories in the order you walk this store. Items export in this order.
      </p>

      {order.length === 0 ? (
        <p className="text-sm text-muted">No categories added to this store yet.</p>
      ) : (
        <ul className="space-y-2">
          {order.map((category, index) => (
            <li
              key={category.id}
              className="flex items-center gap-2 rounded-xl border border-border bg-surface p-2"
            >
              <span className="w-6 text-xs text-muted">{index + 1}.</span>
              <span className="flex-1 text-sm text-fg">{category.name}</span>
              <Button
                size="sm"
                variant="ghost"
                disabled={index === 0}
                onClick={() => move(index, -1)}
                aria-label={`Move ${category.name} up`}
              >
                ↑
              </Button>
              <Button
                size="sm"
                variant="ghost"
                disabled={index === order.length - 1}
                onClick={() => move(index, 1)}
                aria-label={`Move ${category.name} down`}
              >
                ↓
              </Button>
              <Button
                size="sm"
                variant="ghost"
                onClick={() => remove(category.id)}
                aria-label={`Remove ${category.name}`}
              >
                ×
              </Button>
            </li>
          ))}
        </ul>
      )}

      {available.length > 0 && (
        <Select
          aria-label="Add category to store"
          value=""
          onChange={(e) => e.target.value && add(e.target.value)}
        >
          <option value="">+ Add category…</option>
          {available.map((category) => (
            <option key={category.id} value={category.id}>
              {category.name}
            </option>
          ))}
        </Select>
      )}

      <div className="flex items-center gap-2">
        <Button size="sm" disabled={busy} onClick={save}>
          Save order
        </Button>
        {saved && <span className="text-sm text-success">Saved!</span>}
      </div>
    </div>
  )
}
