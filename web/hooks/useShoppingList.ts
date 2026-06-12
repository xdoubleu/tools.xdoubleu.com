import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { ShoppingListService } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'
import type {
  GetCustomListResponse,
  GetMealPlanExportItemsResponse,
  GetPlanIngredientGroupsResponse,
  ListCategoriesResponse,
  ListStoresResponse,
  GetStoreCategoriesResponse,
  ListItemNamesResponse,
  ListItemCategoriesResponse,
  ListShoppingListSharesResponse,
  ListAccessibleListsResponse
} from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'
import { MealPlansService } from '@/lib/gen/mealplans/v1/mealplans_pb'

export function useCustomList(ownerUserId = '') {
  const client = createServiceClient(ShoppingListService)
  return useSWR<GetCustomListResponse, Error>(`/shoppinglist?owner=${ownerUserId}`, () =>
    client.getCustomList({ ownerUserId })
  )
}

export function useMealPlanExportItems(planId: string, excludedGroups: string[] = []) {
  const client = createServiceClient(ShoppingListService)
  const key = planId
    ? `/shoppinglist/export/${planId}?excluded=${excludedGroups.sort().join(',')}`
    : null
  return useSWR<GetMealPlanExportItemsResponse, Error>(key, () =>
    client.getMealPlanExportItems({ planId, excludedGroups })
  )
}

export function usePlanIngredientGroups(planId: string) {
  const client = createServiceClient(ShoppingListService)
  return useSWR<GetPlanIngredientGroupsResponse, Error>(
    planId ? `/shoppinglist/groups/${planId}` : null,
    () => client.getPlanIngredientGroups({ planId })
  )
}

export function useCategories(ownerUserId = '') {
  const client = createServiceClient(ShoppingListService)
  return useSWR<ListCategoriesResponse, Error>(
    `/shoppinglist/categories?owner=${ownerUserId}`,
    () => client.listCategories({ ownerUserId })
  )
}

export function useAccessibleLists() {
  const client = createServiceClient(ShoppingListService)
  return useSWR<ListAccessibleListsResponse, Error>('/shoppinglist/accessible', () =>
    client.listAccessibleLists({})
  )
}

export function useShoppingListShares() {
  const client = createServiceClient(ShoppingListService)
  return useSWR<ListShoppingListSharesResponse, Error>('/shoppinglist/shares', () =>
    client.listShoppingListShares({})
  )
}

export function useShareShoppingList() {
  const client = createServiceClient(ShoppingListService)
  return (contactUserId: string, canEdit: boolean) =>
    client.shareShoppingList({ contactUserId, canEdit })
}

export function useUnshareShoppingList() {
  const client = createServiceClient(ShoppingListService)
  return (targetUserId: string) => client.unshareShoppingList({ targetUserId })
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

export interface AllExportItem {
  name: string
  amount: string
  unit: string
  recipeName: string
  groupName: string
}

export interface AllIngredientGroup {
  recipeName: string
  groupName: string
}

export function useAllMealPlanExportItems(excludedGroups: string[] = []) {
  const mealPlansClient = createServiceClient(MealPlansService)
  const shoppingClient = createServiceClient(ShoppingListService)
  const key = `/shoppinglist/export/all?excluded=${[...excludedGroups].sort().join(',')}`
  return useSWR<{ items: AllExportItem[] }, Error>(key, async () => {
    const plansResp = await mealPlansClient.listPlans({})
    const planIds = plansResp.plans.map((p) => p.id)
    if (planIds.length === 0) return { items: [] }
    const responses = await Promise.all(
      planIds.map((planId) => shoppingClient.getMealPlanExportItems({ planId, excludedGroups }))
    )
    return { items: responses.flatMap((r) => r.items as AllExportItem[]) }
  })
}

export function useAllPlanIngredientGroups() {
  const mealPlansClient = createServiceClient(MealPlansService)
  const shoppingClient = createServiceClient(ShoppingListService)
  return useSWR<{ groups: AllIngredientGroup[] }, Error>('/shoppinglist/groups/all', async () => {
    const plansResp = await mealPlansClient.listPlans({})
    const planIds = plansResp.plans.map((p) => p.id)
    if (planIds.length === 0) return { groups: [] }
    const responses = await Promise.all(
      planIds.map((planId) => shoppingClient.getPlanIngredientGroups({ planId }))
    )
    const seen = new Set<string>()
    const groups: AllIngredientGroup[] = []
    for (const resp of responses) {
      for (const group of resp.groups) {
        if (!seen.has(group.groupName)) {
          seen.add(group.groupName)
          groups.push({ recipeName: group.recipeName, groupName: group.groupName })
        }
      }
    }
    return { groups }
  })
}
