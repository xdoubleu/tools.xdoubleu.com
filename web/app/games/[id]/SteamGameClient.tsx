import { useCallback, useEffect, useState } from 'react'
import { useSearchParams } from '@/hooks/navigation'
import { useAsyncAction } from '@/hooks/useAsyncAction'
import { create } from '@bufbuild/protobuf'
import { mutate as globalMutate } from 'swr'
import { useSteamGame, useRefreshSteamGame } from '@/hooks/useGames'
import { GetSteamGameResponseSchema } from '@/lib/gen/games/v1/games_pb'
import { swrKeys } from '@/lib/swrKeys'
import { Alert } from '@/components/ui/alert'
import type { BreadcrumbItem } from '@/components/ui/breadcrumb'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Artwork } from '@/components/ui/artwork'
import { DetailHeader } from '@/components/ui/detail-header'
import { TogglePill } from '@/components/ui/toggle-pill'
import { Toolbar } from '@/components/ui/toolbar'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { ErrorState, LoadingState } from '@/components/ui/states'
import GamesStatsPanel from '@/components/games/GamesStatsPanel'
import GameFavouriteButton from '@/components/games/GameFavouriteButton'
import { GameAchievements, countAchieved } from '@/components/games/SteamGameDetail'
import { formatDateTime } from '@/lib/dates'
import { formatPercent } from '@/lib/format'

const REFRESH_INTERVAL_MS = 60_000

export default function SteamGameClient({ id }: { id: string }) {
  const gameId = Number(id)
  const searchParams = useSearchParams()
  const { data, error, isLoading, mutate } = useSteamGame(gameId)
  const refreshGame = useRefreshSteamGame()
  const game = data?.data?.game
  const achievements = data?.data?.achievements ?? []
  const [showCompleted, setShowCompleted] = useState(false)
  const { run, pending: isRefetching, error: refreshError } = useAsyncAction()
  const [highPollMode, setHighPollMode] = useState(false)

  const refetch = useCallback(() => {
    if (!gameId) return Promise.resolve()
    return run(async () => {
      const fresh = await refreshGame(gameId)
      // The refetch also updates the library-wide completion graph.
      void globalMutate(swrKeys.games)
      await mutate(create(GetSteamGameResponseSchema, { data: fresh.data }), { revalidate: false })
    }, "Couldn't refresh the game. Try again.")
  }, [gameId, mutate, refreshGame, run])

  useEffect(() => {
    if (!gameId || !highPollMode) return
    const interval = setInterval(() => {
      if (document.hidden) return
      void refetch()
    }, REFRESH_INTERVAL_MS)
    return () => clearInterval(interval)
  }, [gameId, highPollMode, refetch])

  const bucket = searchParams.get('bucket')
  const bucketLabel = searchParams.get('label')
  const breadcrumbItems: BreadcrumbItem[] = [
    { label: 'Games', href: '/games/library' },
    ...(bucket
      ? [
          {
            label: bucketLabel ?? `${bucket}% range`,
            href: `/games/distribution/${bucket}`
          }
        ]
      : []),
    { label: game?.name ?? 'Game' }
  ]

  const achievedCount = countAchieved(achievements)

  if (isLoading && !game) return <LoadingState label="game" placement="page" />
  if (!game) {
    return (
      <PageContainer>
        <PageHeader title="Game" breadcrumb={breadcrumbItems} />
        {error && <ErrorState what="game" placement="page" />}
      </PageContainer>
    )
  }

  return (
    <PageContainer>
      <DetailHeader
        breadcrumb={breadcrumbItems}
        title={game.name}
        artwork={<Artwork src={game.imageUrl} title={game.name} size="square" />}
        meta={
          <>
            {[
              `${Math.round(game.playtime / 60)} hrs played`,
              `Completion: ${formatPercent(game.completionRate)}`,
              game.lastSyncedAt && `Last synced: ${formatDateTime(game.lastSyncedAt)}`
            ]
              .filter(Boolean)
              .join(' · ')}
            {game.isDelisted && (
              <Badge variant="warn" className="ml-2">
                Delisted
              </Badge>
            )}
          </>
        }
        favourite={<GameFavouriteButton game={game} className="text-2xl" />}
        actions={
          <>
            <Button variant="secondary" onClick={() => void refetch()} disabled={isRefetching}>
              {isRefetching ? 'Refreshing…' : 'Refresh'}
            </Button>
            <GamesStatsPanel />
          </>
        }
      />

      <Toolbar aria-label="Achievements">
        <TogglePill
          label="High poll"
          active={highPollMode}
          onClick={() => setHighPollMode((prev) => !prev)}
        />
        {achievedCount > 0 && (
          <TogglePill
            label="Show completed"
            active={showCompleted}
            onClick={() => setShowCompleted((prev) => !prev)}
          />
        )}
      </Toolbar>

      {refreshError && (
        <Alert tone="danger" className="mb-6">
          {refreshError}
        </Alert>
      )}

      <GameAchievements achievements={achievements} showCompleted={showCompleted} />
    </PageContainer>
  )
}
