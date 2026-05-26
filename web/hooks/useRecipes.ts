import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { RecipesService } from '@/lib/gen/recipes/v1/recipes_connect'
import type {
  ListRecipesResponse,
  GetRecipeResponse,
  CreateRecipeRequest,
  UpdateRecipeRequest,
  DeleteRecipeRequest,
  ShareRecipeRequest,
  UnshareRecipeRequest
} from '@/lib/gen/recipes/v1/recipes_pb'

export function useRecipes() {
  const client = createServiceClient(RecipesService)
  return useSWR<ListRecipesResponse, Error>('/recipes', () => client.listRecipes({}))
}

export function useRecipe(id: string, servings?: number) {
  const client = createServiceClient(RecipesService)
  const key = id ? (servings ? `/recipes/${id}?servings=${servings}` : `/recipes/${id}`) : null
  return useSWR<GetRecipeResponse, Error>(key, () =>
    client.getRecipe({ id, servings: servings ?? 0 })
  )
}

export function useCreateRecipe() {
  const client = createServiceClient(RecipesService)
  return (req: CreateRecipeRequest) => client.createRecipe(req)
}

export function useUpdateRecipe() {
  const client = createServiceClient(RecipesService)
  return (req: UpdateRecipeRequest) => client.updateRecipe(req)
}

export function useDeleteRecipe() {
  const client = createServiceClient(RecipesService)
  return (req: DeleteRecipeRequest) => client.deleteRecipe(req)
}

export function useShareRecipe() {
  const client = createServiceClient(RecipesService)
  return (req: ShareRecipeRequest) => client.shareRecipe(req)
}

export function useUnshareRecipe() {
  const client = createServiceClient(RecipesService)
  return (req: UnshareRecipeRequest) => client.unshareRecipe(req)
}
