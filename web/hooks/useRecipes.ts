import useSWR from 'swr'
import { useCallback, useMemo } from 'react'
import { swrKeys } from '@/lib/swrKeys'
import type { MessageInitShape } from '@bufbuild/protobuf'
import { createServiceClient } from '@/lib/client'
import {
  RecipesService,
  CreateRecipeRequestSchema,
  UpdateRecipeRequestSchema,
  DeleteRecipeRequestSchema
} from '@/lib/gen/recipes/v1/recipes_pb'
import type { ListRecipesResponse, GetRecipeResponse } from '@/lib/gen/recipes/v1/recipes_pb'
import { DEFAULT_PAGE_SIZE } from '@/lib/pagination'

export type CreateRecipeInput = MessageInitShape<typeof CreateRecipeRequestSchema>
export type UpdateRecipeInput = MessageInitShape<typeof UpdateRecipeRequestSchema>
export type DeleteRecipeInput = MessageInitShape<typeof DeleteRecipeRequestSchema>

export function useRecipes() {
  const client = createServiceClient(RecipesService)
  return useSWR<ListRecipesResponse, Error>(swrKeys.recipes, () =>
    client.listRecipes({ limit: DEFAULT_PAGE_SIZE })
  )
}

export function useFetchRecipesPage() {
  const client = useMemo(() => createServiceClient(RecipesService), [])
  return useCallback(
    (offset: number) =>
      client
        .listRecipes({ limit: DEFAULT_PAGE_SIZE, offset })
        .then((r) => ({ items: r.recipes, hasMore: r.hasMore })),
    [client]
  )
}

export function useRecipe(id: string, servings?: number) {
  const client = createServiceClient(RecipesService)
  const key = id ? swrKeys.recipe(id, servings) : null
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
