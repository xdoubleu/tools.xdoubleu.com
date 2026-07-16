'use client'

import { useMemo, useState } from 'react'
import { useSharedSteamGame } from '@/hooks/useProfile'
import type { GetSharedSteamGameResponse } from '@/lib/gen/games/v1/public_pb'
import AchievementCard from '@/components/games/AchievementCard'
import { Breadcrumb, type BreadcrumbItem } from '@/components/ui/breadcrumb'
import { Button } from '@/components/ui/button'
import { PageContainer } from '@/components/ui/page-container'
import { formatDateTime } from '@/lib/dates'

// Public, read-only game detail: no refresh, no high-poll, no favourite
// toggle — visitors only see the owner's state.
export default function ProfileGameClient({
  token,
  id,
  initialData
}: {
  token: string
  id: string
  initialData?: GetSharedSteamGameResponse
}) {
  const gameId = Number(id)
  const { data, error, isLoading } = useSharedSteamGame(token, gameId, initialData)
  const game = data?.data?.game
  const achievements = data?.data?.achievements ?? []
  const [showCompleted, setShowCompleted] = useState(false)

  const breadcrumbItems: BreadcrumbItem[] = [
    { label: 'Shared profile', href: `/profile/${token}` },
    { label: 'Games', href: `/profile/${token}/games` },
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

      {isLoading && !game && <p className="mt-6 text-muted">Loading game…</p>}
      {error && !game && <p className="mt-6 text-danger">Failed to load game.</p>}

      {game && (
        <>
          <div className="mt-4 mb-4">
            <h1 className="text-3xl font-bold">
              {game.name}
              {game.favourite && (
                <span className="ml-3 text-2xl text-amber-500" aria-label="Favourite">
                  ♥
                </span>
              )}
            </h1>
            <div className="flex gap-6 mt-2 text-muted flex-wrap">
              <span>{Math.round(game.playtime / 60)} hrs played</span>
              <span>Completion: {game.completionRate}%</span>
              {game.isDelisted && (
                <span className="text-amber-600 text-sm font-medium">Delisted</span>
              )}
              {game.lastSyncedAt && (
                <span className="text-sm">Last synced: {formatDateTime(game.lastSyncedAt)}</span>
              )}
            </div>
          </div>

          {achievedCount > 0 && (
            <div className="mb-6">
              <Button
                variant="secondary"
                size="sm"
                onClick={() => setShowCompleted((prev) => !prev)}
              >
                {showCompleted ? 'Hide completed' : 'Show completed'}
              </Button>
            </div>
          )}

          {achievements.length > 0 && (
            <section>
              <div className="mb-4">
                <h2 className="text-xl font-semibold">
                  Achievements ({achievedCount}/{achievements.length})
                </h2>
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
