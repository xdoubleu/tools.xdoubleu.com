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
import type { Game, RecentGame } from '@/lib/gen/games/v1/games_pb'
import GamesStatCard from '@/components/games/GamesStatCard'
import SteamDistributionChart from '@/components/games/SteamDistributionChart'
import SteamProgressChart from '@/components/games/SteamProgressChart'
import { GameGroup } from '@/components/games/GameCards'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
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
  const [search, setSearch] = useState('')

  const { data, error, isLoading } = useSharedSteam(token, initialSteam)
  const { data: progressData, isLoading: progressLoading } = useSharedSteamProgress(
    view === 'progress' ? token : '',
    progressStart,
    progressEnd
  )
  const { data: recentData } = useSharedRecentlyActiveGames(token, initialRecent)

  const steam = data?.steam
  const recentGames = recentData?.games ?? []
  const gameHref = (game: Game | RecentGame) => `/profile/${token}/games/${game.id}`

  const progressSteam = progressData?.steam
  const progressChartData =
    progressSteam?.labels?.map((label, idx) => ({
      label,
      value: parseFloat(progressSteam.values?.[idx] ?? '0')
    })) ?? []

  if (isLoading && !steam) return <p className="text-muted">Loading games…</p>
  if (error && !steam) return <p className="text-danger">Failed to load games.</p>
  if (!steam) return null

  const filterGames = (games: Game[]) => {
    const q = search.trim().toLowerCase()
    if (!q) return games
    return games.filter((g) => g.name.toLowerCase().includes(q))
  }

  const inProgress = filterGames(steam.inProgress)
  const notStarted = filterGames(steam.notStarted)
  const completed = filterGames(steam.completed)
  const favourites = [...inProgress, ...notStarted, ...completed].filter((g) => g.favourite)

  return (
    <section className="flex flex-col gap-6">
      {data?.lastSyncedAt && (
        <p className="text-xs text-muted">Last synced: {formatDateTime(data.lastSyncedAt)}</p>
      )}

      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        <GamesStatCard label="Total backlog" value={steam.totalBacklog} />
        <GamesStatCard label="Current rate" value={`${steam.currentRate}%`} />
        <GamesStatCard label="In progress" value={steam.inProgress.length} />
        <GamesStatCard label="Completed" value={steam.completed.length} />
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        <div>
          <h2 className="mb-2 text-base font-semibold">Recently active</h2>
          {recentGames.length === 0 && (
            <p className="text-muted text-sm">No recent achievement activity.</p>
          )}
          {recentGames.length > 0 && (
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-1">
              {recentGames.map((g) => (
                <ProfileRecentGameCard key={g.id} game={g} href={gameHref(g)} />
              ))}
            </div>
          )}
        </div>

        <div>
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
            <div className="h-72 w-full">
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
                <div className="h-72 w-full">
                  <SteamProgressChart data={progressChartData} />
                </div>
              )}
            </>
          )}
        </div>
      </div>

      <div>
        <Input
          type="search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search games…"
          className="mb-4 max-w-md"
        />
        <p className="mb-4 text-muted text-sm">
          Total backlog: {steam.totalBacklog} games &mdash; Current rate: {steam.currentRate}
        </p>
        <GameGroup title="Favourites" games={favourites} hrefFor={gameHref} />
        <GameGroup title="In Progress" games={inProgress} hrefFor={gameHref} />
        <GameGroup title="Not Started" games={notStarted} hrefFor={gameHref} />
        <GameGroup title="Completed" games={completed} hrefFor={gameHref} />
        {search.trim() &&
          inProgress.length === 0 &&
          notStarted.length === 0 &&
          completed.length === 0 && (
            <p className="text-muted text-sm">No games match your search.</p>
          )}
      </div>
    </section>
  )
}
