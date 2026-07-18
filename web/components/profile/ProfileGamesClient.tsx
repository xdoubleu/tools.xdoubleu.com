'use client'

import { useState } from 'react'
import Image from 'next/image'
import Link from 'next/link'
import {
  useSharedSteam,
  useSharedSteamProgress,
  useSharedRecentlyActiveGames
} from '@/hooks/useProfile'
import type {
  GetSharedSteamResponse,
  GetSharedRecentlyActiveGamesResponse
} from '@/lib/gen/games/v1/public_pb'
import type { RecentGame } from '@/lib/gen/games/v1/games_pb'
import GamesStatCard from '@/components/games/GamesStatCard'
import SteamDistributionChart from '@/components/games/SteamDistributionChart'
import SteamProgressChart from '@/components/games/SteamProgressChart'
import { Button } from '@/components/ui/button'
import { DateInput } from '@/components/ui/date-input'
import { interactiveCardClass } from '@/components/ui/card'
import { CardLinkStatus } from '@/components/ui/CardLinkStatus'
import { cn } from '@/lib/cn'
import { formatDateTime, oneYearAgo, today } from '@/lib/dates'

function ProfileRecentGameCard({ game, href }: { game: RecentGame; href: string }) {
  const unlockLabel = game.recentUnlocks === 1 ? 'unlock' : 'unlocks'
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
          {game.recentUnlocks} recent {unlockLabel} &mdash; last {game.lastUnlockedAt}
        </p>
      </div>
    </Link>
  )
}

export default function ProfileGamesClient({
  token,
  initialSteam,
  initialRecent
}: {
  token: string
  initialSteam?: GetSharedSteamResponse
  initialRecent?: GetSharedRecentlyActiveGamesResponse
}) {
  const [view, setView] = useState<'progress' | 'distribution'>('distribution')
  const [progressStart, setProgressStart] = useState(oneYearAgo())
  const [progressEnd, setProgressEnd] = useState(today())

  const { data, error, isLoading } = useSharedSteam(token, initialSteam)
  const { data: progressData, isLoading: progressLoading } = useSharedSteamProgress(
    view === 'progress' ? token : '',
    progressStart,
    progressEnd
  )
  const { data: recentData } = useSharedRecentlyActiveGames(token, initialRecent)

  const steam = data?.steam
  const recentGames = recentData?.games ?? []
  const gameHref = (game: RecentGame) => `/profile/games/${token}/${game.id}`

  const progressSteam = progressData?.steam
  const progressChartData =
    progressSteam?.labels?.map((label, idx) => ({
      label,
      value: parseFloat(progressSteam.values?.[idx] ?? '0')
    })) ?? []

  if (isLoading && !steam) return <p className="text-muted">Loading games…</p>
  if (error && !steam) return <p className="text-danger">Failed to load games.</p>
  if (!steam) return null

  return (
    <section className="flex flex-col gap-3 lg:h-full lg:min-h-0">
      <div className="flex flex-wrap items-center justify-between gap-2">
        {data?.lastSyncedAt ? (
          <p className="text-xs text-muted">Last synced: {formatDateTime(data.lastSyncedAt)}</p>
        ) : (
          <span />
        )}
        <Button asChild variant="secondary">
          <Link href={`/profile/games/${token}/library`}>Browse full library</Link>
        </Button>
      </div>

      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <GamesStatCard label="Total backlog" value={steam.totalBacklog} />
        <GamesStatCard label="Current rate" value={`${steam.currentRate}%`} />
        <GamesStatCard label="In progress" value={steam.inProgress.length} />
        <GamesStatCard label="Completed" value={steam.completed.length} />
      </div>

      <div className="grid gap-3 lg:min-h-0 lg:flex-1 lg:grid-cols-2">
        <div className="flex min-h-0 flex-col">
          <h2 className="mb-2 text-base font-semibold">Recently active</h2>
          {recentGames.length === 0 && (
            <p className="text-muted text-sm">No recent achievement activity.</p>
          )}
          {recentGames.length > 0 && (
            <div className="grid min-h-0 gap-3 overflow-y-auto pr-1 sm:grid-cols-2 lg:flex-1 lg:grid-cols-1">
              {recentGames.map((g) => (
                <ProfileRecentGameCard key={g.id} game={g} href={gameHref(g)} />
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
                  <label htmlFor="profile-games-from" className="mb-1 block text-xs text-muted">
                    From
                  </label>
                  <DateInput
                    id="profile-games-from"
                    value={progressStart}
                    onChange={setProgressStart}
                    className="h-9 w-40"
                  />
                </div>
                <div>
                  <label htmlFor="profile-games-to" className="mb-1 block text-xs text-muted">
                    To
                  </label>
                  <DateInput
                    id="profile-games-to"
                    value={progressEnd}
                    onChange={setProgressEnd}
                    className="h-9 w-40"
                  />
                </div>
              </div>
            )}
          </div>

          {view === 'distribution' && (
            <div className="h-72 w-full lg:h-full lg:min-h-0 lg:flex-1">
              <SteamDistributionChart distribution={steam.distribution} />
            </div>
          )}
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
        </div>
      </div>
    </section>
  )
}
