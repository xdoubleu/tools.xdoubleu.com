import GamesSettingsClient from '@/components/games/GamesSettingsClient'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { GamesService } from '@/lib/gen/games/v1/games_pb'

export default async function BacklogGamesSettingsPage() {
  const client = await createServerClient(GamesService)
  const integrations = await fetchOrNull(() => client.getIntegrations({}))

  return <GamesSettingsClient initialData={integrations ?? undefined} />
}
