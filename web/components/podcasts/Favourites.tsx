import { useMemo, useState } from 'react'
import { Podcast } from 'lucide-react'
import type { Favourite } from '@/lib/gen/podcasts/v1/podcasts_pb'
import { usePodcastActions, usePodcastFavourites } from '@/hooks/usePodcasts'
import { Artwork } from '@/components/ui/artwork'
import { Button } from '@/components/ui/button'
import { CollectionGrid } from '@/components/ui/collection-grid'
import { CollectionToolbar } from '@/components/ui/collection-toolbar'
import { ConfirmDialog } from '@/components/ui/dialog'
import { FieldError } from '@/components/ui/field-error'
import { ItemCard } from '@/components/ui/item-card'
import { EmptyState, ErrorState, LoadingState } from '@/components/ui/states'

function FavouriteCard({ favourite }: { favourite: Favourite }) {
  const { remove } = usePodcastActions()
  const [confirming, setConfirming] = useState(false)
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
      setConfirming(false)
    }
  }

  return (
    <>
      <ItemCard
        title={favourite.title}
        artwork={<Artwork src={favourite.artworkUrl} title={favourite.title} size="square" />}
        meta={favourite.author || undefined}
        status={<FieldError>{failed && 'Couldn’t remove. Try again.'}</FieldError>}
        actions={
          <Button variant="secondary" size="sm" onClick={() => setConfirming(true)}>
            Remove
          </Button>
        }
      />
      <ConfirmDialog
        open={confirming}
        onOpenChange={setConfirming}
        title="Remove favourite?"
        description={`${favourite.title} leaves your favourites. You can add it again from search.`}
        confirmLabel="Remove"
        pendingLabel="Removing…"
        destructive
        pending={pending}
        onConfirm={() => void unfavourite()}
      />
    </>
  )
}

/** The favourite shows, filtered by the toolbar's search. */
export default function Favourites({ addAction }: { addAction?: React.ReactNode }) {
  const { data, error, isLoading } = usePodcastFavourites()
  const [query, setQuery] = useState('')
  const all = data?.favourites
  const shown = useMemo(() => {
    const needle = query.trim().toLowerCase()
    return (all ?? []).filter(
      (f) => f.title.toLowerCase().includes(needle) || f.author.toLowerCase().includes(needle)
    )
  }, [all, query])

  if (error && !data) return <ErrorState placement="page" what="favourites" />
  if (isLoading && !data) return <LoadingState placement="page" label="favourites" />
  if (!all || all.length === 0) {
    return (
      <EmptyState placement="page" icon={Podcast} action={addAction}>
        No favourite podcasts yet.
      </EmptyState>
    )
  }

  return (
    <div className="space-y-4">
      <CollectionToolbar noun="shows" query={query} onQueryChange={setQuery} count={shown.length} />
      {shown.length === 0 ? (
        <EmptyState placement="inline">No shows match.</EmptyState>
      ) : (
        <CollectionGrid>
          {shown.map((f) => (
            <FavouriteCard key={f.id} favourite={f} />
          ))}
        </CollectionGrid>
      )}
    </div>
  )
}
