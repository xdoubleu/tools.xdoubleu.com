'use client'

import { useRouter } from 'next/navigation'
import { useLearningPath } from '@/hooks/useLearningPaths'
import LearningPathForm from '@/components/learningpaths/LearningPathForm'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { PageContainer } from '@/components/ui/page-container'

export default function EditLearningPathClient({ id }: { id: string }) {
  const { data, isLoading } = useLearningPath(id)
  const router = useRouter()
  const learningPath = data?.learningPath

  return (
    <PageContainer className="max-w-2xl p-6">
      <Breadcrumb
        className="mb-4"
        items={[
          { label: 'Learning Paths', href: '/learningpaths/list' },
          { label: learningPath?.title ?? 'Edit', href: `/learningpaths/${id}` },
          { label: 'Edit' }
        ]}
      />
      <h1 className="text-3xl font-bold mb-6">Edit Learning Path</h1>
      {isLoading && !learningPath && <p className="text-muted">Loading…</p>}
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
