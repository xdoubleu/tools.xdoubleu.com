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
import { enqueueWrite } from '@/lib/offline/outbox'
import { hasName } from '@/lib/shoppinglist/names'
import {
  createStoreWrite,
  deleteStoreWrite,
  setStoreCategoriesWrite
} from '@/lib/shoppinglist/offlineWrites'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Select } from '@/components/ui/select'
import { EmptyState, LoadingState } from '@/components/ui/states'

interface OrderedCategory {
  id: string
  name: string
}

export default function StoreManager() {
  const { data: storesData, isLoading } = useStores()
  const [selectedStoreId, setSelectedStoreId] = useState('')
  const [newName, setNewName] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const stores = storesData?.stores ?? []

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
    if (hasName(stores, name)) {
      setError('That name is already in use.')
      return
    }
    await run(async () => {
      await enqueueWrite(createStoreWrite, { id: crypto.randomUUID(), name })
      setNewName('')
    })
  }

  const handleDelete = async (id: string) => {
    await run(async () => {
      await enqueueWrite(deleteStoreWrite, { id })
      if (selectedStoreId === id) setSelectedStoreId('')
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
      {isLoading && <LoadingState className="text-sm" />}
      {!isLoading && stores.length === 0 && <EmptyState>No stores yet.</EmptyState>}

      <ul className="space-y-2">
        {stores.map((store) => (
          <li key={store.id}>
            <Card variant="inset" className="flex flex-wrap items-center gap-2 p-2">
              <span className="min-w-0 flex-1 break-words text-sm text-fg">{store.name}</span>
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
            </Card>
          </li>
        ))}
      </ul>

      {selectedStoreId && <StoreCategoryOrder storeId={selectedStoreId} />}
    </div>
  )
}

function StoreCategoryOrder({ storeId }: { storeId: string }) {
  const { data: storeCategoriesData } = useStoreCategories(storeId)
  const { data: categoriesData } = useCategories()
  const [order, setOrder] = useState<OrderedCategory[]>([])
  const [saved, setSaved] = useState(false)
  const [busy, setBusy] = useState(false)

  // Initialize order once per storeId so SWR refetches don't wipe local reordering.
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
    // Small activation distance so taps still reach the × button.
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
      await enqueueWrite(
        setStoreCategoriesWrite,
        { storeId, categoryIds: order.map((o) => o.id) },
        Object.fromEntries(order.map((o) => [o.id, o.name]))
      )
      setSaved(true)
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card className="space-y-3 p-3">
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
    </Card>
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
    <li ref={setNodeRef} style={style}>
      <Card variant="inset" className="flex items-center gap-2 rounded-xl p-2">
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
        <span className="min-w-0 flex-1 break-words text-sm text-fg">{category.name}</span>
        <Button
          size="sm"
          variant="ghost"
          onClick={() => onRemove(category.id)}
          aria-label={`Remove ${category.name}`}
        >
          ×
        </Button>
      </Card>
    </li>
  )
}
