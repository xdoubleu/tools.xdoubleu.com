import { useState } from 'react'
import { useSharedSteamGame } from '@/hooks/useDashboardShare'
import {
  GameAchievements,
  SteamGameHeader,
  countAchieved
} from '@/components/games/SteamGameDetail'
import type { BreadcrumbItem } from '@/components/ui/breadcrumb'
import { Button } from '@/components/ui/button'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { ErrorState, LoadingState } from '@/components/ui/states'
import { formatDateTime } from '@/lib/dates'

// Public, read-only game detail: no refresh, no high-poll, no favourite
// toggle — visitors only see the owner's state.
export default function GamesDashboardPublicGameClient({
  token,
  id
}: {
  token: string
  id: string
}) {
  const gameId = Number(id)
  const { data, error, isLoading } = useSharedSteamGame(token, gameId)
  const game = data?.data?.game
  const achievements = data?.data?.achievements ?? []
  const [showCompleted, setShowCompleted] = useState(false)

  const breadcrumbItems: BreadcrumbItem[] = [
    { label: 'Games', href: `/dashboard/games/${token}` },
    { label: game?.name ?? 'Game' }
  ]

  if (isLoading && !game) return <LoadingState label="game" placement="page" />
  if (!game) {
    return (
      <PageContainer>
        <PageHeader title="Shared game" breadcrumb={breadcrumbItems} />
        {error && <ErrorState what="game" placement="page" />}
      </PageContainer>
    )
  }

  return (
    <PageContainer>
      <SteamGameHeader
        game={game}
        breadcrumb={breadcrumbItems}
        titleSuffix={
          game.favourite && (
            <span className="ml-3 text-star" aria-label="Favourite">
              ♥
            </span>
          )
        }
        meta={game.lastSyncedAt && <span>Last synced: {formatDateTime(game.lastSyncedAt)}</span>}
      />

      {countAchieved(achievements) > 0 && (
        <div className="mb-6">
          <Button variant="secondary" size="sm" onClick={() => setShowCompleted((prev) => !prev)}>
            {showCompleted ? 'Hide completed' : 'Show completed'}
          </Button>
        </div>
      )}

      <GameAchievements achievements={achievements} showCompleted={showCompleted} />
    </PageContainer>
  )
}
