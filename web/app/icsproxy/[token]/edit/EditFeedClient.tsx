'use client'

import Link from 'next/link'
import { useICSConfig } from '@/hooks/useICSProxy'
import FeedForm from '@/components/icsproxy/FeedForm'

interface EditFeedClientProps {
  token: string
}

export default function EditFeedClient({ token }: EditFeedClientProps) {
  const { data, error, isLoading } = useICSConfig(token)

  if (isLoading) return <p>Loading...</p>
  if (error) return <p className="text-danger">Failed to load feed config.</p>

  return (
    <main className="max-w-4xl mx-auto p-6">
      <div className="flex items-center gap-4 mb-6">
        <Link href="/icsproxy" className="text-sm text-accent hover:underline">
          &larr; ICS Proxy
        </Link>
        <h1 className="text-3xl font-bold">Edit Feed</h1>
      </div>
      <FeedForm token={token} initialConfig={data?.config} initialEvents={data?.events} />
    </main>
  )
}
