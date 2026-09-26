'use client'

import { useRouter } from 'next/navigation'
import LearningPathForm from '@/components/learningpaths/LearningPathForm'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'

export default function NewLearningPathPage() {
  const router = useRouter()

  return (
    <PageContainer size="form">
      <PageHeader
        title="New Learning Path"
        breadcrumb={[{ label: 'Learning Paths', href: '/learningpaths/list' }, { label: 'New' }]}
      />
      <LearningPathForm
        onSave={(id) => router.push(`/learningpaths/${id}`)}
        onCancel={() => router.push('/learningpaths/list')}
      />
    </PageContainer>
  )
}
