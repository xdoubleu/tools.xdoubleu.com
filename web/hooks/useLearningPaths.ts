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
  RecordItemProgressRequestSchema
} from '@/lib/gen/learningpaths/v1/learningpaths_pb'
import type {
  ListLearningPathsResponse,
  GetLearningPathResponse
} from '@/lib/gen/learningpaths/v1/learningpaths_pb'
import { DEFAULT_PAGE_SIZE } from '@/lib/pagination'

export type CreateLearningPathInput = MessageInitShape<typeof CreateLearningPathRequestSchema>
export type UpdateLearningPathInput = MessageInitShape<typeof UpdateLearningPathRequestSchema>
export type DeleteLearningPathInput = MessageInitShape<typeof DeleteLearningPathRequestSchema>
export type RecordItemProgressInput = MessageInitShape<typeof RecordItemProgressRequestSchema>

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

export function useRecordItemProgress() {
  const client = createServiceClient(LearningPathsService)
  return (req: RecordItemProgressInput) => client.recordItemProgress(req)
}
