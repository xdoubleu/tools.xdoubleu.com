import Image from 'next/image'
import type { Achievement } from '@/lib/gen/games/v1/games_pb'

// Shared by the owner's game detail page and the public profile game page.
export default function AchievementCard({ achievement }: { achievement: Achievement }) {
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
