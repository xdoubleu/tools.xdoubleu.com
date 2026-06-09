'use client'

import { useCallback, useState } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
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
import { useBacklogSteam, useSteamProgress, useRecentlyActiveGames } from '@/hooks/useBacklog'
import { useSteamRefresh } from '@/lib/backlog/steamRefresh'
import type { RecentGame } from '@/lib/gen/backlog/v1/games_pb'
import SteamDistributionChart from '@/components/backlog/SteamDistributionChart'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, interactiveCardClass } from '@/components/ui/card'
import { cn } from '@/lib/cn'
import { oneYearAgo, today } from '@/lib/backlog/dates'

function StatCard({ label, value }: { label: string; value: string | number }) {
  return (
    <Card className="p-4">
      <p className="text-xs text-muted">{label}</p>
      <p className="text-2xl font-bold mt-1">{value}</p>
    </Card>
  )
}

function RecentGameCard({ game }: { game: RecentGame }) {
  const unlockLabel = game.recentUnlocks === 1 ? 'unlock' : 'unlocks'
  return (
    <Link href={`/backlog/games/${game.id}`} className={cn(interactiveCardClass, 'block p-4')}>
      <h3 className="font-semibold truncate">{game.name}</h3>
      <p className="text-sm text-muted">Completion: {game.completionRate}</p>
      <p className="text-sm text-muted">
        {game.recentUnlocks} recent {unlockLabel} &mdash; last {game.lastUnlockedAt}
      </p>
    </Link>
  )
}

export default function GamesDashboard() {
  const router = useRouter()

  const [progressStart, setProgressStart] = useState(oneYearAgo())
  const [progressEnd, setProgressEnd] = useState(today())

  const { data: steamData, error: steamError, isLoading: steamLoading } = useBacklogSteam()
  const { data: progressData, isLoading: progressLoading } = useSteamProgress(
    progressStart,
    progressEnd
  )
  const { data: recentData, isLoading: recentLoading } = useRecentlyActiveGames()

  const onSynced = useCallback(() => {
    void mutate('/backlog/games')
    void mutate('/backlog/games/recent')
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
    <section className="flex flex-col gap-4 lg:min-h-0 lg:flex-1">
      <div className="flex flex-wrap items-center justify-end gap-2">
        {lastRefresh && (
          <span className="mr-auto text-xs text-muted">
            Last: {lastRefresh.toLocaleString('en-GB')}
          </span>
        )}
        <Button variant="secondary" onClick={refresh} disabled={isRefreshing}>
          {isRefreshing ? 'Refreshing...' : 'Refresh'}
        </Button>
        <Button asChild variant="secondary">
          <Link href="/backlog/games/library">Browse full library</Link>
        </Button>
      </div>

      {steamLoading && <p>Loading dashboard...</p>}
      {steamError && <p className="text-danger">Failed to load Steam data.</p>}

      {steam && (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <StatCard label="Total backlog" value={steam.totalBacklog} />
          <StatCard label="Current rate" value={steam.currentRate} />
          <StatCard label="In progress" value={steam.inProgress.length} />
          <StatCard label="Completed" value={steam.completed.length} />
        </div>
      )}

      <div className="grid min-h-0 gap-4 lg:flex-1 lg:grid-cols-2 lg:grid-rows-2">
        <div className="flex min-h-0 flex-col lg:row-span-2">
          <h2 className="mb-3 text-lg font-semibold">Recently active</h2>
          {recentLoading && <p className="text-muted">Loading recent activity...</p>}
          {!recentLoading && recentGames.length === 0 && (
            <p className="text-muted text-sm">No recent achievement activity.</p>
          )}
          {recentGames.length > 0 && (
            <div className="grid min-h-0 gap-3 overflow-y-auto pr-1 sm:grid-cols-2 lg:grid-cols-1">
              {recentGames.map((g) => (
                <RecentGameCard key={g.id} game={g} />
              ))}
            </div>
          )}
        </div>

        <div className="flex min-h-0 flex-col">
          <div className="mb-3 flex flex-wrap items-end justify-between gap-3">
            <h2 className="text-lg font-semibold">Progress</h2>
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
          </div>
          {progressLoading && <p className="text-muted">Loading progress...</p>}
          {!progressLoading && progressChartData.length === 0 && (
            <p className="text-muted">No progress data for this range.</p>
          )}
          {progressChartData.length > 0 && (
            <div className="min-h-0 w-full flex-1">
              <ResponsiveContainer width="100%" height="100%" minHeight={200}>
                <LineChart data={progressChartData}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="label" tick={{ fontSize: 11 }} />
                  <YAxis />
                  <Tooltip />
                  <Line
                    type="monotone"
                    dataKey="value"
                    stroke="rgb(var(--color-accent))"
                    strokeWidth={2}
                    dot={false}
                  />
                </LineChart>
              </ResponsiveContainer>
            </div>
          )}
        </div>

        <div className="flex min-h-0 flex-col">
          <h2 className="mb-3 text-lg font-semibold">Distribution</h2>
          {steamLoading && <p>Loading distribution...</p>}
          {steam && (
            <SteamDistributionChart
              distribution={steam.distribution}
              onBucketClick={(bucket) => router.push(`/backlog/games/distribution/${bucket}`)}
            />
          )}
        </div>
      </div>
    </section>
  )
}
