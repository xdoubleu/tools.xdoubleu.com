'use client'

import { useState } from 'react'
import type { Season } from '@/lib/gen/movies/v1/movies_pb'
import { useMoviesActions } from '@/hooks/useMovies'
import WatchDates from '@/components/movies/WatchDates'
import { isNewSeason } from '@/lib/movies/format'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Collapsible } from '@/components/ui/collapsible'

function seasonMeta(season: Season): string {
  const parts = [
    season.episodeCount ? `${season.episodeCount} episodes` : '',
    season.aired ? season.airDate : 'Not aired yet',
    season.number === 0 ? "doesn't count toward status" : ''
  ]
  return parts.filter(Boolean).join(' · ')
}

function SeasonRow({
  entryId,
  season,
  isNew
}: {
  entryId: string
  season: Season
  isNew: boolean
}) {
  const actions = useMoviesActions()
  const [pending, setPending] = useState(false)
  const [failed, setFailed] = useState(false)
  const ticked = season.watchedAt.length > 0

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
    <Card variant="inset" className="space-y-1">
      <div className="flex flex-wrap items-center gap-2">
        <Checkbox
          label={season.name || `Season ${season.number}`}
          checked={ticked}
          disabled={pending}
          onChange={(e) =>
            void run(() => actions.setSeasonWatched(entryId, season.number, e.target.checked))
          }
        />
        {isNew && <Badge variant="warn">New</Badge>}
      </div>
      <p className="text-xs text-muted" suppressHydrationWarning>
        {seasonMeta(season)}
      </p>
      {failed && <p className="text-xs text-danger">Couldn&apos;t save. Try again.</p>}
      {ticked && (
        <Collapsible title={`Watch dates (${season.watchedAt.length})`} triggerClassName="text-sm">
          <WatchDates
            idPrefix={`season-${season.number}-watch`}
            dates={season.watchedAt}
            pending={pending}
            onAdd={(date) => void run(() => actions.addWatchDate(entryId, season.number, date))}
            onEdit={(i, date) =>
              void run(() => actions.editWatchDate(entryId, season.number, i, date))
            }
            onRemove={(i) => void run(() => actions.removeWatchDate(entryId, season.number, i))}
          />
        </Collapsible>
      )}
    </Card>
  )
}

/**
 * A series' seasons to tick off; ticking moves the series status. Seasons a
 * watched series hasn't caught up on are marked new.
 */
export default function SeasonChecklist({
  entryId,
  status,
  seasons
}: {
  entryId: string
  status: string
  seasons: Season[]
}) {
  if (seasons.length === 0) return null
  return (
    <section className="space-y-2" aria-label="Seasons">
      <h2 className="text-sm text-subtle">Seasons</h2>
      <ul className="space-y-2">
        {seasons.map((s) => (
          <li key={s.number}>
            <SeasonRow entryId={entryId} season={s} isNew={isNewSeason(status, s)} />
          </li>
        ))}
      </ul>
    </section>
  )
}
