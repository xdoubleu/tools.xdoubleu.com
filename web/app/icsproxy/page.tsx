'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useICSFeeds, useDeleteConfig } from '@/hooks/useICSProxy'
import { getApiUrl } from '@/lib/env'
import type { FilterConfig } from '@/lib/gen/icsproxy/v1/proxy_pb'

const BASE_URL = getApiUrl()

function FeedCard({ config, onDeleted }: { config: FilterConfig; onDeleted: () => void }) {
  const feedUrl = `${BASE_URL}/icsproxy/${config.token}/feed.ics`
  const deleteConfig = useDeleteConfig()
  const [deleteConfirm, setDeleteConfirm] = useState(false)
  const [isDeleting, setIsDeleting] = useState(false)

  const handleDelete = async () => {
    setIsDeleting(true)
    try {
      await deleteConfig(config.token)
      onDeleted()
    } finally {
      setIsDeleting(false)
      setDeleteConfirm(false)
    }
  }

  return (
    <div className="border border-border rounded p-4">
      <p className="text-sm font-mono text-subtle break-all">{config.sourceUrl}</p>
      <div className="flex items-center gap-3 mt-3 flex-wrap">
        <a
          href={feedUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="text-sm text-accent hover:underline"
        >
          ICS Feed
        </a>
        <span className="text-border">|</span>
        <span className="text-xs text-muted">
          {config.hideEventUids.length} hidden event
          {config.hideEventUids.length !== 1 ? 's' : ''}
        </span>
        <span className="text-border">|</span>
        <Link
          href={`/icsproxy/${config.token}/edit`}
          className="text-sm text-accent hover:underline"
        >
          Edit
        </Link>
        <span className="text-border">|</span>
        {deleteConfirm ? (
          <div className="flex gap-2 items-center">
            <button
              onClick={handleDelete}
              disabled={isDeleting}
              className="rounded-xl bg-danger px-3 py-1.5 text-xs text-white hover:opacity-90 disabled:opacity-50"
            >
              {isDeleting ? 'Deleting...' : 'Confirm delete'}
            </button>
            <button
              onClick={() => setDeleteConfirm(false)}
              className="px-3 py-1 border border-border text-xs rounded hover:bg-surface"
            >
              Cancel
            </button>
          </div>
        ) : (
          <button
            onClick={() => setDeleteConfirm(true)}
            className="text-sm text-danger hover:underline"
          >
            Delete
          </button>
        )}
      </div>
    </div>
  )
}

export default function ICSProxyPage() {
  const { data, error, isLoading, mutate } = useICSFeeds()

  return (
    <main className="max-w-4xl mx-auto p-6">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-3xl font-bold">ICS Proxy</h1>
        <Link
          href="/icsproxy/new"
          className="rounded-xl bg-accent px-4 py-2 text-sm text-white hover:bg-accent-hover"
        >
          New Feed
        </Link>
      </div>

      {isLoading && <p>Loading feeds...</p>}
      {error && <p className="text-danger">Failed to load feeds.</p>}
      {data && data.configs.length === 0 && <p className="text-muted">No filter configs yet.</p>}
      {data && data.configs.length > 0 && (
        <div className="grid gap-4">
          {data.configs.map((config) => (
            <FeedCard key={config.token} config={config} onDeleted={() => mutate()} />
          ))}
        </div>
      )}
    </main>
  )
}
