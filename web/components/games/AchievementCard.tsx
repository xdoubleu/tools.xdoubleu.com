import Image from 'next/image'
import type { Achievement } from '@/lib/gen/games/v1/games_pb'
import { Badge } from '@/components/ui/badge'
import { Card } from '@/components/ui/card'

// Shared by the owner's game detail page and the public profile game page.
export default function AchievementCard({ achievement }: { achievement: Achievement }) {
  return (
    <Card className="flex items-start gap-3 p-3">
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
          <h3 className="min-w-0 break-words font-semibold text-sm">{achievement.displayName}</h3>
          {achievement.achieved ? (
            <Badge variant="success">Achieved</Badge>
          ) : (
            <Badge variant="secondary">Locked</Badge>
          )}
          {!achievement.description && <Badge variant="secondary">Hidden</Badge>}
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
    </Card>
  )
}
