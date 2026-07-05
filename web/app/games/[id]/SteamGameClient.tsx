'use client'

import { useCallback, useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'next/navigation'
import Image from 'next/image'
import { create } from '@bufbuild/protobuf'
import { useSteamGame, useRefreshSteamGame } from '@/hooks/useGames'
import type { Achievement } from '@/lib/gen/games/v1/games_pb'
import { GetSteamGameResponseSchema } from '@/lib/gen/games/v1/games_pb'
import type { GetSteamGameResponse } from '@/lib/gen/games/v1/games_pb'
import { Breadcrumb, type BreadcrumbItem } from '@/components/ui/breadcrumb'
import { Button } from '@/components/ui/button'
import { PageContainer } from '@/components/ui/page-container'

interface AchievementCardProps {
  achievement: Achievement
}

function AchievementCard({ achievement }: AchievementCardProps) {
  return (
    <div className="border border-border bg-card rounded-2xl p-3 flex gap-3 items-start">
      {achievement.iconUrl && (
        <Image
          src={achievement.iconUrl}
          alt={achievement.displayName}
          width={48}
          height={48}
          className="h-12 w-12 rounded-lg object-cover shrink-0"
        />
      )}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2 flex-wrap">
          <h3 className="font-semibold text-sm">{achievement.displayName}</h3>
          {achievement.achieved ? (
            <span className="rounded-full border border-success/20 bg-success/10 px-2 py-0.5 text-xs text-success">
              Achieved
            </span>
          ) : (
            <span className="text-xs px-2 py-0.5 rounded-full bg-surface text-muted">Locked</span>
          )}
          {!achievement.description && (
            <span className="text-xs px-2 py-0.5 rounded-full bg-surface text-muted">Hidden</span>
          )}
        </div>
        {achievement.description && (
          <p className="text-xs text-muted mt-0.5 line-clamp-2">{achievement.description}</p>
        )}
        {achievement.globalPercent !== undefined && (
          <p className="text-xs text-muted mt-0.5">
            {achievement.globalPercent.toFixed(1)}% of players
          </p>
        )}
      </div>
    </div>
  )
}

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
  const [isRefetching, setIsRefetching] = useState(false)
  const [highPollMode, setHighPollMode] = useState(false)

  const refetch = useCallback(() => {
    if (!gameId) return Promise.resolve()
    setIsRefetching(true)
    return refreshGame(gameId)
      .then((fresh) =>
        mutate(create(GetSteamGameResponseSchema, { data: fresh.data }), {
          revalidate: false
        })
      )
      .catch(() => {})
      .finally(() => setIsRefetching(false))
  }, [gameId, mutate, refreshGame])

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
    { label: 'Games', href: '/games' },
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

  const sortedAchievements = useMemo(
    () => [...achievements].sort((a, b) => (b.globalPercent ?? -1) - (a.globalPercent ?? -1)),
    [achievements]
  )

  const achievedCount = sortedAchievements.filter((a) => a.achieved).length
  const visibleAchievements = showCompleted
    ? sortedAchievements
    : sortedAchievements.filter((a) => !a.achieved)

  return (
    <PageContainer className="p-6">
      <Breadcrumb items={breadcrumbItems} />

      {isLoading && <p className="mt-6 text-muted">Loading game...</p>}
      {error && <p className="mt-6 text-danger">Failed to load game.</p>}

      {game && (
        <>
          <div className="mt-4 mb-4">
            <h1 className="text-3xl font-bold">{game.name}</h1>
            <div className="flex gap-6 mt-2 text-muted">
              <span>{Math.round(game.playtime / 60)} hrs played</span>
              <span>Completion: {game.completionRate}%</span>
              {game.isDelisted && (
                <span className="text-amber-600 text-sm font-medium">Delisted</span>
              )}
            </div>
          </div>

          <div className="flex items-center gap-3 mb-6 flex-wrap">
            <Button
              variant="secondary"
              size="sm"
              onClick={() => void refetch()}
              disabled={isRefetching}
            >
              {isRefetching ? 'Refreshing...' : 'Refresh'}
            </Button>
            <Button
              variant={highPollMode ? 'default' : 'secondary'}
              size="sm"
              onClick={() => setHighPollMode((prev) => !prev)}
            >
              {highPollMode ? 'High poll: on' : 'High poll: off'}
            </Button>
            {game.lastSyncedAt && (
              <span className="text-xs text-muted">
                Last synced: {new Date(game.lastSyncedAt).toLocaleString()}
              </span>
            )}
          </div>

          {achievements.length > 0 && (
            <section>
              <div className="flex items-center justify-between gap-4 mb-4 flex-wrap">
                <h2 className="text-xl font-semibold">
                  Achievements ({achievedCount}/{achievements.length})
                </h2>
                {achievedCount > 0 && (
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={() => setShowCompleted((prev) => !prev)}
                  >
                    {showCompleted ? 'Hide completed' : 'Show completed'}
                  </Button>
                )}
              </div>
              {visibleAchievements.length > 0 ? (
                <div className="flex flex-col gap-3">
                  {visibleAchievements.map((achievement) => (
                    <AchievementCard key={achievement.name} achievement={achievement} />
                  ))}
                </div>
              ) : (
                <p className="text-muted">All achievements completed.</p>
              )}
            </section>
          )}

          {achievements.length === 0 && (
            <p className="text-muted">No achievements for this game.</p>
          )}
        </>
      )}
    </PageContainer>
  )
}
