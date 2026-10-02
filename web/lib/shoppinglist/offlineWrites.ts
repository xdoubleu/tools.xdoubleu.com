import { create, isMessage, type DescMessage, type MessageShape } from '@bufbuild/protobuf'
import { defineOfflineWrite } from '@/lib/offline/registry'
import { swrKeys } from '@/lib/swrKeys'
import {
  CategorySchema,
  GetCustomListResponseSchema,
  GetStoreCategoriesResponseSchema,
  ItemCategorySchema,
  ItemNameSchema,
  ListCategoriesResponseSchema,
  ListItemCategoriesResponseSchema,
  ListItemNamesResponseSchema,
  ListStoresResponseSchema,
  ShoppingItemSchema,
  ShoppingListService,
  StoreSchema
} from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'

const { method } = ShoppingListService
const REVALIDATE = '/shoppinglist'

const byName = (a: { name: string }, b: { name: string }) => a.name.localeCompare(b.name)

/** Mirrors the API's amount formatting (up to 3 decimals, "0" when unset). */
export function formatAmount(raw: string): string {
  const n = Number.parseFloat(raw)
  return n > 0 ? String(Number(n.toFixed(3))) : '0'
}

/** Runs `fn` when `key` is `expected` and `data` is a `schema` message. */
function onKey<D extends DescMessage>(
  key: unknown,
  expected: string,
  schema: D,
  data: unknown,
  fn: (data: MessageShape<D>) => MessageShape<D>
): unknown {
  return key === expected && isMessage(data, schema) ? fn(data) : data
}

function upsert<T extends { id: string; name: string }>(list: T[], entry: T): T[] {
  return [...list.filter((e) => e.id !== entry.id), entry].sort(byName)
}

const customList = swrKeys.shoppingList('')
const categoriesKey = swrKeys.shoppingCategories('')

export const createShoppingItemWrite = defineOfflineWrite({
  method: method.createShoppingItem,
  apply: (key, data, req) =>
    onKey(key, customList, GetCustomListResponseSchema, data, (d) => ({
      ...d,
      items: upsert(
        d.items,
        create(ShoppingItemSchema, {
          id: req.id,
          name: req.name,
          amount: formatAmount(req.amount),
          unit: req.unit
        })
      )
    })),
  describe: (req) => `Add “${req.name}”`,
  revalidate: REVALIDATE
})

export const updateShoppingItemWrite = defineOfflineWrite({
  method: method.updateShoppingItem,
  apply: (key, data, req) =>
    onKey(key, customList, GetCustomListResponseSchema, data, (d) => ({
      ...d,
      items: d.items
        .map((i) =>
          i.id === req.itemId
            ? { ...i, name: req.name, amount: formatAmount(req.amount), unit: req.unit }
            : i
        )
        .sort(byName)
    })),
  describe: (req) => `Edit “${req.name}”`,
  revalidate: REVALIDATE
})

export const deleteShoppingItemWrite = defineOfflineWrite({
  method: method.deleteShoppingItem,
  apply: (key, data, req) =>
    onKey(key, customList, GetCustomListResponseSchema, data, (d) => ({
      ...d,
      items: d.items.filter((i) => i.id !== req.itemId)
    })),
  describe: () => 'Delete an item',
  revalidate: REVALIDATE
})

export const createCategoryWrite = defineOfflineWrite({
  method: method.createCategory,
  apply: (key, data, req) =>
    onKey(key, categoriesKey, ListCategoriesResponseSchema, data, (d) => ({
      ...d,
      categories: upsert(d.categories, create(CategorySchema, { id: req.id, name: req.name }))
    })),
  describe: (req) => `Add category “${req.name}”`,
  revalidate: REVALIDATE
})

export const renameCategoryWrite = defineOfflineWrite({
  method: method.renameCategory,
  apply: (key, data, req) =>
    onKey(key, categoriesKey, ListCategoriesResponseSchema, data, (d) => ({
      ...d,
      categories: d.categories
        .map((c) => (c.id === req.id ? { ...c, name: req.name } : c))
        .sort(byName)
    })),
  describe: (req) => `Rename category to “${req.name}”`,
  revalidate: REVALIDATE
})

