import useSWR from 'swr'
import { useCallback, useMemo } from 'react'
import { swrKeys } from '@/lib/swrKeys'
import { create, type MessageInitShape } from '@bufbuild/protobuf'
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
  Recipe
} from '@/lib/gen/recipes/v1/recipes_pb'
import { DEFAULT_PAGE_SIZE } from '@/lib/pagination'
import { enqueueWrite, sendWrite } from '@/lib/offline/outbox'
import {
  createRecipeWrite,
  deleteRecipeWrite,
  updateRecipeWrite
} from '@/lib/recipes/offlineWrites'

export type CreateRecipeInput = MessageInitShape<typeof CreateRecipeRequestSchema>
export type UpdateRecipeInput = MessageInitShape<typeof UpdateRecipeRequestSchema>
export type DeleteRecipeInput = MessageInitShape<typeof DeleteRecipeRequestSchema>

export function useRecipes() {
  const client = createServiceClient(RecipesService)
  return useSWR<ListRecipesResponse, Error>(swrKeys.recipes, () =>
    client.listRecipes({ limit: DEFAULT_PAGE_SIZE })
  )
}

// Server-side cap on ListRecipes' limit.
const MAX_PAGE_SIZE = 200

/** Every recipe in the family, for pickers and lookups that can't page. */
export function useAllRecipes() {
  const client = createServiceClient(RecipesService)
  return useSWR<Recipe[], Error>(swrKeys.allRecipes, async () => {
    const all: Recipe[] = []
    for (;;) {
      const page = await client.listRecipes({ limit: MAX_PAGE_SIZE, offset: all.length })
      all.push(...page.recipes)
      if (!page.hasMore || page.recipes.length === 0) return all
    }
  })
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

// Writes go through the offline outbox and wait for the send attempt, so a
// following navigation sees the server's state when online. A create or edit
// the server rejects throws; one still queued (offline) resolves.

/** Queues a create with a client ID; `synced` is false while it waits offline. */
export function useCreateRecipe() {
  return async (req: CreateRecipeInput) => {
    const msg = create(CreateRecipeRequestSchema, req)
    msg.id = crypto.randomUUID()
    const status = await sendWrite(enqueueWrite(createRecipeWrite, msg))
    return { id: msg.id, synced: status === 'sent' }
  }
}

export function useUpdateRecipe() {
  return async (req: UpdateRecipeInput) => {
    await sendWrite(enqueueWrite(updateRecipeWrite, req))
  }
}

/** Queues a delete; throws a `WriteRejectedError` if the server refuses it. */
export function useDeleteRecipe() {
  return async (req: DeleteRecipeInput) => {
    await sendWrite(enqueueWrite(deleteRecipeWrite, req))
  }
}
