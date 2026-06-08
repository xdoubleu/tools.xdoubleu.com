'use client'

import { useState } from 'react'
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
import { useBacklogSteam, useSteamProgress, useRefreshSteam } from '@/hooks/useBacklog'
import type { Game } from '@/lib/gen/backlog/v1/games_pb'
import SteamDistributionChart from '@/components/backlog/SteamDistributionChart'
import SectionTabBar from '@/components/backlog/SectionTabBar'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { interactiveCardClass } from '@/components/ui/card'
import { cn } from '@/lib/cn'
import { oneYearAgo, today } from '@/lib/backlog/dates'

type GamesTab = 'backlog' | 'progress' | 'distribution'

function GameCard({ game }: { game: Game }) {
  return (
    <Link href={`/backlog/steam/${game.id}`} className={cn(interactiveCardClass, 'block p-4')}>
      <h3 className="font-semibold">{game.name}</h3>
      <p className="text-sm text-muted">Playtime: {Math.round(game.playtime / 60)} hrs</p>
      <p className="text-sm text-muted">Completion: {game.completionRate}</p>
    </Link>
  )
}

export default function GamesSection() {
  const router = useRouter()
  const [gamesTab, setGamesTab] = useState<GamesTab>('backlog')
  const [search, setSearch] = useState('')
  const [refreshing, setRefreshing] = useState(false)

  const [progressStart, setProgressStart] = useState(oneYearAgo())
  const [progressEnd, setProgressEnd] = useState(today())

  const { data: steamData, error: steamError, isLoading: steamLoading } = useBacklogSteam()
  const { data: progressData, isLoading: progressLoading } = useSteamProgress(
    progressStart,
    progressEnd
  )
  const refreshSteam = useRefreshSteam()

  const steam = steamData?.steam
  const progressSteam = progressData?.steam

  const progressChartData =
    progressSteam?.labels?.map((label, idx) => ({
      label,
      value: parseFloat(progressSteam.values?.[idx] ?? '0')
    })) ?? []

  const filterGames = (games: Game[]) => {
    const q = search.trim().toLowerCase()
    if (!q) return games
    return games.filter((g) => g.name.toLowerCase().includes(q))
  }

  const handleRefresh = async () => {
    setRefreshing(true)
    try {
      await refreshSteam()
      await mutate('/backlog/steam')
    } finally {
      setRefreshing(false)
    }
  }

  const inProgress = steam ? filterGames(steam.inProgress) : []
  const notStarted = steam ? filterGames(steam.notStarted) : []
  const completed = steam ? filterGames(steam.completed) : []

  return (
    <section>
      <div className="mb-4 flex flex-col gap-2 sm:flex-row sm:items-center">
        <Input
          type="search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search games..."
          className="flex-1"
        />
        <Button variant="secondary" onClick={handleRefresh} disabled={refreshing}>
          {refreshing ? 'Refreshing...' : 'Refresh'}
        </Button>
      </div>

      <SectionTabBar
        tabs={[
          { id: 'backlog' as GamesTab, label: 'Backlog' },
          { id: 'progress' as GamesTab, label: 'Progress' },
          { id: 'distribution' as GamesTab, label: 'Distribution' }
        ]}
        active={gamesTab}
        onChange={setGamesTab}
      />

      {gamesTab === 'backlog' && (
        <>
          {steamLoading && <p>Loading Steam library...</p>}
          {steamError && <p className="text-danger">Failed to load Steam data.</p>}
          {steam && (
            <>
              <p className="mb-4 text-muted text-sm">
                Total backlog: {steam.totalBacklog} games &mdash; Current rate: {steam.currentRate}
              </p>
              {inProgress.length > 0 && (
                <div className="mb-6">
                  <h2 className="text-lg font-semibold mb-3">In Progress ({inProgress.length})</h2>
                  <div className="grid sm:grid-cols-2 gap-3">
                    {inProgress.map((g) => (
                      <GameCard key={g.id} game={g} />
                    ))}
                  </div>
                </div>
              )}
              {notStarted.length > 0 && (
                <div className="mb-6">
                  <h2 className="text-lg font-semibold mb-3">Not Started ({notStarted.length})</h2>
                  <div className="grid sm:grid-cols-2 gap-3">
                    {notStarted.map((g) => (
                      <GameCard key={g.id} game={g} />
                    ))}
                  </div>
                </div>
              )}
              {completed.length > 0 && (
                <div className="mb-6">
                  <h2 className="text-lg font-semibold mb-3">Completed ({completed.length})</h2>
                  <div className="grid sm:grid-cols-2 gap-3">
                    {completed.map((g) => (
                      <GameCard key={g.id} game={g} />
                    ))}
                  </div>
                </div>
              )}
              {search.trim() &&
                inProgress.length === 0 &&
                notStarted.length === 0 &&
                completed.length === 0 && (
                  <p className="text-muted text-sm">No games match your search.</p>
                )}
            </>
          )}
        </>
      )}

      {gamesTab === 'progress' && (
        <div>
          <div className="flex gap-4 mb-4 flex-wrap">
            <div>
              <label htmlFor="steam-from" className="block text-xs text-muted mb-1">
                From
              </label>
              <Input
                id="steam-from"
                type="date"
                value={progressStart}
                onChange={(e) => setProgressStart(e.target.value)}
                className="h-9 w-auto"
              />
            </div>
            <div>
              <label htmlFor="steam-to" className="block text-xs text-muted mb-1">
                To
              </label>
              <Input
                id="steam-to"
                type="date"
                value={progressEnd}
                onChange={(e) => setProgressEnd(e.target.value)}
                className="h-9 w-auto"
              />
            </div>
          </div>
          {progressLoading && <p className="text-muted">Loading progress...</p>}
          {!progressLoading && progressChartData.length === 0 && (
            <p className="text-muted">No progress data for this range.</p>
          )}
          {progressChartData.length > 0 && (
            <div className="w-full h-64">
              <ResponsiveContainer width="100%" height="100%">
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
      )}

      {gamesTab === 'distribution' && (
        <>
          {steamLoading && <p>Loading distribution...</p>}
          {steam && (
            <SteamDistributionChart
              distribution={steam.distribution}
              onBucketClick={(bucket) => router.push(`/backlog/steam/distribution/${bucket}`)}
            />
          )}
          <p className="text-xs text-muted mt-2">Click a bar to see games in that range.</p>
        </>
      )}
    </section>
  )
}
