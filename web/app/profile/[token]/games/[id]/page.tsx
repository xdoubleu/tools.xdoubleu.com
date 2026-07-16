import type { Metadata } from 'next'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { PublicGamesService } from '@/lib/gen/games/v1/public_pb'
import ProfileGameClient from '@/components/profile/ProfileGameClient'

// Token URLs are capability links — keep them out of search indexes.
export const metadata: Metadata = {
  title: 'Shared game',
  robots: { index: false, follow: false }
}

export default async function ProfileGamePage({
  params
}: {
  params: Promise<{ token: string; id: string }>
}) {
  const { token, id } = await params
  const gameId = Number(id)
  const client = await createServerClient(PublicGamesService)
  const data = gameId ? await fetchOrNull(() => client.getSharedSteamGame({ token, gameId })) : null

  return <ProfileGameClient token={token} id={id} initialData={data ?? undefined} />
}
