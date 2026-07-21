'use client'

import { useCallback } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import { mutate } from 'swr'
import { useSteam, useSteamProgress, useRecentlyActiveGames } from '@/hooks/useGames'
import { useSteamRefresh } from '@/lib/games/steamRefresh'
import type { GetRecentlyActiveGamesResponse, GetSteamResponse } from '@/lib/gen/games/v1/games_pb'
import GamesSearch from '@/components/games/GamesSearch'
import GamesDashboardView from '@/components/games/GamesDashboardView'
import ProfileShareButton from '@/components/profile/ProfileShareButton'
import { Button } from '@/components/ui/button'
import { useDashboardChartState } from '@/hooks/useDashboardChartState'
import { swrKeys } from '@/lib/swrKeys'

export default function GamesDashboard({
  initialSteam,
  initialRecent
}: {
  initialSteam?: GetSteamResponse
  initialRecent?: GetRecentlyActiveGamesResponse
}) {
  const router = useRouter()
  const chart = useDashboardChartState<'progress' | 'distribution'>('distribution')

  const { data: steamData, error: steamError, isLoading: steamLoading } = useSteam(initialSteam)
  const { data: progressData, isLoading: progressLoading } = useSteamProgress(
    chart.start,
    chart.end
  )
  const { data: recentData } = useRecentlyActiveGames(initialRecent)

  const onSynced = useCallback(() => {
    void mutate(swrKeys.games)
    void mutate(swrKeys.gamesRecent)
  }, [])
  const { isRefreshing, lastRefresh, refresh } = useSteamRefresh(onSynced)

  const steam = steamData?.steam
  const progressSteam = progressData?.steam
  const progressChartData =
    progressSteam?.labels?.map((label, idx) => ({
      label,
      value: parseFloat(progressSteam.values?.[idx] ?? '0')
    })) ?? []

  if (steamLoading && !steam) return <p className="text-muted">Loading dashboard…</p>
  if (steamError && !steam) return <p className="text-danger">Failed to load Steam data.</p>
  if (!steam) return null

  return (
    <GamesDashboardView
      steam={steam}
      recentGames={recentData?.games ?? []}
      gameHref={(g) => `/games/${g.id}`}
      chart={chart}
      progressChartData={progressChartData}
      progressLoading={progressLoading}
      onBucketClick={(bucket) => router.push(`/games/distribution/${bucket}`)}
      favouritesHref="/games/library"
      actions={
        <>
          <GamesSearch className="mr-auto flex-1 max-w-xs" />
          {lastRefresh && (
            <span className="text-xs text-muted">Last: {lastRefresh.toLocaleString('en-GB')}</span>
          )}
          <Button variant="secondary" onClick={refresh} disabled={isRefreshing}>
            {isRefreshing ? 'Refreshing…' : 'Refresh'}
          </Button>
          <ProfileShareButton app="games" />
          <Button asChild variant="secondary">
            <Link href="/games/library">Browse full library</Link>
          </Button>
        </>
      }
    />
  )
}
