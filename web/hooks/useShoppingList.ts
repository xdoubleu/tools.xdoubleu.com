import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { ShoppingListService } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'
import type {
  GetCustomListResponse,
  GetMealPlanExportItemsResponse
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
