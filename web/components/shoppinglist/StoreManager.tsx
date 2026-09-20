'use client'

import { useEffect, useRef, useState } from 'react'
import {
  DndContext,
  KeyboardSensor,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent
} from '@dnd-kit/core'
import {
  SortableContext,
  arrayMove,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
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

  const sensors = useSensors(
    // A small activation distance lets taps still reach the × button without
    // accidentally starting a drag.
    useSensor(PointerSensor, { activationConstraint: { distance: 4 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates })
  )

  const handleDragEnd = (event: DragEndEvent) => {
    const { active, over } = event
    if (!over || active.id === over.id) return
    const oldIndex = order.findIndex((o) => o.id === active.id)
    const newIndex = order.findIndex((o) => o.id === over.id)
    if (oldIndex === -1 || newIndex === -1) return
    setOrder(arrayMove(order, oldIndex, newIndex))
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
        <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
          <SortableContext items={order.map((o) => o.id)} strategy={verticalListSortingStrategy}>
            <ul className="space-y-2">
              {order.map((category, index) => (
                <SortableCategoryRow
                  key={category.id}
                  category={category}
                  index={index}
                  onRemove={remove}
                />
              ))}
            </ul>
          </SortableContext>
        </DndContext>
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

function SortableCategoryRow({
  category,
  index,
  onRemove
}: {
  category: OrderedCategory
  index: number
  onRemove: (id: string) => void
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: category.id
  })

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : undefined
  }

  return (
    <li
      ref={setNodeRef}
      style={style}
      className="flex items-center gap-2 rounded-xl border border-border bg-surface p-2"
    >
      <Button
        size="iconSm"
        variant="ghost"
        className="cursor-grab touch-none active:cursor-grabbing"
        aria-label={`Reorder ${category.name}`}
        {...attributes}
        {...listeners}
      >
        ⠿
      </Button>
      <span className="w-6 text-xs text-muted">{index + 1}.</span>
      <span className="flex-1 text-sm text-fg">{category.name}</span>
      <Button
        size="sm"
        variant="ghost"
        onClick={() => onRemove(category.id)}
        aria-label={`Remove ${category.name}`}
      >
        ×
      </Button>
    </li>
  )
}
