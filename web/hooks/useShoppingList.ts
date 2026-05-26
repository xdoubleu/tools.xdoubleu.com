import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { ShoppingListService } from '@/lib/gen/shoppinglist/v1/shoppinglist_connect'
import type { GetShoppingListResponse } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'

export function useShoppingList(planId: string) {
  const client = createServiceClient(ShoppingListService)
  const swr = useSWR<GetShoppingListResponse, Error>(
    planId ? `/shoppinglist/${planId}` : null,
    () => client.getShoppingList({ planId })
  )
  return swr
}
