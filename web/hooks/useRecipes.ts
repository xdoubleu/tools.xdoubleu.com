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
import type { ListRecipesResponse, GetRecipeResponse } from '@/lib/gen/recipes/v1/recipes_pb'
import { DEFAULT_PAGE_SIZE } from '@/lib/pagination'
import { enqueueWrite, flushOutbox, sendWrite } from '@/lib/offline/outbox'
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

/** Queues a delete; a rejection shows in the offline banner instead of throwing. */
export function useDeleteRecipe() {
  return async (req: DeleteRecipeInput) => {
    await enqueueWrite(deleteRecipeWrite, req)
    await flushOutbox()
  }
}
