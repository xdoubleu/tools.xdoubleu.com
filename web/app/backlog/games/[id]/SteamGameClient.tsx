'use client'

import { useMemo, useState } from 'react'
import { useSearchParams } from 'next/navigation'
import Image from 'next/image'
import { useBacklogSteamGame } from '@/hooks/useBacklog'
import type { Achievement } from '@/lib/gen/backlog/v1/games_pb'
import { Breadcrumb, type BreadcrumbItem } from '@/components/ui/breadcrumb'
import { Button } from '@/components/ui/button'

interface AchievementCardProps {
  achievement: Achievement
}

function AchievementCard({ achievement }: AchievementCardProps) {
  return (
    <div className="border border-border rounded-2xl p-3 flex gap-3 items-start">
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

export default function SteamGameClient({ id }: { id: string }) {
  const gameId = Number(id)
  const searchParams = useSearchParams()
  const { data, error, isLoading } = useBacklogSteamGame(gameId)
  const game = data?.data?.game
  const achievements = data?.data?.achievements ?? []
  const [showCompleted, setShowCompleted] = useState(false)

  const bucket = searchParams.get('bucket')
  const bucketLabel = searchParams.get('label')
  const breadcrumbItems: BreadcrumbItem[] = [
    { label: 'Games', href: '/backlog/games' },
    ...(bucket
      ? [
          {
            label: bucketLabel ?? `${bucket}% range`,
            href: `/backlog/games/distribution/${bucket}`
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
    <main className="max-w-4xl mx-auto p-6">
      <Breadcrumb items={breadcrumbItems} />

      {isLoading && <p className="mt-6 text-muted">Loading game...</p>}
      {error && <p className="mt-6 text-danger">Failed to load game.</p>}

      {game && (
        <>
          <div className="mt-4 mb-6">
            <h1 className="text-3xl font-bold">{game.name}</h1>
            <div className="flex gap-6 mt-2 text-muted">
              <span>{Math.round(game.playtime / 60)} hrs played</span>
              <span>Completion: {game.completionRate}%</span>
              {game.isDelisted && (
                <span className="text-amber-600 text-sm font-medium">Delisted</span>
              )}
            </div>
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
    </main>
  )
}
