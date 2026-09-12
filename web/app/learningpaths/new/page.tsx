'use client'

import { useRouter } from 'next/navigation'
import LearningPathForm from '@/components/learningpaths/LearningPathForm'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { PageContainer } from '@/components/ui/page-container'

export default function NewLearningPathPage() {
  const router = useRouter()

  return (
    <PageContainer className="max-w-2xl p-6">
      <Breadcrumb
        className="mb-4"
        items={[{ label: 'Learning Paths', href: '/learningpaths/list' }, { label: 'New' }]}
      />
      <h1 className="text-3xl font-bold mb-6">New Learning Path</h1>
      <LearningPathForm
        onSave={(id) => router.push(`/learningpaths/${id}`)}
        onCancel={() => router.push('/learningpaths/list')}
      />
    </PageContainer>
  )
}
