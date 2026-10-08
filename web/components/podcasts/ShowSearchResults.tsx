'use client'

import { useState } from 'react'
import { Code, ConnectError } from '@connectrpc/connect'
import type { SearchResult } from '@/lib/gen/podcasts/v1/podcasts_pb'
import { usePodcastActions, usePodcastSearch } from '@/hooks/usePodcasts'
import ShowArtwork from '@/components/podcasts/ShowArtwork'
import { Alert } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { EmptyState, LoadingState } from '@/components/ui/states'

function errorMessage(err: unknown): string {
  if (ConnectError.from(err).code === Code.Unavailable) {
    return 'iTunes is unavailable right now. Try again later.'
  }
  return 'Search failed. Try again.'
}

function SearchResultRow({ result }: { result: SearchResult }) {
  const { add } = usePodcastActions()
  const [pending, setPending] = useState(false)
  const [failed, setFailed] = useState(false)

  const favourite = async () => {
    setPending(true)
    setFailed(false)
    try {
      await add(result.itunesId)
    } catch {
      setFailed(true)
    } finally {
      setPending(false)
    }
  }

  return (
    <Card variant="inset" className="flex flex-wrap items-center gap-3">
      <ShowArtwork url={result.artworkUrl} title={result.title} />
      <div className="min-w-0 flex-1">
        <p className="break-words font-medium">{result.title}</p>
        {result.author && <p className="break-words text-xs text-muted">{result.author}</p>}
        {failed && <p className="text-xs text-danger">Couldn&apos;t add. Try again.</p>}
      </div>
      {result.favourite ? (
        <Badge variant="secondary">Favourite</Badge>
      ) : (
        <Button disabled={pending} onClick={() => void favourite()}>
          {pending ? 'Adding…' : 'Add'}
        </Button>
      )}
    </Card>
  )
}

export default function ShowSearchResults({ query }: { query: string }) {
  const { data, error, isLoading } = usePodcastSearch(query)

  if (error) return <Alert tone="danger">{errorMessage(error)}</Alert>
  if (isLoading && !data) return <LoadingState label="results" />
  if (!data || data.results.length === 0) {
    return <EmptyState>No podcasts match &ldquo;{query.trim()}&rdquo;.</EmptyState>
  }

  return (
    <ul className="space-y-2" aria-label="Search results">
      {data.results.map((r) => (
        <li key={r.itunesId.toString()}>
          <SearchResultRow result={r} />
        </li>
      ))}
    </ul>
  )
}
