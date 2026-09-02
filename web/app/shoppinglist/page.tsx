import ShoppingListPageClient from '@/components/shoppinglist/ShoppingListPageClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { ShoppingListService } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'

export default async function ShoppingListPage() {
  const client = await createServerClient(ShoppingListService)
  const [list, categories] = await Promise.all([
    fetchOrNull(() => client.getCustomList({})),
    fetchOrNull(() => client.listCategories({}))
  ])

  return (
    <SWRFallback
      fallback={{
        ...(list ? { [swrKeys.shoppingList('')]: list } : {}),
        ...(categories ? { [swrKeys.shoppingCategories('')]: categories } : {})
      }}
    >
      <ShoppingListPageClient />
    </SWRFallback>
  )
}
