import { useCallback } from 'react'
import { useRouter } from '@/hooks/navigation'
import { Link } from '@/components/ui/link'
import { mutate } from 'swr'
import { useSteam, useSteamProgress, useRecentlyActiveGames } from '@/hooks/useGames'
import { useSteamRefresh } from '@/lib/games/steamRefresh'
import GamesSearch from '@/components/games/GamesSearch'
import GamesDashboardView from '@/components/dashboard/GamesDashboardView'
import DashboardShareButton from '@/components/dashboard/DashboardShareButton'
import { Button } from '@/components/ui/button'
import { ErrorState, LoadingState } from '@/components/ui/states'
import { useDashboardChartState } from '@/hooks/useDashboardChartState'
import { swrKeys } from '@/lib/swrKeys'

export default function GamesDashboard() {
  const router = useRouter()
  const chart = useDashboardChartState<'progress' | 'distribution'>('distribution')

  const { data: steamData, error: steamError, isLoading: steamLoading } = useSteam()
  const { data: progressData, isLoading: progressLoading } = useSteamProgress(
    chart.start,
    chart.end
  )
  const { data: recentData } = useRecentlyActiveGames()

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

  if (steamLoading && !steam) return <LoadingState label="dashboard" />
  if (steamError && !steam) return <ErrorState what="Steam data" />
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
          <DashboardShareButton kind="games" />
          <Button asChild variant="secondary">
            <Link href="/games/library">Browse full library</Link>
          </Button>
        </>
      }
    />
  )
}
