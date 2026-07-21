'use client'

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
import GamesDashboardView from '@/components/games/GamesDashboardView'
import { Button } from '@/components/ui/button'
import { useDashboardChartState } from '@/hooks/useDashboardChartState'
import { formatDateTime } from '@/lib/dates'

export default function ProfileGamesClient({
  token,
  initialSteam,
  initialRecent
}: {
  token: string
  initialSteam?: GetSharedSteamResponse
  initialRecent?: GetSharedRecentlyActiveGamesResponse
}) {
  const chart = useDashboardChartState<'progress' | 'distribution'>('distribution')

  const { data, error, isLoading } = useSharedSteam(token, initialSteam)
  const { data: progressData, isLoading: progressLoading } = useSharedSteamProgress(
    chart.view === 'progress' ? token : '',
    chart.start,
    chart.end
  )
  const { data: recentData } = useSharedRecentlyActiveGames(token, initialRecent)

  const steam = data?.steam
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
    <GamesDashboardView
      steam={steam}
      recentGames={recentData?.games ?? []}
      gameHref={(g) => `/profile/games/${token}/${g.id}`}
      chart={chart}
      progressChartData={progressChartData}
      progressLoading={progressLoading}
      favouritesHref={`/profile/games/${token}/library`}
      actions={
        <>
          {data?.lastSyncedAt ? (
            <p className="mr-auto text-xs text-muted">
              Last synced: {formatDateTime(data.lastSyncedAt)}
            </p>
          ) : (
            <span className="mr-auto" />
          )}
          <Button asChild variant="secondary">
            <Link href={`/profile/games/${token}/library`}>Browse full library</Link>
          </Button>
        </>
      }
    />
  )
}
