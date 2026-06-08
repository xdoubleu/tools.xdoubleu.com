'use client'

import FeedForm from '@/components/icsproxy/FeedForm'
import { Breadcrumb } from '@/components/ui/breadcrumb'

export default function NewFeedPage() {
  return (
    <main className="max-w-4xl mx-auto p-6">
      <Breadcrumb
        className="mb-4"
        items={[{ label: 'ICS Proxy', href: '/icsproxy' }, { label: 'New Feed' }]}
      />
      <h1 className="text-3xl font-bold mb-6">New Feed</h1>
      <FeedForm />
    </main>
  )
}
