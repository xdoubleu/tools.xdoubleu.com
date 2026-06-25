'use client'

import FeedForm from '@/components/icsproxy/FeedForm'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { PageContainer } from '@/components/ui/page-container'

export default function NewFeedPage() {
  return (
    <PageContainer className="p-6">
      <Breadcrumb
        className="mb-4"
        items={[{ label: 'ICS Proxy', href: '/icsproxy' }, { label: 'New Feed' }]}
      />
      <h1 className="text-3xl font-bold mb-6">New Feed</h1>
      <FeedForm />
    </PageContainer>
  )
}
