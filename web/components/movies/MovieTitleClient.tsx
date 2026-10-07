'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { useMovieTitle, useMoviesActions } from '@/hooks/useMovies'
import { mediaTypeLabel, releaseLabel, todayISO, watchDateLabel } from '@/lib/movies/format'
import MoviePoster from '@/components/movies/MoviePoster'
import StatusSelect from '@/components/movies/StatusSelect'
import TmdbAttribution from '@/components/movies/TmdbAttribution'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/components/ui/dialog'
import { Field } from '@/components/ui/field'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { ErrorState, LoadingState } from '@/components/ui/states'

export default function MovieTitleClient({ id }: { id: string }) {
  const router = useRouter()
  const { data, error, isLoading } = useMovieTitle(id)
  const { setStatus, remove } = useMoviesActions()
  const [pending, setPending] = useState(false)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [removing, setRemoving] = useState(false)
  const [failed, setFailed] = useState<'status' | 'remove' | null>(null)

  const breadcrumb = [{ label: 'Movies & Series', href: '/movies' }]

  if (isLoading && !data) return <LoadingState label="title" />
  if (error || !data?.entry) {
    return (
      <PageContainer>
        <PageHeader title="Movies & Series" breadcrumb={breadcrumb} />
        <ErrorState what="title" />
      </PageContainer>
    )
  }

  const entry = data.entry
  const release = releaseLabel(entry.releaseDate, todayISO())
  const facts = [
    mediaTypeLabel(entry.mediaType),
    release,
    entry.runtime ? `${entry.runtime} min` : '',
    entry.seasonCount ? `${entry.seasonCount} season${entry.seasonCount === 1 ? '' : 's'}` : ''
  ].filter(Boolean)

  const change = async (status: string) => {
    setPending(true)
    setFailed(null)
    try {
      await setStatus(entry.id, status)
    } catch {
      setFailed('status')
    } finally {
      setPending(false)
    }
  }

  const confirmRemove = async () => {
    setRemoving(true)
    setFailed(null)
    try {
      await remove(entry.id)
      router.push('/movies')
    } catch {
      setFailed('remove')
    } finally {
      setRemoving(false)
    }
  }

  return (
    <PageContainer>
      <PageHeader
        title={entry.title}
        breadcrumb={[...breadcrumb, { label: entry.title }]}
        description={
          entry.originalTitle && entry.originalTitle !== entry.title
            ? entry.originalTitle
            : undefined
        }
      />
      <div className="flex flex-col gap-6 sm:flex-row">
        <MoviePoster posterPath={entry.posterPath} title={entry.title} size="lg" />
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
          <Field label="Status" htmlFor="movie-status" className="max-w-xs">
            <StatusSelect
              id="movie-status"
              value={entry.status}
              disabled={pending}
              onChange={(s) => void change(s)}
            />
          </Field>
          {failed === 'status' && (
            <p className="text-sm text-danger">Couldn&apos;t update status. Try again.</p>
          )}
          {entry.watchedAt.length > 0 && (
            <div>
              <p className="text-sm text-subtle">Watched</p>
              <ul className="text-sm">
                {entry.watchedAt.map((w, i) => (
                  <li key={`${w}-${i}`} suppressHydrationWarning>
                    {watchDateLabel(w)}
                  </li>
                ))}
              </ul>
            </div>
          )}
          <Button variant="destructive" onClick={() => setConfirmOpen(true)}>
            Remove from backlog
          </Button>
        </div>
      </div>
      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={`Remove ${entry.title}?`}
        description="Its status and watch dates are deleted."
        confirmLabel="Remove"
        pendingLabel="Removing…"
        destructive
        pending={removing}
        onConfirm={() => void confirmRemove()}
      >
        {failed === 'remove' && (
          <p className="text-sm text-danger">Couldn&apos;t remove. Try again.</p>
        )}
      </ConfirmDialog>
      <TmdbAttribution />
    </PageContainer>
  )
}
