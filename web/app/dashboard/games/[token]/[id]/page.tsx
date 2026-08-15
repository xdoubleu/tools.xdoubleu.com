import type { Metadata } from 'next'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { PublicGamesDashboardService } from '@/lib/gen/dashboard/v1/games_pb'
import GamesDashboardPublicGameClient from '@/components/dashboard/GamesDashboardPublicGameClient'

// Token URLs are capability links — keep them out of search indexes.
export const metadata: Metadata = {
  title: 'Shared game',
  robots: { index: false, follow: false }
}

export default async function GamesDashboardPublicGamePage({
  params
}: {
  params: Promise<{ token: string; id: string }>
}) {
  const { token, id } = await params
  const gameId = Number(id)
  const client = await createServerClient(PublicGamesDashboardService)
  const data = gameId ? await fetchOrNull(() => client.getSharedSteamGame({ token, gameId })) : null

  return <GamesDashboardPublicGameClient token={token} id={id} initialData={data ?? undefined} />
}
