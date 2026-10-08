'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { Code, ConnectError } from '@connectrpc/connect'
import { useMovieTitle, useMoviesActions } from '@/hooks/useMovies'
import { mediaTypeLabel, releaseLabel, titleName, todayISO } from '@/lib/movies/format'
import MoviePoster from '@/components/movies/MoviePoster'
import MovieWatches from '@/components/movies/MovieWatches'
import SeasonChecklist from '@/components/movies/SeasonChecklist'
import TitleStatus from '@/components/movies/TitleStatus'
import MovieRating from '@/components/movies/MovieRating'
import WhereToWatch from '@/components/movies/WhereToWatch'
import TmdbAttribution from '@/components/movies/TmdbAttribution'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/ui/dialog'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { ErrorState, LoadingState } from '@/components/ui/states'

export default function MovieTitleClient({ id }: { id: string }) {
  const router = useRouter()
  const { data, error, isLoading } = useMovieTitle(id)
  const { remove } = useMoviesActions()
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [removing, setRemoving] = useState(false)
  const [removeFailed, setRemoveFailed] = useState(false)

  const breadcrumb = [{ label: 'Movies & Series', href: '/movies' }]

  if (isLoading && !data) return <LoadingState label="title" />
  // Other failed revalidations (e.g. offline) keep showing the loaded data.
  const removed = ConnectError.from(error).code === Code.NotFound
  if (removed || !data?.entry) {
    return (
      <PageContainer>
        <PageHeader title="Movies & Series" breadcrumb={breadcrumb} />
        <ErrorState what="title" />
      </PageContainer>
    )
  }

  const entry = data.entry
  const release = releaseLabel(entry.releaseDate, todayISO())
  const displayed = titleName(entry)
  const facts = [
    mediaTypeLabel(entry.mediaType),
    release,
    entry.runtime ? `${entry.runtime} min` : '',
    entry.seasonCount ? `${entry.seasonCount} season${entry.seasonCount === 1 ? '' : 's'}` : ''
  ].filter(Boolean)

  const confirmRemove = async () => {
    setRemoving(true)
    setRemoveFailed(false)
    try {
      await remove(entry.id)
      router.push('/movies')
    } catch {
      setRemoveFailed(true)
    } finally {
      setRemoving(false)
    }
  }

  return (
    <PageContainer>
      <PageHeader
        title={displayed}
        breadcrumb={[...breadcrumb, { label: displayed }]}
        description={entry.title !== displayed ? entry.title : undefined}
      />
      <div className="flex flex-col gap-6 sm:flex-row">
        <MoviePoster posterPath={entry.posterPath} title={displayed} size="lg" />
        <div className="min-w-0 flex-1 space-y-4">
          <p className="text-sm text-muted" suppressHydrationWarning>
            {facts.join(' · ')}
          </p>
          {entry.genres.length > 0 && (
            <div className="flex flex-wrap gap-2">
              {entry.genres.map((g) => (
                <Badge key={g} variant="secondary">
                  {g}
                </Badge>
              ))}
            </div>
          )}
          {data.overview && <p className="text-sm">{data.overview}</p>}
          <TitleStatus entry={entry} seasons={data.seasons} />
          <MovieRating entry={entry} />
          {entry.mediaType === 'series' ? (
            <SeasonChecklist entryId={entry.id} status={entry.status} seasons={data.seasons} />
          ) : (
            <MovieWatches entry={entry} />
          )}
          <WhereToWatch offers={data.offers ?? []} watchLink={data.watchLink} />
          <Button variant="destructive" onClick={() => setConfirmOpen(true)}>
            Remove from backlog
          </Button>
        </div>
      </div>
      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={`Remove ${displayed}?`}
        description="Its status and watch dates are deleted."
        confirmLabel="Remove"
        pendingLabel="Removing…"
        destructive
        pending={removing}
        onConfirm={() => void confirmRemove()}
      >
        {removeFailed && <p className="text-sm text-danger">Couldn&apos;t remove. Try again.</p>}
      </ConfirmDialog>
      <TmdbAttribution />
    </PageContainer>
  )
}
