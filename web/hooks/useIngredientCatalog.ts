// The ingredient catalog (categories, item names and units) is served by
// shoppinglist but edited from recipes and mealplans too.
import { useMemo } from 'react'
import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { ShoppingListService } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'
import type {
  Category,
  ListCategoriesResponse,
  ListIngredientUnitsResponse,
  ListItemCategoriesResponse,
  ListItemNamesResponse
} from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'
import { swrKeys } from '@/lib/swrKeys'
import { enqueueWrite } from '@/lib/offline/outbox'
import { hasName } from '@/lib/shoppinglist/names'
import { createCategoryWrite, setItemCategoryWrite } from '@/lib/shoppinglist/offlineWrites'

export function useCategories() {
  const client = createServiceClient(ShoppingListService)
  return useSWR<ListCategoriesResponse, Error>(swrKeys.shoppingCategories(''), () =>
    client.listCategories({})
  )
}

export function useItemNames() {
  const client = createServiceClient(ShoppingListService)
  return useSWR<ListItemNamesResponse, Error>(swrKeys.itemNames, () => client.listItemNames({}))
}

/** Merged name → the name it exports as. */
export function useItemAliases(): Record<string, string> {
  const { data } = useItemNames()
  return useMemo(
    () =>
      Object.fromEntries(
        (data?.names ?? []).filter((n) => n.mergedInto).map((n) => [n.name, n.mergedInto])
      ),
    [data]
  )
}

export function useIngredientUnits() {
  const client = createServiceClient(ShoppingListService)
  return useSWR<ListIngredientUnitsResponse, Error>(swrKeys.ingredientUnits, () =>
    client.listIngredientUnits({})
  )
}

export function useItemCategories() {
  const client = createServiceClient(ShoppingListService)
  return useSWR<ListItemCategoriesResponse, Error>(swrKeys.itemCategories, () =>
    client.listItemCategories({})
  )
}

/** Queues a category create, reusing a same-named one, and resolves to its ID. */
export async function queueCategory(name: string, categories: Category[]): Promise<string> {
  const existing = categories.find((c) => hasName([c], name))
  if (existing) return existing.id
  const id = crypto.randomUUID()
  await enqueueWrite(createCategoryWrite, { id, name })
  return id
}

/** Queues a name→category catalog write. */
export async function queueItemCategory(name: string, categoryId: string): Promise<void> {
  await enqueueWrite(setItemCategoryWrite, { name, categoryId })
}
