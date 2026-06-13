'use client'

import { useMemo, useState } from 'react'
import { useItemNames, useCategories } from '@/hooks/useShoppingList'
import { createServiceClient } from '@/lib/client'
import { ShoppingListService } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'
import type { ItemName } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'
import { Select } from '@/components/ui/select'

// Excluded items collect under this trailing group; it is collapsed by default.
const NOT_EXPORTED = 'Not exported'
const UNASSIGNED = 'Unassigned'

interface CatalogGroup {
  key: string
  title: string
  items: ItemName[]
}

// Partitions the catalog names into collapsible groups: an "Unassigned" group
// first, then each category alphabetically, then a trailing "Not exported"
// group holding every excluded name regardless of category.
function buildGroups(names: ItemName[], categoryNames: Map<string, string>): CatalogGroup[] {
  const notExported: ItemName[] = []
  const byCategory = new Map<string, ItemName[]>()

  for (const item of names) {
    if (item.excluded) {
      notExported.push(item)
      continue
    }
    const list = byCategory.get(item.categoryId)
    if (list) list.push(item)
    else byCategory.set(item.categoryId, [item])
  }

  const groups: CatalogGroup[] = []
  if (byCategory.has('')) {
    groups.push({ key: '', title: UNASSIGNED, items: byCategory.get('')! })
  }
  const assigned = [...byCategory.keys()].filter((id) => id !== '')
  assigned.sort((a, b) => (categoryNames.get(a) ?? '').localeCompare(categoryNames.get(b) ?? ''))
  for (const id of assigned) {
    groups.push({ key: id, title: categoryNames.get(id) ?? id, items: byCategory.get(id)! })
  }
  if (notExported.length > 0) {
    groups.push({ key: NOT_EXPORTED, title: NOT_EXPORTED, items: notExported })
  }
  return groups
}

export default function ItemCatalog() {
  const { data: namesData, isLoading, mutate } = useItemNames()
  const { data: categoriesData } = useCategories()
  const [error, setError] = useState('')
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({
    [NOT_EXPORTED]: true
  })

  const client = createServiceClient(ShoppingListService)
  const names = namesData?.names ?? []
  const categories = categoriesData?.categories ?? []

  const categoryNames = useMemo(() => {
    const map = new Map<string, string>()
    for (const c of categories) map.set(c.id, c.name)
    return map
  }, [categories])

  const groups = useMemo(() => buildGroups(names, categoryNames), [names, categoryNames])

  const handleCategoryChange = async (name: string, categoryId: string) => {
    setError('')
    try {
      await client.setItemCategory({ name, categoryId })
      await mutate()
    } catch {
      setError('Failed to update category.')
    }
  }

  const handleExcludedChange = async (name: string, excluded: boolean) => {
    setError('')
    try {
      await client.setItemExcluded({ name, excluded })
      await mutate()
    } catch {
      setError('Failed to update item.')
    }
  }

  const toggleGroup = (key: string) => setCollapsed((prev) => ({ ...prev, [key]: !prev[key] }))

  if (isLoading) return <p className="text-sm text-muted">Loading…</p>
  if (names.length === 0) {
    return (
      <p className="text-sm text-muted">
        No items yet. Add custom items or recipe ingredients first.
      </p>
    )
  }

  return (
    <div className="space-y-3">
      {error && <p className="text-sm text-danger">{error}</p>}
      {groups.map((group) => {
        const isCollapsed = collapsed[group.key] ?? false
        return (
          <div key={group.key} className="space-y-2">
            <button
              type="button"
              onClick={() => toggleGroup(group.key)}
              aria-expanded={!isCollapsed}
              className="flex w-full items-center gap-2 text-left text-sm font-semibold text-fg"
            >
              <span aria-hidden className="text-muted">
                {isCollapsed ? '▸' : '▾'}
              </span>
              {group.title}
              <span className="text-xs font-normal text-muted">({group.items.length})</span>
            </button>
            {!isCollapsed && (
              <ul className="space-y-2">
                {group.items.map((item) => (
                  <li
                    key={item.name}
                    className="flex items-center gap-2 rounded-2xl border border-border bg-surface p-2"
                  >
                    <span
                      className={`flex-1 text-sm ${item.excluded ? 'text-muted line-through' : 'text-fg'}`}
                    >
                      {item.name}
                    </span>
                    {!item.excluded && (
                      <Select
                        aria-label={`Category for ${item.name}`}
                        value={item.categoryId}
                        onChange={(e) => handleCategoryChange(item.name, e.target.value)}
                        className="h-9 w-auto px-2"
                      >
                        <option value="">-- Unassigned --</option>
                        {categories.map((category) => (
                          <option key={category.id} value={category.id}>
                            {category.name}
                          </option>
                        ))}
                      </Select>
                    )}
                    <label className="flex shrink-0 items-center gap-1.5 text-xs text-muted">
                      <input
                        type="checkbox"
                        aria-label={`Export ${item.name} to list`}
                        checked={!item.excluded}
                        onChange={(e) => handleExcludedChange(item.name, !e.target.checked)}
                        className="size-4 rounded accent-accent"
                      />
                      Export
                    </label>
                  </li>
                ))}
              </ul>
            )}
          </div>
        )
      })}
    </div>
  )
}
