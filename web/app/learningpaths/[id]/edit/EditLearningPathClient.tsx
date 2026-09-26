'use client'

import { useRouter } from 'next/navigation'
import { useLearningPath } from '@/hooks/useLearningPaths'
import LearningPathForm from '@/components/learningpaths/LearningPathForm'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { LoadingState } from '@/components/ui/states'

export default function EditLearningPathClient({ id }: { id: string }) {
  const { data, isLoading } = useLearningPath(id)
  const router = useRouter()
  const learningPath = data?.learningPath

  return (
    <PageContainer size="form">
      <PageHeader
        title="Edit Learning Path"
        breadcrumb={[
          { label: 'Learning Paths', href: '/learningpaths/list' },
          { label: learningPath?.title ?? 'Edit', href: `/learningpaths/${id}` },
          { label: 'Edit' }
        ]}
      />
      {isLoading && !learningPath && <LoadingState />}
      {learningPath && (
        <LearningPathForm
          learningPath={learningPath}
          onSave={(savedId) => router.push(`/learningpaths/${savedId}`)}
          onCancel={() => router.push(`/learningpaths/${id}`)}
        />
      )}
    </PageContainer>
  )
}
