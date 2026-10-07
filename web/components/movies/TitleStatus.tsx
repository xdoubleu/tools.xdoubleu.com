'use client'

import { useState } from 'react'
import type { BacklogEntry, Season } from '@/lib/gen/movies/v1/movies_pb'
import { useMoviesActions } from '@/hooks/useMovies'
import StatusSelect from '@/components/movies/StatusSelect'
import WatchedWhenDialog from '@/components/movies/WatchedWhenDialog'
import { Field } from '@/components/ui/field'

/** How many seasons marking the series watched would date at once. */
function countedAiredSeasons(seasons: Season[]): number {
  return seasons.filter((s) => s.number > 0 && s.aired).length
}

/**
 * The title's status control. Marking a series with several aired seasons
 * watched asks when, so an old binge doesn't date as today.
 */
export default function TitleStatus({
  entry,
  seasons
}: {
  entry: BacklogEntry
  seasons: Season[]
}) {
  const { setStatus } = useMoviesActions()
  const [pending, setPending] = useState(false)
  const [failed, setFailed] = useState(false)
  const [asking, setAsking] = useState(false)

  const change = async (status: string, unknownDate = false) => {
    setPending(true)
    setFailed(false)
    try {
      await setStatus(entry.id, status, unknownDate)
    } catch {
      setFailed(true)
    } finally {
      setPending(false)
      setAsking(false)
    }
  }

  const select = (status: string) => {
    if (status === 'watched' && countedAiredSeasons(seasons) > 1) setAsking(true)
    else void change(status)
  }

  return (
    <>
      <Field label="Status" htmlFor="movie-status" className="max-w-xs">
        <StatusSelect id="movie-status" value={entry.status} disabled={pending} onChange={select} />
      </Field>
      {failed && <p className="text-sm text-danger">Couldn&apos;t update status. Try again.</p>}
      <WatchedWhenDialog
        open={asking}
        onOpenChange={setAsking}
        title={entry.title}
        pending={pending}
        onChoose={(unknownDate) => void change('watched', unknownDate)}
      />
    </>
  )
}
