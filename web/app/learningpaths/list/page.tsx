import LearningPathsListClient from '@/components/learningpaths/LearningPathsListClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { LearningPathsService } from '@/lib/gen/learningpaths/v1/learningpaths_pb'
import { DEFAULT_PAGE_SIZE } from '@/lib/pagination'

export default async function LearningPathsListPage() {
  const client = await createServerClient(LearningPathsService)
  const learningPaths = await fetchOrNull(() =>
    client.listLearningPaths({ limit: DEFAULT_PAGE_SIZE })
  )

  return (
    <SWRFallback fallback={learningPaths ? { [swrKeys.learningPaths]: learningPaths } : {}}>
      <LearningPathsListClient />
    </SWRFallback>
  )
}
