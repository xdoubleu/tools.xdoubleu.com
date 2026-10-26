import { Link } from '@/components/ui/link'
import {
  useSharedSteam,
  useSharedSteamProgress,
  useSharedRecentlyActiveGames
} from '@/hooks/useDashboardShare'
import GamesDashboardView from '@/components/dashboard/GamesDashboardView'
import { Button } from '@/components/ui/button'
import { ErrorState, LoadingState } from '@/components/ui/states'
import { useDashboardChartState } from '@/hooks/useDashboardChartState'
import { formatDateTime } from '@/lib/dates'

export default function GamesDashboardPublicClient({ token }: { token: string }) {
  const chart = useDashboardChartState<'progress' | 'distribution'>('distribution')

  const { data, error, isLoading } = useSharedSteam(token)
  const { data: progressData, isLoading: progressLoading } = useSharedSteamProgress(
    chart.view === 'progress' ? token : '',
    chart.start,
    chart.end
  )
  const { data: recentData } = useSharedRecentlyActiveGames(token)

  const steam = data?.steam
  const progressSteam = progressData?.steam
  const progressChartData =
    progressSteam?.labels?.map((label, idx) => ({
      label,
      value: parseFloat(progressSteam.values?.[idx] ?? '0')
    })) ?? []

  if (isLoading && !steam) return <LoadingState label="games" />
  if (error && !steam) return <ErrorState what="games" />
  if (!steam) return null

  return (
    <GamesDashboardView
      steam={steam}
      recentGames={recentData?.games ?? []}
      gameHref={(g) => `/dashboard/games/${token}/${g.id}`}
      chart={chart}
      progressChartData={progressChartData}
      progressLoading={progressLoading}
      favouritesHref={`/dashboard/games/${token}/library`}
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
            <Link href={`/dashboard/games/${token}/library`}>Browse full library</Link>
          </Button>
        </>
      }
    />
  )
}
