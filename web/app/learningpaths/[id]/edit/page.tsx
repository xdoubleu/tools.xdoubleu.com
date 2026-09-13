import EditLearningPathClient from './EditLearningPathClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { LearningPathsService } from '@/lib/gen/learningpaths/v1/learningpaths_pb'

export default async function EditLearningPathPage({
  params
}: {
  params: Promise<{ id: string }>
}) {
  const { id } = await params
  const client = await createServerClient(LearningPathsService)
  const learningPath = await fetchOrNull(() => client.getLearningPath({ id }))

  return (
    <SWRFallback fallback={learningPath ? { [swrKeys.learningPath(id)]: learningPath } : {}}>
      <EditLearningPathClient id={id} />
    </SWRFallback>
  )
}
