import ShoppingListPageClient from '@/components/shoppinglist/ShoppingListPageClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { ShoppingListService } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'

export default async function ShoppingListPage() {
  const client = await createServerClient(ShoppingListService)
  const [accessible, list, categories, shares] = await Promise.all([
    fetchOrNull(() => client.listAccessibleLists({})),
    fetchOrNull(() => client.getCustomList({ ownerUserId: '' })),
    fetchOrNull(() => client.listCategories({ ownerUserId: '' })),
    fetchOrNull(() => client.listShoppingListShares({}))
  ])

  return (
    <SWRFallback
      fallback={{
        ...(accessible ? { [swrKeys.accessibleShoppingLists]: accessible } : {}),
        ...(list ? { [swrKeys.shoppingList('')]: list } : {}),
        ...(categories ? { [swrKeys.shoppingCategories('')]: categories } : {}),
        ...(shares ? { [swrKeys.shoppingListShares]: shares } : {})
      }}
    >
      <ShoppingListPageClient />
    </SWRFallback>
  )
}
