import SteamDistributionClient from './SteamDistributionClient'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { GamesService } from '@/lib/gen/games/v1/games_pb'

export default async function Page({ params }: { params: Promise<{ bucket: string }> }) {
  const { bucket } = await params
  const client = await createServerClient(GamesService)
  const data = await fetchOrNull(() => client.getSteamDistribution({ bucket: Number(bucket) }))

  return <SteamDistributionClient bucket={bucket} initialData={data ?? undefined} />
}