export const deleteCategoryWrite = defineOfflineWrite({
  method: method.deleteCategory,
  apply: (key, data, req) => {
    let next = onKey(key, categoriesKey, ListCategoriesResponseSchema, data, (d) => ({
      ...d,
      categories: d.categories.filter((c) => c.id !== req.id)
    }))
    next = onKey(key, swrKeys.itemNames, ListItemNamesResponseSchema, next, (d) => ({
      ...d,
      names: d.names.map((n) => (n.categoryId === req.id ? { ...n, categoryId: '' } : n))
    }))
    return onKey(key, swrKeys.itemCategories, ListItemCategoriesResponseSchema, next, (d) => ({
      ...d,
      items: d.items.filter((i) => i.categoryId !== req.id)
    }))
  },
  describe: () => 'Delete a category',
  revalidate: REVALIDATE
})

export const createStoreWrite = defineOfflineWrite({
  method: method.createStore,
  apply: (key, data, req) =>
    onKey(key, swrKeys.stores, ListStoresResponseSchema, data, (d) => ({
      ...d,
      stores: upsert(d.stores, create(StoreSchema, { id: req.id, name: req.name }))
    })),
  describe: (req) => `Add store “${req.name}”`,
  revalidate: REVALIDATE
})

export const renameStoreWrite = defineOfflineWrite({
  method: method.renameStore,
  apply: (key, data, req) =>
    onKey(key, swrKeys.stores, ListStoresResponseSchema, data, (d) => ({
      ...d,
      stores: d.stores.map((s) => (s.id === req.id ? { ...s, name: req.name } : s)).sort(byName)
    })),
  describe: (req) => `Rename store to “${req.name}”`,
  revalidate: REVALIDATE
})

export const deleteStoreWrite = defineOfflineWrite({
  method: method.deleteStore,
  apply: (key, data, req) =>
    onKey(key, swrKeys.stores, ListStoresResponseSchema, data, (d) => ({
      ...d,
      stores: d.stores.filter((s) => s.id !== req.id)
    })),
  describe: () => 'Delete a store',
  revalidate: REVALIDATE
})

function isNameMap(hint: unknown): hint is Record<string, string> {
  return typeof hint === 'object' && hint !== null
}

/** Hint: category id → name, since the request carries only ids. */
export const setStoreCategoriesWrite = defineOfflineWrite({
  method: method.setStoreCategories,
  apply: (key, data, req, hint) =>
    onKey(
      key,
      swrKeys.storeCategories(req.storeId),
      GetStoreCategoriesResponseSchema,
      data,
      (d) => {
        const names = new Map(d.categories.map((c) => [c.id, c.name]))
        const hinted = isNameMap(hint) ? hint : {}
        return {
          ...d,
          categories: req.categoryIds.flatMap((id) => {
            const name = hinted[id] ?? names.get(id)
            return name === undefined ? [] : [create(CategorySchema, { id, name })]
          })
        }
      }
    ),
  describe: () => 'Reorder store categories',
  revalidate: REVALIDATE
})

export const setItemCategoryWrite = defineOfflineWrite({
  method: method.setItemCategory,
  apply: (key, data, req) => {
    const next = onKey(key, swrKeys.itemNames, ListItemNamesResponseSchema, data, (d) => {
      const existing = d.names.find((n) => n.name === req.name)
      const entry = create(ItemNameSchema, {
        name: req.name,
        categoryId: req.categoryId,
        excluded: existing?.excluded ?? false
      })
      return { ...d, names: [...d.names.filter((n) => n.name !== req.name), entry].sort(byName) }
    })
    return onKey(key, swrKeys.itemCategories, ListItemCategoriesResponseSchema, next, (d) => {
      const rest = d.items.filter((i) => i.name !== req.name)
      const entry = create(ItemCategorySchema, { name: req.name, categoryId: req.categoryId })
      return { ...d, items: req.categoryId ? [...rest, entry].sort(byName) : rest }
    })
  },
  describe: (req) => `Set the category of “${req.name}”`,
  revalidate: REVALIDATE
})

export const setItemExcludedWrite = defineOfflineWrite({
  method: method.setItemExcluded,
  apply: (key, data, req) =>
    onKey(key, swrKeys.itemNames, ListItemNamesResponseSchema, data, (d) => ({
      ...d,
      names: d.names.map((n) => (n.name === req.name ? { ...n, excluded: req.excluded } : n))
    })),
  describe: (req) => `${req.excluded ? 'Remove' : 'Restore'} “${req.name}”`,
  revalidate: REVALIDATE
})

export const shoppingListWrites = [
  createShoppingItemWrite,
  updateShoppingItemWrite,
  deleteShoppingItemWrite,
  createCategoryWrite,
  renameCategoryWrite,
  deleteCategoryWrite,
  createStoreWrite,
  renameStoreWrite,
  deleteStoreWrite,
  setStoreCategoriesWrite,
  setItemCategoryWrite,
  setItemExcludedWrite
]
