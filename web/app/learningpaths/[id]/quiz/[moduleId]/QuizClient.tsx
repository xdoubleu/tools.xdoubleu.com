'use client'

import Link from 'next/link'
import { useLearningPath, useRecordItemProgress } from '@/hooks/useLearningPaths'
import ModuleQuiz from '@/components/learningpaths/ModuleQuiz'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'
import { moduleQuiz } from '@/lib/learningpaths/quiz'

export default function QuizClient({ id, moduleId }: { id: string; moduleId: string }) {
  const { data, error, isLoading, mutate } = useLearningPath(id)
  const recordItemProgress = useRecordItemProgress()

  const learningPath = data?.learningPath
  const quizModule = learningPath?.modules.find((m) => m.id === moduleId)
  const pathHref = `/learningpaths/${id}`

  const completeCheckpoint = async (itemId: string) => {
    await recordItemProgress({ itemId, completed: true })
    await mutate()
  }

  return (
    <PageContainer size="form">
      <PageHeader
        title={quizModule ? `Quiz: ${quizModule.title}` : 'Quiz'}
        breadcrumb={[
          { label: 'Learning Paths', href: '/learningpaths/list' },
          { label: learningPath?.title ?? 'Learning Path', href: pathHref },
          { label: 'Quiz' }
        ]}
      />

      {isLoading && !learningPath && <LoadingState label="quiz" />}
      {error && !learningPath && <ErrorState what="quiz" />}
      {learningPath && (!quizModule || !moduleQuiz(quizModule)) && (
        <EmptyState
          action={
            <Button asChild variant="secondary" size="sm">
              <Link href={pathHref}>Back to path</Link>
            </Button>
          }
        >
          This module has no quiz.
        </EmptyState>
      )}
      {quizModule && moduleQuiz(quizModule) && (
        <>
          <Card variant="inset" className="mb-4">
            <ModuleQuiz module={quizModule} onPassed={completeCheckpoint} />
          </Card>
          <Button asChild variant="secondary" size="sm">
            <Link href={pathHref}>Back to path</Link>
          </Button>
        </>
      )}
    </PageContainer>
  )
}
