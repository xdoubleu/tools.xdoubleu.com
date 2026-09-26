import Link from 'next/link'
import GamesDashboard from '@/components/dashboard/GamesDashboard'
import { Button } from '@/components/ui/button'
import SettingsIcon from '@/components/SettingsIcon'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { GamesService } from '@/lib/gen/games/v1/games_pb'

export default async function GamesDashboardPage() {
  const client = await createServerClient(GamesService)
  const [steam, recent] = await Promise.all([
    fetchOrNull(() => client.getSteam({})),
    fetchOrNull(() => client.getRecentlyActiveGames({}))
  ])

  return (
    <PageContainer className="lg:flex lg:h-[calc(100dvh-9rem)] lg:flex-col lg:overflow-hidden">
      <PageHeader
        title="Games"
        className="mb-4 lg:mb-3"
        actions={
          <Button asChild variant="ghost" size="sm" className="gap-2">
            <Link href="/games/settings">
              <SettingsIcon />
              Settings
            </Link>
          </Button>
        }
      />

      <GamesDashboard initialSteam={steam ?? undefined} initialRecent={recent ?? undefined} />
    </PageContainer>
  )
}
