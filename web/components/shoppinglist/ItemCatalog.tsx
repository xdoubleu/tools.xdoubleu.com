'use client'

import { useMemo, useState } from 'react'
import { useItemNames, useCategories } from '@/hooks/useShoppingList'
import { createServiceClient } from '@/lib/client'
import { ShoppingListService } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'
import type { ItemName } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'
import { Card } from '@/components/ui/card'
import { Select } from '@/components/ui/select'
import { Checkbox } from '@/components/ui/checkbox'
import { Collapsible } from '@/components/ui/collapsible'
import { EmptyState, LoadingState } from '@/components/ui/states'
import { cn } from '@/lib/cn'

const NOT_EXPORTED = 'Not exported'
const UNASSIGNED = 'Unassigned'

interface CatalogGroup {
  key: string
  title: string
  items: ItemName[]
}

// "Unassigned" first, then categories alphabetically, then "Not exported"
// (collapsed) for every excluded name.
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

  if (isLoading) return <LoadingState className="text-sm" />
  if (names.length === 0) {
    return <EmptyState>No items yet. Add custom items or recipe ingredients first.</EmptyState>
  }

  return (
    <div className="space-y-3">
      {error && <p className="text-sm text-danger">{error}</p>}
      {groups.map((group) => (
        <Collapsible
          key={group.key}
          defaultCollapsed={group.key === NOT_EXPORTED}
          triggerClassName="text-sm"
          title={
            <>
              {group.title}
              <span className="text-xs font-normal text-muted">({group.items.length})</span>
            </>
          }
        >
          <ul className="space-y-2">
            {group.items.map((item) => (
              <li key={item.name}>
                <Card variant="inset" className="flex flex-wrap items-center gap-2 p-2">
                  <span
                    className={cn(
                      'min-w-0 flex-1 basis-32 break-words text-sm',
                      item.excluded ? 'text-muted line-through' : 'text-fg'
                    )}
                  >
                    {item.name}
                  </span>
                  {!item.excluded && (
                    <Select
                      aria-label={`Category for ${item.name}`}
                      value={item.categoryId}
                      onChange={(e) => handleCategoryChange(item.name, e.target.value)}
                      className="w-auto min-w-0 max-w-full px-2"
                    >
                      <option value="">-- Unassigned --</option>
                      {categories.map((category) => (
                        <option key={category.id} value={category.id}>
                          {category.name}
                        </option>
                      ))}
                    </Select>
                  )}
                  <Checkbox
                    aria-label={`Export ${item.name} to list`}
                    checked={!item.excluded}
                    onChange={(e) => handleExcludedChange(item.name, !e.target.checked)}
                    label={<span className="text-xs text-muted">Export</span>}
                    labelClassName="shrink-0"
                  />
                </Card>
              </li>
            ))}
          </ul>
        </Collapsible>
      ))}
    </div>
  )
}
