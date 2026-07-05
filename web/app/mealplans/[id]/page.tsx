import MealPlanClient from './MealPlanClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { MealPlansService } from '@/lib/gen/mealplans/v1/mealplans_pb'
import { RecipesService } from '@/lib/gen/recipes/v1/recipes_pb'

export default async function MealPlanPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  const [plansClient, recipesClient] = await Promise.all([
    createServerClient(MealPlansService),
    createServerClient(RecipesService)
  ])
  const [plan, recipes] = await Promise.all([
    fetchOrNull(() => plansClient.getPlan({ id, offset: 0 })),
    fetchOrNull(() => recipesClient.listRecipes({}))
  ])

  return (
    <SWRFallback
      fallback={{
        ...(plan ? { [swrKeys.mealPlan(id, 0)]: plan } : {}),
        ...(recipes ? { [swrKeys.recipes]: recipes } : {})
      }}
    >
      <MealPlanClient id={id} />
    </SWRFallback>
  )
}
