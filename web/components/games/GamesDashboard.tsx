'use client'

import { useCallback, useState } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import Image from 'next/image'
import { mutate } from 'swr'
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer
} from 'recharts'
import { useSteam, useSteamProgress, useRecentlyActiveGames } from '@/hooks/useGames'
import { useSteamRefresh } from '@/lib/games/steamRefresh'
import type {
  GetRecentlyActiveGamesResponse,
  GetSteamResponse,
  RecentGame
} from '@/lib/gen/games/v1/games_pb'
import GamesSearch from '@/components/games/GamesSearch'
import SteamDistributionChart from '@/components/games/SteamDistributionChart'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, interactiveCardClass } from '@/components/ui/card'
import { cn } from '@/lib/cn'
import { oneYearAgo, today } from '@/lib/dates'
import { swrKeys } from '@/lib/swrKeys'

function StatCard({ label, value }: { label: string; value: string | number }) {
  return (
    <Card className="p-3">
      <p className="text-xs text-muted">{label}</p>
      <p className="text-xl font-bold mt-0.5">{value}</p>
    </Card>
  )
}

function RecentGameCard({ game }: { game: RecentGame }) {
  const unlockLabel = game.recentUnlocks === 1 ? 'unlock' : 'unlocks'
  return (
    <Link href={`/games/${game.id}`} className={cn(interactiveCardClass, 'flex gap-3 p-4')}>
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

export default function GamesDashboard({
  initialSteam,
  initialRecent
}: {
  initialSteam?: GetSteamResponse
  initialRecent?: GetRecentlyActiveGamesResponse
}) {
  const router = useRouter()

  const [progressStart, setProgressStart] = useState(oneYearAgo())
  const [progressEnd, setProgressEnd] = useState(today())
  const [view, setView] = useState<'progress' | 'distribution'>('distribution')

  const { data: steamData, error: steamError, isLoading: steamLoading } = useSteam(initialSteam)
  const { data: progressData, isLoading: progressLoading } = useSteamProgress(
    progressStart,
    progressEnd
  )
  const { data: recentData, isLoading: recentLoading } = useRecentlyActiveGames(initialRecent)

  const onSynced = useCallback(() => {
    void mutate(swrKeys.games)
    void mutate(swrKeys.gamesRecent)
  }, [])
  const { isRefreshing, lastRefresh, refresh } = useSteamRefresh(onSynced)

  const steam = steamData?.steam
  const progressSteam = progressData?.steam
  const recentGames = recentData?.games ?? []

  const progressChartData =
    progressSteam?.labels?.map((label, idx) => ({
      label,
      value: parseFloat(progressSteam.values?.[idx] ?? '0')
    })) ?? []

  return (
    <section className="flex flex-col gap-3 lg:h-full lg:min-h-0">
      <div className="flex flex-wrap items-center justify-end gap-2">
        <GamesSearch className="mr-auto flex-1 max-w-xs" />
        {lastRefresh && (
          <span className="text-xs text-muted">Last: {lastRefresh.toLocaleString('en-GB')}</span>
        )}
        <Button variant="secondary" onClick={refresh} disabled={isRefreshing}>
          {isRefreshing ? 'Refreshing…' : 'Refresh'}
        </Button>
        <Button asChild variant="secondary">
          <Link href="/games/library">Browse full library</Link>
        </Button>
      </div>

      {steamLoading && <p className="text-muted">Loading dashboard…</p>}
      {steamError && <p className="text-danger">Failed to load Steam data.</p>}

      {steam && (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <StatCard label="Total backlog" value={steam.totalBacklog} />
          <StatCard label="Current rate" value={`${steam.currentRate}%`} />
          <StatCard label="In progress" value={steam.inProgress.length} />
          <StatCard label="Completed" value={steam.completed.length} />
        </div>
      )}

      <div className="grid gap-3 lg:min-h-0 lg:flex-1 lg:grid-cols-2">
        <div className="flex min-h-0 flex-col">
          <h2 className="mb-2 text-base font-semibold">Recently active</h2>
          {recentLoading && <p className="text-muted">Loading recent activity…</p>}
          {!recentLoading && recentGames.length === 0 && (
            <p className="text-muted text-sm">No recent achievement activity.</p>
          )}
          {recentGames.length > 0 && (
            <div className="grid min-h-0 gap-3 overflow-y-auto pr-1 sm:grid-cols-2 lg:flex-1 lg:grid-cols-1">
              {recentGames.map((g) => (
                <RecentGameCard key={g.id} game={g} />
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
                  <Input
                    id="dash-from"
                    type="date"
                    value={progressStart}
                    onChange={(e) => setProgressStart(e.target.value)}
                    className="h-9 w-auto"
                  />
                </div>
                <div>
                  <label htmlFor="dash-to" className="mb-1 block text-xs text-muted">
                    To
                  </label>
                  <Input
                    id="dash-to"
                    type="date"
                    value={progressEnd}
                    onChange={(e) => setProgressEnd(e.target.value)}
                    className="h-9 w-auto"
                  />
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
                  <ResponsiveContainer width="100%" height="100%">
                    <LineChart data={progressChartData}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="label" tick={{ fontSize: 11 }} />
                      <YAxis />
                      <Tooltip
                        cursor={{ stroke: 'var(--color-border)' }}
                        contentStyle={{
                          backgroundColor: 'var(--color-surface)',
                          border: '1px solid var(--color-border)',
                          borderRadius: '0.75rem',
                          color: 'var(--color-fg)'
                        }}
                      />
                      <Line
                        type="monotone"
                        dataKey="value"
                        stroke="var(--color-accent)"
                        strokeWidth={2}
                        dot={false}
                      />
                    </LineChart>
                  </ResponsiveContainer>
                </div>
              )}
            </>
          )}

          {view === 'distribution' && (
            <>
              {steamLoading && <p className="text-muted">Loading distribution…</p>}
              {steam && (
                <div className="h-72 w-full lg:h-full lg:min-h-0 lg:flex-1">
                  <SteamDistributionChart
                    distribution={steam.distribution}
                    onBucketClick={(bucket) => router.push(`/games/distribution/${bucket}`)}
                  />
                </div>
              )}
            </>
          )}
        </div>
      </div>
    </section>
  )
}
