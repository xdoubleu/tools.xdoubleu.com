'use client'

import { useMemo, type ReactNode } from 'react'
import type { Achievement, Game } from '@/lib/gen/games/v1/games_pb'
import AchievementCard from '@/components/games/AchievementCard'
import { Badge } from '@/components/ui/badge'
import { PageHeader } from '@/components/ui/page-header'

// Game detail pieces shared by the owner's page and the public dashboard page.

export function SteamGameHeader({
  game,
  titleSuffix,
  actions,
  meta
}: {
  game: Game
  titleSuffix?: ReactNode
  actions?: ReactNode
  meta?: ReactNode
}) {
  return (
    <PageHeader
      className="mt-4 mb-4"
      title={
        <>
          {game.name}
          {titleSuffix}
        </>
      }
      actions={actions}
      description={
        <span className="flex flex-wrap items-center gap-x-6 gap-y-1">
          <span>{Math.round(game.playtime / 60)} hrs played</span>
          <span>Completion: {game.completionRate}%</span>
          {game.isDelisted && <Badge variant="warn">Delisted</Badge>}
          {meta}
        </span>
      }
    />
  )
}

export function countAchieved(achievements: Achievement[]) {
  return achievements.filter((a) => a.achieved).length
}

/** Achievements, most-unlocked first; completed ones only when `showCompleted`. */
export function GameAchievements({
  achievements,
  showCompleted
}: {
  achievements: Achievement[]
  showCompleted: boolean
}) {
  const sorted = useMemo(
    () => [...achievements].sort((a, b) => (b.globalPercent ?? -1) - (a.globalPercent ?? -1)),
    [achievements]
  )

  if (achievements.length === 0) {
    return <p className="text-muted">No achievements for this game.</p>
  }

  const visible = showCompleted ? sorted : sorted.filter((a) => !a.achieved)
  return (
    <section>
      <h2 className="mb-4 text-xl font-semibold">
        Achievements ({countAchieved(achievements)}/{achievements.length})
      </h2>
      {visible.length > 0 ? (
        <div className="flex flex-col gap-3">
          {visible.map((achievement) => (
            <AchievementCard key={achievement.name} achievement={achievement} />
          ))}
        </div>
      ) : (
        <p className="text-muted">All achievements completed.</p>
      )}
    </section>
  )
}
