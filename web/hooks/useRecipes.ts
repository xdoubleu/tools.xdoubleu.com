import useSWR from 'swr'
import type { MessageInitShape } from '@bufbuild/protobuf'
import { createServiceClient } from '@/lib/client'
import {
  RecipesService,
  CreateRecipeRequestSchema,
  UpdateRecipeRequestSchema,
  DeleteRecipeRequestSchema
} from '@/lib/gen/recipes/v1/recipes_pb'
import type {
  ListRecipesResponse,
  GetRecipeResponse,
  ListRecipeBookSharesResponse
} from '@/lib/gen/recipes/v1/recipes_pb'

export type CreateRecipeInput = MessageInitShape<typeof CreateRecipeRequestSchema>
export type UpdateRecipeInput = MessageInitShape<typeof UpdateRecipeRequestSchema>
export type DeleteRecipeInput = MessageInitShape<typeof DeleteRecipeRequestSchema>

export function useRecipes() {
  const client = createServiceClient(RecipesService)
  return useSWR<ListRecipesResponse, Error>('/recipes', () => client.listRecipes({}))
}

export function useRecipe(id: string, servings?: number) {
  const client = createServiceClient(RecipesService)
  const key = id ? (servings ? `/recipes/${id}?servings=${servings}` : `/recipes/${id}`) : null
  return useSWR<GetRecipeResponse, Error>(
    key,
    () => client.getRecipe({ id, servings: servings ?? 0 }),
    {
      keepPreviousData: true
    }
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

export function useRecipeBookShares() {
  const client = createServiceClient(RecipesService)
  return useSWR<ListRecipeBookSharesResponse, Error>('/recipes/book-shares', () =>
    client.listRecipeBookShares({})
  )
}

export function useShareRecipeBook() {
  const client = createServiceClient(RecipesService)
  return (contactUserId: string, canEdit: boolean) =>
    client.shareRecipeBook({ contactUserId, canEdit })
}

export function useUnshareRecipeBook() {
  const client = createServiceClient(RecipesService)
  return (targetUserId: string) => client.unshareRecipeBook({ targetUserId })
}
