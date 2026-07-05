import Link from 'next/link'
import GamesDashboard from '@/components/games/GamesDashboard'
import { Button } from '@/components/ui/button'
import SettingsIcon from '@/components/SettingsIcon'
import { PageContainer } from '@/components/ui/page-container'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { GamesService } from '@/lib/gen/games/v1/games_pb'

export default async function BacklogGamesPage() {
  const client = await createServerClient(GamesService)
  const [steam, recent] = await Promise.all([
    fetchOrNull(() => client.getSteam({})),
    fetchOrNull(() => client.getRecentlyActiveGames({}))
  ])

  return (
    <PageContainer className="p-6 lg:flex lg:h-[calc(100dvh-9rem)] lg:flex-col lg:overflow-hidden lg:p-4">
      <div className="mb-4 flex items-center justify-between gap-4 lg:mb-3">
        <h1 className="text-3xl font-bold lg:text-2xl">Games</h1>
        <Button asChild variant="ghost" size="sm" className="gap-2">
          <Link href="/games/settings">
            <SettingsIcon />
            Settings
          </Link>
        </Button>
      </div>

      <GamesDashboard initialSteam={steam ?? undefined} initialRecent={recent ?? undefined} />
    </PageContainer>
  )
}
