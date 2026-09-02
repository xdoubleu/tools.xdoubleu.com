import RecipesListClient from '@/components/recipes/RecipesListClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { RecipesService } from '@/lib/gen/recipes/v1/recipes_pb'
import { DEFAULT_PAGE_SIZE } from '@/lib/pagination'

export default async function RecipesListPage() {
  const client = await createServerClient(RecipesService)
  const recipes = await fetchOrNull(() => client.listRecipes({ limit: DEFAULT_PAGE_SIZE }))

  return (
    <SWRFallback fallback={recipes ? { [swrKeys.recipes]: recipes } : {}}>
      <RecipesListClient />
    </SWRFallback>
  )
}
