import SteamGameClient from './SteamGameClient'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { GamesService } from '@/lib/gen/games/v1/games_pb'

export default async function SteamGamePage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params
  const gameId = Number(id)
  const client = await createServerClient(GamesService)
  const data = gameId ? await fetchOrNull(() => client.getSteamGame({ gameId })) : null

  return <SteamGameClient id={id} initialData={data ?? undefined} />
}
