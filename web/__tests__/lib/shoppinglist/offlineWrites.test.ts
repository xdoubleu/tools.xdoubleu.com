import { create, type DescMessage, type MessageInitShape } from '@bufbuild/protobuf'
import type { OfflineWrite } from '@/lib/offline/registry'
import { swrKeys } from '@/lib/swrKeys'
import {
  GetCustomListResponseSchema,
  GetStoreCategoriesResponseSchema,
  ListCategoriesResponseSchema,
  ListItemCategoriesResponseSchema,
  ListItemNamesResponseSchema,
  ListStoresResponseSchema
} from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'
import {
  createCategoryWrite,
  createShoppingItemWrite,
  createStoreWrite,
  deleteCategoryWrite,
  deleteShoppingItemWrite,
  deleteStoreWrite,
  formatAmount,
  renameCategoryWrite,
  renameStoreWrite,
  setItemCategoryWrite,
  setItemExcludedWrite,
  setStoreCategoriesWrite,
  shoppingListWrites,
  updateShoppingItemWrite
} from '@/lib/shoppinglist/offlineWrites'

// Applies a write to `data` under `key` the way the outbox does.
function run<I extends DescMessage>(
  write: OfflineWrite<I>,
  init: MessageInitShape<I>,
  key: unknown,
  data: unknown,
  hint?: unknown
) {
  return write.apply(key, data, write.encode(init), hint)
}

const list = swrKeys.shoppingList('')
const categories = swrKeys.shoppingCategories('')

const customList = () =>
  create(GetCustomListResponseSchema, {
    items: [
      { id: 'a', name: 'Apples', amount: '1', unit: '' },
      { id: 'm', name: 'Milk', amount: '2', unit: 'L' }
    ]
  })

describe('formatAmount', () => {
  it.each([
    ['2.50', '2.5'],
    ['1.23456', '1.235'],
    ['3', '3'],
    ['0', '0'],
    ['', '0'],
    ['-1', '0']
  ])('%s → %s', (raw, formatted) => {
    expect(formatAmount(raw)).toBe(formatted)
  })
})

