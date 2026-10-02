import useSWR from 'swr'
import { useCallback, useMemo } from 'react'
import { swrKeys } from '@/lib/swrKeys'
import type { MessageInitShape } from '@bufbuild/protobuf'
import { createServiceClient } from '@/lib/client'
import {
  LearningPathsService,
  CreateLearningPathRequestSchema,
  UpdateLearningPathRequestSchema,
  DeleteLearningPathRequestSchema,
  RecordItemProgressRequestSchema,
  SetLearningPathPausedRequestSchema
} from '@/lib/gen/learningpaths/v1/learningpaths_pb'
import type {
  ListLearningPathsResponse,
  GetLearningPathResponse
} from '@/lib/gen/learningpaths/v1/learningpaths_pb'
import { DEFAULT_PAGE_SIZE } from '@/lib/pagination'
import { enqueueWrite, flushOutbox } from '@/lib/offline/outbox'
import { recordItemProgressWrite, setPausedWrite } from '@/lib/learningpaths/offlineWrites'

export type CreateLearningPathInput = MessageInitShape<typeof CreateLearningPathRequestSchema>
export type UpdateLearningPathInput = MessageInitShape<typeof UpdateLearningPathRequestSchema>
export type DeleteLearningPathInput = MessageInitShape<typeof DeleteLearningPathRequestSchema>
export type RecordItemProgressInput = MessageInitShape<typeof RecordItemProgressRequestSchema>
export type SetLearningPathPausedInput = MessageInitShape<typeof SetLearningPathPausedRequestSchema>

export function useLearningPaths() {
  const client = createServiceClient(LearningPathsService)
  return useSWR<ListLearningPathsResponse, Error>(swrKeys.learningPaths, () =>
    client.listLearningPaths({ limit: DEFAULT_PAGE_SIZE })
  )
}

export function useFetchLearningPathsPage() {
  const client = useMemo(() => createServiceClient(LearningPathsService), [])
  return useCallback(
    (offset: number) =>
      client
        .listLearningPaths({ limit: DEFAULT_PAGE_SIZE, offset })
        .then((r) => ({ items: r.learningPaths, hasMore: r.hasMore })),
    [client]
  )
}

export function useLearningPath(id: string) {
  const client = createServiceClient(LearningPathsService)
  const key = id ? swrKeys.learningPath(id) : null
  return useSWR<GetLearningPathResponse, Error>(key, () => client.getLearningPath({ id }))
}

export function useCreateLearningPath() {
  const client = createServiceClient(LearningPathsService)
  return (req: CreateLearningPathInput) => client.createLearningPath(req)
}

export function useUpdateLearningPath() {
  const client = createServiceClient(LearningPathsService)
  return (req: UpdateLearningPathInput) => client.updateLearningPath(req)
}

export function useDeleteLearningPath() {
  const client = createServiceClient(LearningPathsService)
  return (req: DeleteLearningPathInput) => client.deleteLearningPath(req)
}

// Progress and pause go through the offline outbox: they show at once, and a
// rejection (e.g. an item an agent's edit replaced) shows in the offline
// banner and rolls back.

export function useRecordItemProgress() {
  return async (req: RecordItemProgressInput) => {
    await enqueueWrite(recordItemProgressWrite, req)
    await flushOutbox()
  }
}

export function useSetLearningPathPaused() {
  return async (req: SetLearningPathPausedInput) => {
    await enqueueWrite(setPausedWrite, req)
    await flushOutbox()
  }
}
