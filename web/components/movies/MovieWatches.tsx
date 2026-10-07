'use client'

import { useState } from 'react'
import type { BacklogEntry } from '@/lib/gen/movies/v1/movies_pb'
import { useMoviesActions } from '@/hooks/useMovies'
import WatchDates from '@/components/movies/WatchDates'

/** A movie's watches, including rewatches. */
export default function MovieWatches({ entry }: { entry: BacklogEntry }) {
  const actions = useMoviesActions()
  const [pending, setPending] = useState(false)
  const [failed, setFailed] = useState(false)

  const run = async (write: () => Promise<void>) => {
    setPending(true)
    setFailed(false)
    try {
      await write()
    } catch {
      setFailed(true)
    } finally {
      setPending(false)
    }
  }

  return (
    <section className="space-y-2" aria-label="Watched">
      <h2 className="text-sm text-subtle">Watched</h2>
      <WatchDates
        idPrefix="movie-watch"
        dates={entry.watchedAt}
        pending={pending}
        onAdd={(date) => void run(() => actions.addWatchDate(entry.id, undefined, date))}
        onEdit={(i, date) => void run(() => actions.editWatchDate(entry.id, undefined, i, date))}
        onRemove={(i) => void run(() => actions.removeWatchDate(entry.id, undefined, i))}
      />
      {failed && <p className="text-sm text-danger">Couldn&apos;t save. Try again.</p>}
    </section>
  )
}