describe('shopping list offline writes', () => {
  it('leaves unrelated keys and non-message data untouched', () => {
    const data = customList()
    for (const write of shoppingListWrites) {
      expect(write.apply('/recipes', data, new Uint8Array(), undefined)).toBe(data)
    }
    expect(run(deleteShoppingItemWrite, { itemId: 'a' }, list, 'not a message')).toBe(
      'not a message'
    )
  })

  it('adds a created item in name order, idempotently', () => {
    const once = run(
      createShoppingItemWrite,
      { id: 'b', name: 'Bread', amount: '1.50', unit: 'loaf' },
      list,
      customList()
    )
    const twice = run(
      createShoppingItemWrite,
      { id: 'b', name: 'Bread', amount: '1.50', unit: 'loaf' },
      list,
      once
    )

    expect(twice).toEqual(once)
    expect(once).toMatchObject({
      items: [{ id: 'a' }, { id: 'b', name: 'Bread', amount: '1.5', unit: 'loaf' }, { id: 'm' }]
    })
  })

  it('edits and deletes items by id', () => {
    const edited = run(
      updateShoppingItemWrite,
      { itemId: 'm', name: 'Butter', amount: '0', unit: 'g' },
      list,
      customList()
    )
    expect(edited).toMatchObject({
      items: [{ id: 'a' }, { id: 'm', name: 'Butter', amount: '0', unit: 'g' }]
    })

    expect(run(deleteShoppingItemWrite, { itemId: 'a' }, list, customList())).toMatchObject({
      items: [{ id: 'm' }]
    })
  })

  it('creates, renames and deletes categories', () => {
    const data = create(ListCategoriesResponseSchema, {
      categories: [{ id: 'p', name: 'Produce' }]
    })

    const created = run(createCategoryWrite, { id: 'd', name: 'Dairy' }, categories, data)
    expect(created).toMatchObject({ categories: [{ id: 'd' }, { id: 'p' }] })

    const renamed = run(renameCategoryWrite, { id: 'p', name: 'Bakery' }, categories, created)
    expect(renamed).toMatchObject({
      categories: [{ id: 'p', name: 'Bakery' }, { id: 'd' }]
    })

    expect(run(deleteCategoryWrite, { id: 'd' }, categories, renamed)).toMatchObject({
      categories: [{ id: 'p' }]
    })
  })

  it('unassigns a deleted category from the item catalog', () => {
    const names = create(ListItemNamesResponseSchema, {
      names: [
        { name: 'milk', categoryId: 'd' },
        { name: 'kiwi', categoryId: 'p' }
      ]
    })
    expect(run(deleteCategoryWrite, { id: 'd' }, swrKeys.itemNames, names)).toMatchObject({
      names: [
        { name: 'milk', categoryId: '' },
        { name: 'kiwi', categoryId: 'p' }
      ]
    })

    const itemCategories = create(ListItemCategoriesResponseSchema, {
      items: [
        { name: 'milk', categoryId: 'd' },
        { name: 'kiwi', categoryId: 'p' }
      ]
    })
    expect(
      run(deleteCategoryWrite, { id: 'd' }, swrKeys.itemCategories, itemCategories)
    ).toMatchObject({ items: [{ name: 'kiwi' }] })
  })

  it('creates, renames and deletes stores', () => {
    const data = create(ListStoresResponseSchema, { stores: [{ id: 'c', name: 'Colruyt' }] })

    const created = run(createStoreWrite, { id: 'a', name: 'Aldi' }, swrKeys.stores, data)
    expect(created).toMatchObject({ stores: [{ id: 'a' }, { id: 'c' }] })

    const renamed = run(renameStoreWrite, { id: 'a', name: 'Lidl' }, swrKeys.stores, created)
    expect(renamed).toMatchObject({ stores: [{ id: 'c' }, { id: 'a', name: 'Lidl' }] })

    expect(run(deleteStoreWrite, { id: 'c' }, swrKeys.stores, renamed)).toMatchObject({
      stores: [{ id: 'a' }]
    })
  })

  it('reorders store categories, naming new ones from the hint', () => {
    const key = swrKeys.storeCategories('s1')
    const data = create(GetStoreCategoriesResponseSchema, {
      categories: [
        { id: 'v', name: 'Veg' },
        { id: 'd', name: 'Dairy' }
      ]
    })

    expect(
      run(
        setStoreCategoriesWrite,
        { storeId: 's1', categoryIds: ['d', 'v', 'b', 'x'] },
        key,
        data,
        {
          b: 'Bakery'
        }
      )
    ).toMatchObject({
      categories: [
        { id: 'd', name: 'Dairy' },
        { id: 'v', name: 'Veg' },
        { id: 'b', name: 'Bakery' }
      ]
    })
    expect(run(setStoreCategoriesWrite, { storeId: 's2', categoryIds: [] }, key, data)).toBe(data)
    expect(
      run(setStoreCategoriesWrite, { storeId: 's1', categoryIds: ['v'] }, key, data, null)
    ).toMatchObject({ categories: [{ id: 'v' }] })
  })

  it('sets an item’s category in both catalog views', () => {
    const names = create(ListItemNamesResponseSchema, {
      names: [{ name: 'milk', categoryId: '', excluded: true }]
    })
    expect(
      run(setItemCategoryWrite, { name: 'milk', categoryId: 'd' }, swrKeys.itemNames, names)
    ).toMatchObject({ names: [{ name: 'milk', categoryId: 'd', excluded: true }] })
    expect(
      run(setItemCategoryWrite, { name: 'bread', categoryId: 'b' }, swrKeys.itemNames, names)
    ).toMatchObject({
      names: [{ name: 'bread', categoryId: 'b', excluded: false }, { name: 'milk' }]
    })

    const itemCategories = create(ListItemCategoriesResponseSchema, {
      items: [{ name: 'milk', categoryId: 'x' }]
    })
    expect(
      run(
        setItemCategoryWrite,
        { name: 'milk', categoryId: 'd' },
        swrKeys.itemCategories,
        itemCategories
      )
    ).toMatchObject({ items: [{ name: 'milk', categoryId: 'd' }] })
    expect(
      run(
        setItemCategoryWrite,
        { name: 'milk', categoryId: '' },
        swrKeys.itemCategories,
        itemCategories
      )
    ).toMatchObject({ items: [] })
  })

  it('flags an item name as excluded', () => {
    const names = create(ListItemNamesResponseSchema, {
      names: [
        { name: 'milk', excluded: false },
        { name: 'kiwi', excluded: false }
      ]
    })
    expect(
      run(setItemExcludedWrite, { name: 'milk', excluded: true }, swrKeys.itemNames, names)
    ).toMatchObject({
      names: [
        { name: 'milk', excluded: true },
        { name: 'kiwi', excluded: false }
      ]
    })
  })

  it('describes each write for the couldn’t-sync list', () => {
    const describe = <I extends DescMessage>(write: OfflineWrite<I>, init: MessageInitShape<I>) =>
      write.describe(write.encode(init))

    expect(describe(createShoppingItemWrite, { name: 'Milk' })).toBe('Add “Milk”')
    expect(describe(updateShoppingItemWrite, { name: 'Milk' })).toBe('Edit “Milk”')
    expect(describe(deleteShoppingItemWrite, {})).toBe('Delete an item')
    expect(describe(createCategoryWrite, { name: 'Dairy' })).toBe('Add category “Dairy”')
    expect(describe(renameCategoryWrite, { name: 'Veg' })).toBe('Rename category to “Veg”')
    expect(describe(deleteCategoryWrite, {})).toBe('Delete a category')
    expect(describe(createStoreWrite, { name: 'Aldi' })).toBe('Add store “Aldi”')
    expect(describe(renameStoreWrite, { name: 'Lidl' })).toBe('Rename store to “Lidl”')
    expect(describe(deleteStoreWrite, {})).toBe('Delete a store')
    expect(describe(setStoreCategoriesWrite, {})).toBe('Reorder store categories')
    expect(describe(setItemCategoryWrite, { name: 'milk' })).toBe('Set the category of “milk”')
    expect(describe(setItemExcludedWrite, { name: 'milk', excluded: true })).toBe('Remove “milk”')
    expect(describe(setItemExcludedWrite, { name: 'milk' })).toBe('Restore “milk”')
  })
})
