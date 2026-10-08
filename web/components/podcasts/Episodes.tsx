'use client'

import { useMemo } from 'react'
import type { Episode } from '@/lib/gen/podcasts/v1/podcasts_pb'
import { useFetchPodcastEpisodesPage, usePodcastEpisodes } from '@/hooks/usePodcasts'
import { usePaginatedList } from '@/hooks/usePaginatedList'
import { formatDate } from '@/lib/dates'
import { durationLabel } from '@/lib/podcasts/format'
import ShowArtwork from '@/components/podcasts/ShowArtwork'
import { Card } from '@/components/ui/card'
import { LoadMoreButton } from '@/components/ui/LoadMoreButton'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'

const linkClass = 'inline-flex min-h-11 items-center underline'

function EpisodeRow({ episode }: { episode: Episode }) {
  const published = formatDate(episode.publishedAt)
  const duration = durationLabel(episode.durationSeconds)
  const meta = [episode.showTitle, published, duration].filter(Boolean)

  return (
    <Card variant="inset" className="flex items-start gap-3">
      <ShowArtwork url={episode.artworkUrl} title={episode.showTitle} />
      <div className="min-w-0 flex-1">
        <p className="break-words font-medium">{episode.title}</p>
        <p className="break-words text-xs text-muted" suppressHydrationWarning>
          {meta.join(' · ')}
        </p>
        {episode.summary && (
          <p className="mt-1 line-clamp-3 break-words text-sm text-subtle">{episode.summary}</p>
        )}
        <div className="flex flex-wrap gap-x-4 text-sm">
          {episode.link && (
            <a href={episode.link} target="_blank" rel="noopener noreferrer" className={linkClass}>
              Episode page
            </a>
          )}
          {episode.appleUrl && (
            <a
              href={episode.appleUrl}
              target="_blank"
              rel="noopener noreferrer"
              className={linkClass}
            >
              Open in Apple Podcasts
            </a>
          )}
        </div>
      </div>
    </Card>
  )
}

/** The newest episodes of every favourite, newest first. */
export default function Episodes() {
  const { data, error, isLoading } = usePodcastEpisodes()
  const fetchPage = useFetchPodcastEpisodesPage()
  const initialPage = useMemo(
    // Stryker disable next-line BooleanLiteral: without data there are no episodes to load more of.
    () => ({ items: data?.episodes ?? [], hasMore: data?.hasMore ?? false }),
    [data]
  )
  const {
    items: episodes,
    hasMore,
    loading: loadingMore,
    loadMore
  } = usePaginatedList(initialPage, fetchPage, (a, b) => a.id === b.id)

  if (error && !data) return <ErrorState what="episodes" />
  if (isLoading && !data) return <LoadingState label="episodes" />
  if (episodes.length === 0) {
    return (
      <EmptyState>
        No episodes yet. Favourite a show to see its new episodes here; they can take a moment to
        arrive.
      </EmptyState>
    )
  }

  return (
    <>
      <ul className="space-y-2" aria-label="Episodes">
        {episodes.map((e) => (
          <li key={e.id}>
            <EpisodeRow episode={e} />
          </li>
        ))}
      </ul>
      {hasMore && <LoadMoreButton onClick={loadMore} loading={loadingMore} />}
    </>
  )
}
