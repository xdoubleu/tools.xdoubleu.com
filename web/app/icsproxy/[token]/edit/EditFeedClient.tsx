'use client'

import { useICSConfig } from '@/hooks/useICSProxy'
import FeedForm from '@/components/icsproxy/FeedForm'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { PageContainer } from '@/components/ui/page-container'

interface EditFeedClientProps {
  token: string
}

export default function EditFeedClient({ token }: EditFeedClientProps) {
  const { data, error, isLoading } = useICSConfig(token)

  if (isLoading) return <p>Loading...</p>
  if (error) return <p className="text-danger">Failed to load feed config.</p>

  return (
    <PageContainer className="p-6">
      <Breadcrumb
        className="mb-4"
        items={[{ label: 'ICS Proxy', href: '/icsproxy' }, { label: 'Edit Feed' }]}
      />
      <h1 className="text-3xl font-bold mb-6">Edit Feed</h1>
      <FeedForm token={token} initialConfig={data?.config} initialEvents={data?.events} />
    </PageContainer>
  )
}
