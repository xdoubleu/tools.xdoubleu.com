'use client'

import { useState } from 'react'
import type { Favourite } from '@/lib/gen/podcasts/v1/podcasts_pb'
import { usePodcastActions, usePodcastFavourites } from '@/hooks/usePodcasts'
import ShowArtwork from '@/components/podcasts/ShowArtwork'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'

function FavouriteRow({ favourite }: { favourite: Favourite }) {
  const { remove } = usePodcastActions()
  const [pending, setPending] = useState(false)
  const [failed, setFailed] = useState(false)

  const unfavourite = async () => {
    setPending(true)
    setFailed(false)
    try {
      await remove(favourite.id)
    } catch {
      setFailed(true)
      setPending(false)
    }
  }

  return (
    <Card variant="inset" className="flex flex-wrap items-center gap-3">
      <ShowArtwork url={favourite.artworkUrl} title={favourite.title} />
      <div className="min-w-0 flex-1">
        <p className="break-words font-medium">{favourite.title}</p>
        {favourite.author && <p className="break-words text-xs text-muted">{favourite.author}</p>}
        {favourite.fetchError && (
          <p className="text-xs text-danger">Episodes unavailable: {favourite.fetchError}.</p>
        )}
        {failed && <p className="text-xs text-danger">Couldn&apos;t remove. Try again.</p>}
      </div>
      <Button variant="secondary" disabled={pending} onClick={() => void unfavourite()}>
        {pending ? 'Removing…' : 'Remove'}
      </Button>
    </Card>
  )
}

export default function Favourites() {
  const { data, error, isLoading } = usePodcastFavourites()

  if (error && !data) return <ErrorState what="favourites" />
  if (isLoading && !data) return <LoadingState label="favourites" />
  if (!data || data.favourites.length === 0) {
    return <EmptyState>No favourite podcasts yet. Search above to add one.</EmptyState>
  }

  return (
    <ul className="space-y-2" aria-label="Favourite podcasts">
      {data.favourites.map((f) => (
        <li key={f.id}>
          <FavouriteRow favourite={f} />
        </li>
      ))}
    </ul>
  )
}
