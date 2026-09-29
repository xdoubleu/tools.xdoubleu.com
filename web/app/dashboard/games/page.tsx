import GamesDashboard from '@/components/dashboard/GamesDashboard'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader, PageHeaderSettingsLink } from '@/components/ui/page-header'
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
        actions={<PageHeaderSettingsLink href="/games/settings" />}
      />

      <GamesDashboard initialSteam={steam ?? undefined} initialRecent={recent ?? undefined} />
    </PageContainer>
  )
}
