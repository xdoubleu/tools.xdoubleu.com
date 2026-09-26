import GamesLibrary from '@/components/games/GamesLibrary'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { GamesService } from '@/lib/gen/games/v1/games_pb'

export default async function BacklogGamesLibraryPage() {
  const client = await createServerClient(GamesService)
  const steam = await fetchOrNull(() => client.getSteam({}))

  return (
    <PageContainer>
      <PageHeader
        title="Library"
        breadcrumb={[{ label: 'Games', href: '/dashboard/games' }, { label: 'Library' }]}
      />

      <GamesLibrary initialSteam={steam ?? undefined} />
    </PageContainer>
  )
}
