import RecipesListClient from '@/components/recipes/RecipesListClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { RecipesService } from '@/lib/gen/recipes/v1/recipes_pb'

export default async function RecipesListPage() {
  const client = await createServerClient(RecipesService)
  const [recipes, shares] = await Promise.all([
    fetchOrNull(() => client.listRecipes({})),
    fetchOrNull(() => client.listRecipeBookShares({}))
  ])

  return (
    <SWRFallback
      fallback={{
        ...(recipes ? { [swrKeys.recipes]: recipes } : {}),
        ...(shares ? { [swrKeys.recipeBookShares]: shares } : {})
      }}
    >
      <RecipesListClient />
    </SWRFallback>
  )
}
