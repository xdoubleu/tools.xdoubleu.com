import { useCallback, useEffect, useState } from 'react'
import { useSearchParams } from '@/hooks/navigation'
import { useAsyncAction } from '@/hooks/useAsyncAction'
import { create } from '@bufbuild/protobuf'
import { mutate as globalMutate } from 'swr'
import { useSteamGame, useRefreshSteamGame } from '@/hooks/useGames'
import { GetSteamGameResponseSchema } from '@/lib/gen/games/v1/games_pb'
import type { GetSteamGameResponse } from '@/lib/gen/games/v1/games_pb'
import { swrKeys } from '@/lib/swrKeys'
import { Alert } from '@/components/ui/alert'
import { Breadcrumb, type BreadcrumbItem } from '@/components/ui/breadcrumb'
import { Button } from '@/components/ui/button'
import { PageContainer } from '@/components/ui/page-container'
import { ErrorState, LoadingState } from '@/components/ui/states'
import GamesStatsPanel from '@/components/games/GamesStatsPanel'
import GameFavouriteButton from '@/components/games/GameFavouriteButton'
import {
  GameAchievements,
  SteamGameHeader,
  countAchieved
} from '@/components/games/SteamGameDetail'
import { formatDateTime } from '@/lib/dates'

const REFRESH_INTERVAL_MS = 60_000

export default function SteamGameClient({
  id,
  initialData
}: {
  id: string
  initialData?: GetSteamGameResponse
}) {
  const gameId = Number(id)
  const searchParams = useSearchParams()
  const { data, error, isLoading, mutate } = useSteamGame(gameId, initialData)
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
    { label: 'Games', href: '/dashboard/games' },
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

  return (
    <PageContainer>
      <Breadcrumb items={breadcrumbItems} />

      {isLoading && <LoadingState label="game" className="mt-6" />}
      {error && <ErrorState what="game" className="mt-6" />}

      {game && (
        <>
          <SteamGameHeader
            game={game}
            actions={<GameFavouriteButton game={game} className="text-2xl" />}
          />

          <div className="flex items-center gap-3 mb-6 flex-wrap">
            <Button
              variant="secondary"
              size="sm"
              className="h-auto min-h-11 flex-col gap-0.5 py-1.5"
              aria-label={isRefetching ? 'Refreshing…' : 'Refresh'}
              onClick={() => void refetch()}
              disabled={isRefetching}
            >
              <span>{isRefetching ? 'Refreshing…' : 'Refresh'}</span>
              {game.lastSyncedAt && (
                <span className="text-xs font-normal text-muted">
                  Last synced: {formatDateTime(game.lastSyncedAt)}
                </span>
              )}
            </Button>
            <Button
              variant={highPollMode ? 'default' : 'secondary'}
              size="sm"
              onClick={() => setHighPollMode((prev) => !prev)}
            >
              {highPollMode ? 'High poll: on' : 'High poll: off'}
            </Button>
            {achievedCount > 0 && (
              <Button
                variant="secondary"
                size="sm"
                onClick={() => setShowCompleted((prev) => !prev)}
              >
                {showCompleted ? 'Hide completed' : 'Show completed'}
              </Button>
            )}
            <GamesStatsPanel />
          </div>

          {refreshError && (
            <Alert tone="danger" className="mb-6">
              {refreshError}
            </Alert>
          )}

          <GameAchievements achievements={achievements} showCompleted={showCompleted} />
        </>
      )}
    </PageContainer>
  )
}
