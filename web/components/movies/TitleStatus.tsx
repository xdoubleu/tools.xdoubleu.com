import { useState } from 'react'
import type { BacklogEntry, Season } from '@/lib/gen/movies/v1/movies_pb'
import { useMoviesActions } from '@/hooks/useMovies'
import { STATUSES, STATUS_LABELS, titleName } from '@/lib/movies/format'
import WatchedWhenDialog from '@/components/movies/WatchedWhenDialog'
import { SegmentedTabs } from '@/components/ui/segmented-tabs'
import { FieldError } from '@/components/ui/field-error'

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
      <SegmentedTabs
        aria-label="Status"
        value={STATUSES.find((s) => s === entry.status) ?? null}
        options={STATUSES.map((value) => ({ value, label: STATUS_LABELS[value] }))}
        disabled={pending}
        onChange={select}
      />
      <FieldError>{failed && 'Couldn’t update status. Try again.'}</FieldError>
      <WatchedWhenDialog
        open={asking}
        onOpenChange={setAsking}
        title={titleName(entry)}
        pending={pending}
        onChoose={(unknownDate) => void change('watched', unknownDate)}
      />
    </>
  )
}
