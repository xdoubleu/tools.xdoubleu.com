'use client'

import Link from 'next/link'
import FeedForm from '@/components/icsproxy/FeedForm'

export default function NewFeedPage() {
  return (
    <main className="max-w-4xl mx-auto p-6">
      <div className="flex items-center gap-4 mb-6">
        <Link href="/icsproxy" className="text-blue-600 hover:underline text-sm">
          &larr; ICS Proxy
        </Link>
        <h1 className="text-3xl font-bold">New Feed</h1>
      </div>
      <FeedForm />
    </main>
  )
}
