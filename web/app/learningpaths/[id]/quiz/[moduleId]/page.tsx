import QuizClient from './QuizClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { LearningPathsService } from '@/lib/gen/learningpaths/v1/learningpaths_pb'

export default async function ModuleQuizPage({
  params
}: {
  params: Promise<{ id: string; moduleId: string }>
}) {
  const { id, moduleId } = await params
  const client = await createServerClient(LearningPathsService)
  const learningPath = await fetchOrNull(() => client.getLearningPath({ id }))

  return (
    <SWRFallback fallback={learningPath ? { [swrKeys.learningPath(id)]: learningPath } : {}}>
      <QuizClient id={id} moduleId={moduleId} />
    </SWRFallback>
  )
}
