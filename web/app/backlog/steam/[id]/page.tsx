'use client'

import Link from 'next/link'
import Image from 'next/image'
import { useBacklogSteamGame } from '@/hooks/useBacklog'
import type { Achievement } from '@/lib/gen/backlog/v1/games_pb'

interface AchievementCardProps {
  achievement: Achievement
}

function AchievementCard({ achievement }: AchievementCardProps) {
  return (
    <div className="border border-border rounded p-3 flex gap-3 items-start">
      {achievement.iconUrl && (
        <Image
          src={achievement.iconUrl}
          alt={achievement.displayName}
          width={48}
          height={48}
          className="rounded shrink-0"
        />
      )}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2 flex-wrap">
          <h3 className="font-semibold text-sm">{achievement.displayName}</h3>
          {achievement.achieved ? (
            <span className="text-xs px-2 py-0.5 rounded-full bg-green-100 text-green-800">
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

export default function SteamGamePage({ params }: { params: { id: string } }) {
  const gameId = Number(params.id)
  const { data, error, isLoading } = useBacklogSteamGame(gameId)
  const game = data?.data?.game
  const achievements = data?.data?.achievements ?? []

  return (
    <main className="max-w-4xl mx-auto p-6">
      <Link href="/backlog" className="text-blue-600 hover:underline text-sm">
        &larr; Backlog
      </Link>

      {isLoading && <p className="mt-6">Loading game...</p>}
      {error && <p className="mt-6 text-red-600">Failed to load game.</p>}

      {game && (
        <>
          <div className="mt-4 mb-6">
            <h1 className="text-3xl font-bold">{game.name}</h1>
            <div className="flex gap-6 mt-2 text-muted">
              <span>{Math.round(game.playtime / 60)} hrs played</span>
              <span>Completion: {game.completionRate}</span>
              {game.isDelisted && (
                <span className="text-amber-600 text-sm font-medium">Delisted</span>
              )}
            </div>
          </div>

          {achievements.length > 0 && (
            <section>
              <h2 className="text-xl font-semibold mb-4">
                Achievements ({achievements.filter((a) => a.achieved).length}/{achievements.length})
              </h2>
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3">
                {achievements.map((achievement) => (
                  <AchievementCard key={achievement.name} achievement={achievement} />
                ))}
              </div>
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
