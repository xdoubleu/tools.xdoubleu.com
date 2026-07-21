'use client'

import type { ReactNode } from 'react'
import Link from 'next/link'
import Image from 'next/image'
import type { RecentGame, SteamResponse } from '@/lib/gen/games/v1/games_pb'
import GamesStatCard from '@/components/games/GamesStatCard'
import SteamDistributionChart from '@/components/games/SteamDistributionChart'
import SteamProgressChart from '@/components/games/SteamProgressChart'
import { Button } from '@/components/ui/button'
import { DateInput } from '@/components/ui/date-input'
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
 * Presentational games dashboard shared by the private (`GamesDashboard`) and
 * public (`ProfileGamesClient`) wrappers so their cards/charts can't drift.
 * The wrappers fetch data and pass owner actions via the `actions` slot; the
 * public one simply passes no mutating controls and omits `onBucketClick`.
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
        <GamesStatCard label="Total backlog" value={steam.totalBacklog} />
        <GamesStatCard label="Current rate" value={`${steam.currentRate}%`} />
        <GamesStatCard label="In progress" value={steam.inProgress.length} />
        <GamesStatCard label="Completed" value={steam.completed.length} />
        <GamesStatCard label="Favourites" value={favouritesCount} href={favouritesHref} />
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
            <div
              role="tablist"
              aria-label="Chart view"
              className="flex gap-1 rounded-xl border border-border bg-surface p-1"
            >
              <Button
                role="tab"
                aria-selected={view === 'distribution'}
                size="sm"
                variant={view === 'distribution' ? 'default' : 'ghost'}
                onClick={() => setView('distribution')}
              >
                Distribution
              </Button>
              <Button
                role="tab"
                aria-selected={view === 'progress'}
                size="sm"
                variant={view === 'progress' ? 'default' : 'ghost'}
                onClick={() => setView('progress')}
              >
                Progress
              </Button>
            </div>
            {view === 'progress' && (
              <div className="flex gap-3">
                <div>
                  <label htmlFor="dash-from" className="mb-1 block text-xs text-muted">
                    From
                  </label>
                  <DateInput
                    id="dash-from"
                    value={start}
                    onChange={setStart}
                    className="h-9 w-40"
                  />
                </div>
                <div>
                  <label htmlFor="dash-to" className="mb-1 block text-xs text-muted">
                    To
                  </label>
                  <DateInput id="dash-to" value={end} onChange={setEnd} className="h-9 w-40" />
                </div>
              </div>
            )}
          </div>

          {view === 'progress' && (
            <>
              {progressLoading && <p className="text-muted">Loading progress…</p>}
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
