import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { ShoppingListService } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'
import type {
  GetCustomListResponse,
  GetMealPlanExportItemsResponse,
  ListCategoriesResponse,
  ListStoresResponse,
  GetStoreCategoriesResponse,
  ListItemNamesResponse,
  ListItemCategoriesResponse
} from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'

export function useCustomList() {
  const client = createServiceClient(ShoppingListService)
  return useSWR<GetCustomListResponse, Error>('/shoppinglist', () => client.getCustomList({}))
}

export function useMealPlanExportItems(planId: string) {
  const client = createServiceClient(ShoppingListService)
  return useSWR<GetMealPlanExportItemsResponse, Error>(
    planId ? `/shoppinglist/export/${planId}` : null,
    () => client.getMealPlanExportItems({ planId })
  )
}

export function useCategories() {
  const client = createServiceClient(ShoppingListService)
  return useSWR<ListCategoriesResponse, Error>('/shoppinglist/categories', () =>
    client.listCategories({})
  )
}

export function useStores() {
  const client = createServiceClient(ShoppingListService)
  return useSWR<ListStoresResponse, Error>('/shoppinglist/stores', () => client.listStores({}))
}

export function useStoreCategories(storeId: string) {
  const client = createServiceClient(ShoppingListService)
  return useSWR<GetStoreCategoriesResponse, Error>(
    storeId ? `/shoppinglist/stores/${storeId}/categories` : null,
    () => client.getStoreCategories({ storeId })
  )
}

export function useItemNames() {
  const client = createServiceClient(ShoppingListService)
  return useSWR<ListItemNamesResponse, Error>('/shoppinglist/item-names', () =>
    client.listItemNames({})
  )
}

export function useItemCategories() {
  const client = createServiceClient(ShoppingListService)
  return useSWR<ListItemCategoriesResponse, Error>('/shoppinglist/item-categories', () =>
    client.listItemCategories({})
  )
}
