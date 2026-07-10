'use client'

import { useState } from 'react'
import Link from 'next/link'
import { useICSFeeds, useDeleteConfig } from '@/hooks/useICSProxy'
import { getApiUrl } from '@/lib/env'
import type { FilterConfig } from '@/lib/gen/icsproxy/v1/proxy_pb'
import { Button } from '@/components/ui/button'
import { PageContainer } from '@/components/ui/page-container'

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
    <div className="border border-border rounded-2xl bg-card p-4">
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
            <Button variant="destructive" size="sm" onClick={handleDelete} disabled={isDeleting}>
              {isDeleting ? 'Deleting…' : 'Confirm delete'}
            </Button>
            <Button variant="secondary" size="sm" onClick={() => setDeleteConfirm(false)}>
              Cancel
            </Button>
          </div>
        ) : (
          <Button
            variant="link"
            size="sm"
            onClick={() => setDeleteConfirm(true)}
            className="h-auto px-0 text-sm text-danger focus-visible:ring-danger/50"
          >
            Delete
          </Button>
        )}
      </div>
    </div>
  )
}

export default function FeedsListClient() {
  const { data, error, isLoading, mutate } = useICSFeeds()

  return (
    <PageContainer className="p-6">
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-3xl font-bold">ICS Proxy</h1>
        <Button asChild>
          <Link href="/icsproxy/new">New Feed</Link>
        </Button>
      </div>

      {isLoading && <p className="text-muted">Loading feeds…</p>}
      {error && <p className="text-danger">Failed to load feeds.</p>}
      {data && data.configs.length === 0 && <p className="text-muted">No filter configs yet.</p>}
      {data && data.configs.length > 0 && (
        <div className="grid gap-4">
          {data.configs.map((config) => (
            <FeedCard key={config.token} config={config} onDeleted={() => mutate()} />
          ))}
        </div>
      )}
    </PageContainer>
  )
}
