import { useMemo, type ReactNode } from 'react'
import type { Achievement, Game } from '@/lib/gen/games/v1/games_pb'
import AchievementCard from '@/components/games/AchievementCard'
import { Badge } from '@/components/ui/badge'
import type { BreadcrumbItem } from '@/components/ui/breadcrumb'
import { PageHeader } from '@/components/ui/page-header'
import { SectionHeading } from '@/components/ui/section-heading'
import { EmptyState } from '@/components/ui/states'

// Game detail pieces shared by the owner's page and the public dashboard page.

export function SteamGameHeader({
  game,
  breadcrumb,
  titleSuffix,
  actions,
  meta
}: {
  game: Game
  breadcrumb: BreadcrumbItem[]
  titleSuffix?: ReactNode
  actions?: ReactNode
  meta?: ReactNode
}) {
  return (
    <PageHeader
      breadcrumb={breadcrumb}
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
    return <EmptyState placement="inline">No achievements for this game.</EmptyState>
  }

  const visible = showCompleted ? sorted : sorted.filter((a) => !a.achieved)
  return (
    <section>
      <SectionHeading className="mb-4">
        Achievements ({countAchieved(achievements)}/{achievements.length})
      </SectionHeading>
      {visible.length > 0 ? (
        <div className="flex flex-col gap-3">
          {visible.map((achievement) => (
            <AchievementCard key={achievement.name} achievement={achievement} />
          ))}
        </div>
      ) : (
        <EmptyState placement="inline">All achievements completed.</EmptyState>
      )}
    </section>
  )
}
