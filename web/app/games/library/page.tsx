import GamesLibrary from '@/components/games/GamesLibrary'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { PageContainer } from '@/components/ui/page-container'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { GamesService } from '@/lib/gen/games/v1/games_pb'

export default async function BacklogGamesLibraryPage() {
  const client = await createServerClient(GamesService)
  const steam = await fetchOrNull(() => client.getSteam({}))

  return (
    <PageContainer className="p-6">
      <Breadcrumb
        className="mb-6"
        items={[{ label: 'Games', href: '/games' }, { label: 'Library' }]}
      />

      <h1 className="text-3xl font-bold mb-6">Library</h1>

      <GamesLibrary initialSteam={steam ?? undefined} />
    </PageContainer>
  )
}
