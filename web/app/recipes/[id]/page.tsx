import RecipeClient from './RecipeClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { RecipesService } from '@/lib/gen/recipes/v1/recipes_pb'

export default async function RecipePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  const client = await createServerClient(RecipesService)
  const recipe = await fetchOrNull(() => client.getRecipe({ id, servings: 0 }))

  return (
    <SWRFallback fallback={recipe ? { [swrKeys.recipe(id)]: recipe } : {}}>
      <RecipeClient id={id} />
    </SWRFallback>
  )
}
