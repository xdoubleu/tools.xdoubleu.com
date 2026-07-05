import SharingPageClient from '@/components/sharing/SharingPageClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { RecipesService } from '@/lib/gen/recipes/v1/recipes_pb'
import { ShoppingListService } from '@/lib/gen/shoppinglist/v1/shoppinglist_pb'
import { MealPlansService } from '@/lib/gen/mealplans/v1/mealplans_pb'

// Server-side version of the client's owned-plan-shares aggregation. Doing the
// listPlans -> getPlan fan-out here turns the browser's N+1 waterfall into
// parallel server-to-server calls for the initial paint.
async function fetchOwnedPlanShares() {
  const client = await createServerClient(MealPlansService)
  const { plans } = await client.listPlans({})
  const detailed = await Promise.all(plans.map((p) => client.getPlan({ id: p.id, offset: 0 })))
  return detailed
    .filter((d) => d.isOwner && d.plan)
    .map((d) => ({
      id: d.plan!.id,
      name: d.plan!.name,
      shares: d.sharedWith.map((u) => ({
        userId: u.userId,
        displayName: u.displayName,
        canEdit: u.canEdit
      }))
    }))
}

export default async function SharingPage() {
  const [recipesClient, shoppingClient] = await Promise.all([
    createServerClient(RecipesService),
    createServerClient(ShoppingListService)
  ])
  const [bookShares, listShares, planShares] = await Promise.all([
    fetchOrNull(() => recipesClient.listRecipeBookShares({})),
    fetchOrNull(() => shoppingClient.listShoppingListShares({})),
    fetchOrNull(() => fetchOwnedPlanShares())
  ])

  return (
    <SWRFallback
      fallback={{
        ...(bookShares ? { [swrKeys.recipeBookShares]: bookShares } : {}),
        ...(listShares ? { [swrKeys.shoppingListShares]: listShares } : {}),
        ...(planShares ? { [swrKeys.sharedMealPlans]: planShares } : {})
      }}
    >
      <SharingPageClient />
    </SWRFallback>
  )
}
