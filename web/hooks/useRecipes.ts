import useSWR from 'swr'
import type { MessageInitShape } from '@bufbuild/protobuf'
import { createServiceClient } from '@/lib/client'
import {
  RecipesService,
  CreateRecipeRequestSchema,
  UpdateRecipeRequestSchema,
  DeleteRecipeRequestSchema,
  ShareRecipeRequestSchema,
  UnshareRecipeRequestSchema
} from '@/lib/gen/recipes/v1/recipes_pb'
import type { ListRecipesResponse, GetRecipeResponse } from '@/lib/gen/recipes/v1/recipes_pb'

export type CreateRecipeInput = MessageInitShape<typeof CreateRecipeRequestSchema>
export type UpdateRecipeInput = MessageInitShape<typeof UpdateRecipeRequestSchema>
export type DeleteRecipeInput = MessageInitShape<typeof DeleteRecipeRequestSchema>
export type ShareRecipeInput = MessageInitShape<typeof ShareRecipeRequestSchema>
export type UnshareRecipeInput = MessageInitShape<typeof UnshareRecipeRequestSchema>

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
  return (req: CreateRecipeInput) => client.createRecipe(req)
}

export function useUpdateRecipe() {
  const client = createServiceClient(RecipesService)
  return (req: UpdateRecipeInput) => client.updateRecipe(req)
}

export function useDeleteRecipe() {
  const client = createServiceClient(RecipesService)
  return (req: DeleteRecipeInput) => client.deleteRecipe(req)
}

export function useShareRecipe() {
  const client = createServiceClient(RecipesService)
  return (req: ShareRecipeInput) => client.shareRecipe(req)
}

export function useUnshareRecipe() {
  const client = createServiceClient(RecipesService)
  return (req: UnshareRecipeInput) => client.unshareRecipe(req)
}
