'use client'

import { useState } from 'react'
import { Code, ConnectError } from '@connectrpc/connect'
import type { SearchResult } from '@/lib/gen/movies/v1/movies_pb'
import { useMovieSearch, useMoviesActions } from '@/hooks/useMovies'
import { mediaTypeLabel, releaseLabel, statusLabel, todayISO } from '@/lib/movies/format'
import MoviePoster from '@/components/movies/MoviePoster'
import { Alert } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { EmptyState, LoadingState } from '@/components/ui/states'

type QuickStatus = 'want' | 'watched'

function errorMessage(err: unknown): string {
  if (ConnectError.from(err).code === Code.FailedPrecondition) {
    return 'TMDB is not configured yet, so search is unavailable.'
  }
  return 'Search failed. Try again.'
}

function SearchResultRow({ result, today }: { result: SearchResult; today: string }) {
  const { add } = useMoviesActions()
  const [pending, setPending] = useState<QuickStatus | null>(null)
  const [failed, setFailed] = useState(false)

  const quickAdd = async (status: QuickStatus) => {
    setPending(status)
    setFailed(false)
    try {
      await add(result.mediaType, result.tmdbId, status)
    } catch {
      setFailed(true)
    } finally {
      setPending(null)
    }
  }

  const release = releaseLabel(result.releaseDate, today)

  return (
    <Card variant="inset" className="flex flex-wrap items-center gap-3">
      <MoviePoster posterPath={result.posterPath} title={result.title} />
      <div className="min-w-0 flex-1">
        <p className="break-words font-medium">{result.title}</p>
        <p className="text-xs text-muted">
          {mediaTypeLabel(result.mediaType)}
          {release && (
            <>
              {' '}
              &middot; <span suppressHydrationWarning>{release}</span>
            </>
          )}
        </p>
        {failed && <p className="text-xs text-danger">Couldn&apos;t add. Try again.</p>}
      </div>
      {result.status ? (
        <Badge variant="secondary">{statusLabel(result.status)}</Badge>
      ) : (
        <div className="flex gap-2">
          <Button
            variant="secondary"
            disabled={pending !== null}
            onClick={() => void quickAdd('want')}
          >
            {pending === 'want' ? 'Adding…' : 'Want'}
          </Button>
          <Button disabled={pending !== null} onClick={() => void quickAdd('watched')}>
            {pending === 'watched' ? 'Adding…' : 'Watched'}
          </Button>
        </div>
      )}
    </Card>
  )
}

export default function MovieSearchResults({ query }: { query: string }) {
  const { data, error, isLoading } = useMovieSearch(query)
  const today = todayISO()

  if (error) return <Alert tone="danger">{errorMessage(error)}</Alert>
  if (isLoading && !data) return <LoadingState label="results" />
  if (!data || data.results.length === 0) {
    return <EmptyState>No movies or series match &ldquo;{query.trim()}&rdquo;.</EmptyState>
  }

  return (
    <ul className="space-y-2" aria-label="Search results">
      {data.results.map((r) => (
        // Stryker disable next-line StringLiteral: React keys aren't observable.
        <li key={`${r.mediaType}-${r.tmdbId}`}>
          <SearchResultRow result={r} today={today} />
        </li>
      ))}
    </ul>
  )
}
