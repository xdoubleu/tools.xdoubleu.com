'use client'

import type { ReactNode } from 'react'
import Link from 'next/link'
import Image from 'next/image'
import type { RecentGame, SteamResponse } from '@/lib/gen/games/v1/games_pb'
import { StatTile } from '@/components/ui/stat'
import SteamDistributionChart from '@/components/games/SteamDistributionChart'
import SteamProgressChart from '@/components/games/SteamProgressChart'
import { DateRangeFields } from '@/components/ui/date-range-fields'
import { SegmentedTabs } from '@/components/ui/segmented-tabs'
import { LoadingState } from '@/components/ui/states'
import { interactiveCardClass } from '@/components/ui/card'
import { CardLinkStatus } from '@/components/ui/CardLinkStatus'
import { cn } from '@/lib/cn'
import { formatDate } from '@/lib/dates'
import type { DashboardChartState } from '@/hooks/useDashboardChartState'

function RecentGameCard({ game, href }: { game: RecentGame; href: string }) {
  return (
    <Link href={href} className={cn(interactiveCardClass, 'relative flex gap-3 p-4')}>
      <CardLinkStatus />
      {game.imageUrl && (
        <Image
          src={game.imageUrl}
          alt={game.name}
          width={32}
          height={32}
          className="h-8 w-8 rounded-lg object-cover shrink-0"
        />
      )}
      <div className="min-w-0 flex-1">
        <h3 className="font-semibold truncate">{game.name}</h3>
        <p className="text-sm text-muted">Completion: {game.completionRate}%</p>
        <p className="text-sm text-muted">
          Last played {formatDate(game.lastPlayedAt)} &mdash; {Math.round(game.playtime / 60)} hrs
        </p>
      </div>
    </Link>
  )
}

/**
 * Games dashboard view shared by the private and public wrappers so they
 * can't drift; the public one passes no mutating actions.
 */
export default function GamesDashboardView({
  steam,
  recentGames,
  gameHref,
  chart,
  progressChartData,
  progressLoading,
  onBucketClick,
  favouritesHref,
  actions
}: {
  steam: SteamResponse
  recentGames: RecentGame[]
  gameHref: (g: RecentGame) => string
  chart: DashboardChartState<'progress' | 'distribution'>
  progressChartData: { label: string; value: number }[]
  progressLoading?: boolean
  onBucketClick?: (bucket: number) => void
  favouritesHref?: string
  actions: ReactNode
}) {
  const { view, setView, start, setStart, end, setEnd } = chart
  const favouritesCount = [...steam.inProgress, ...steam.notStarted, ...steam.completed].filter(
    (g) => g.favourite
  ).length

  return (
    <section className="flex flex-col gap-3 lg:h-full lg:min-h-0">
      <div className="flex flex-wrap items-center justify-end gap-2">{actions}</div>

      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
        <StatTile label="Total backlog" value={steam.totalBacklog} />
        <StatTile label="Current rate" value={`${steam.currentRate}%`} />
        <StatTile label="In progress" value={steam.inProgress.length} />
        <StatTile label="Completed" value={steam.completed.length} />
        <StatTile label="Favourites" value={favouritesCount} href={favouritesHref} />
      </div>

      <div className="grid gap-3 lg:min-h-0 lg:flex-1 lg:grid-cols-2">
        <div className="flex min-h-0 flex-col">
          <h2 className="mb-2 text-base font-semibold">Recently active</h2>
          {recentGames.length === 0 && (
            <p className="text-muted text-sm">No recently played games.</p>
          )}
          {recentGames.length > 0 && (
            <div className="grid min-h-0 gap-3 overflow-y-auto pr-1 sm:grid-cols-2 lg:flex-1 lg:grid-cols-1">
              {recentGames.map((g) => (
                <RecentGameCard key={g.id} game={g} href={gameHref(g)} />
              ))}
            </div>
          )}
        </div>

        <div className="flex min-h-0 flex-col">
          <div className="mb-2 flex flex-wrap items-end justify-between gap-3">
            <SegmentedTabs
              aria-label="Chart view"
              value={view}
              onChange={setView}
              options={[
                { value: 'distribution', label: 'Distribution' },
                { value: 'progress', label: 'Progress' }
              ]}
            />
            {view === 'progress' && (
              <DateRangeFields
                idPrefix="dash"
                start={start}
                onStartChange={setStart}
                end={end}
                onEndChange={setEnd}
              />
            )}
          </div>

          {view === 'progress' && (
            <>
              {progressLoading && <LoadingState label="progress" />}
              {!progressLoading && progressChartData.length === 0 && (
                <p className="text-muted">No progress data for this range.</p>
              )}
              {progressChartData.length > 0 && (
                <div className="h-72 w-full lg:h-full lg:min-h-0 lg:flex-1">
                  <SteamProgressChart data={progressChartData} />
                </div>
              )}
            </>
          )}

          {view === 'distribution' && (
            <div className="h-72 w-full lg:h-full lg:min-h-0 lg:flex-1">
              <SteamDistributionChart
                distribution={steam.distribution}
                onBucketClick={onBucketClick}
              />
            </div>
          )}
        </div>
      </div>
    </section>
  )
}
