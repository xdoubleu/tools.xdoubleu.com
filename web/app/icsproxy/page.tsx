'use client'

import { useICSFeeds } from '@/hooks/useICSProxy'
import type { FilterConfig } from '@/lib/gen/icsproxy/v1/proxy_pb'

const BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? ''

function FeedCard({ config }: { config: FilterConfig }) {
  const feedUrl = `${BASE_URL}/icsproxy/${config.token}/feed.ics`

  return (
    <div className="border rounded p-4">
      <p className="text-sm font-mono text-gray-700 break-all">{config.sourceUrl}</p>
      <div className="flex items-center gap-3 mt-3">
        <a
          href={feedUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="text-sm text-blue-600 hover:underline"
        >
          ICS Feed
        </a>
        <span className="text-gray-300">|</span>
        <span className="text-xs text-gray-500">
          {config.hideEventUids.length} hidden event
          {config.hideEventUids.length !== 1 ? 's' : ''}
        </span>
      </div>
    </div>
  )
}

export default function ICSProxyPage() {
  const { data, error, isLoading } = useICSFeeds()

  return (
    <main className="max-w-4xl mx-auto p-6">
      <h1 className="text-3xl font-bold mb-6">ICS Proxy</h1>

      {isLoading && <p>Loading feeds...</p>}
      {error && <p className="text-red-600">Failed to load feeds.</p>}
      {data && data.configs.length === 0 && (
        <p className="text-gray-500">No filter configs yet.</p>
      )}
      {data && data.configs.length > 0 && (
        <div className="grid gap-4">
          {data.configs.map((config) => (
            <FeedCard key={config.token} config={config} />
          ))}
        </div>
      )}
    </main>
  )
}
