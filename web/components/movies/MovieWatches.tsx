import { useState } from 'react'
import type { BacklogEntry } from '@/lib/gen/movies/v1/movies_pb'
import { useMoviesActions } from '@/hooks/useMovies'
import WatchDates from '@/components/movies/WatchDates'
import { FieldError } from '@/components/ui/field-error'
import { SectionHeading } from '@/components/ui/section-heading'

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
      <SectionHeading level="eyebrow" as="h2">
        Watched
      </SectionHeading>
      <WatchDates
        idPrefix="movie-watch"
        dates={entry.watchedAt}
        pending={pending}
        onAdd={(date) => void run(() => actions.addWatchDate(entry.id, undefined, date))}
        onEdit={(i, date) => void run(() => actions.editWatchDate(entry.id, undefined, i, date))}
        onRemove={(i) => void run(() => actions.removeWatchDate(entry.id, undefined, i))}
      />
      <FieldError>{failed && 'Couldn’t save. Try again.'}</FieldError>
    </section>
  )
}
