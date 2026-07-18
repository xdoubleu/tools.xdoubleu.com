import type { Metadata } from 'next'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { PublicGamesService } from '@/lib/gen/games/v1/public_pb'
import ProfileGamesClient from '@/components/profile/ProfileGamesClient'
import { PageContainer } from '@/components/ui/page-container'

// Token URLs are capability links — keep them out of search indexes.
export const metadata: Metadata = {
  title: 'Shared games',
  robots: { index: false, follow: false }
}

export default async function ProfileGamesPage({ params }: { params: Promise<{ token: string }> }) {
  const { token } = await params
  const client = await createServerClient(PublicGamesService)
  const [steam, recent] = await Promise.all([
    fetchOrNull(() => client.getSharedSteam({ token })),
    fetchOrNull(() => client.getSharedRecentlyActiveGames({ token }))
  ])

  return (
    <PageContainer className="p-6 lg:flex lg:h-[calc(100dvh-9rem)] lg:flex-col lg:overflow-hidden lg:p-4">
      <h1 className="mb-6 text-3xl font-bold lg:mb-3 lg:text-2xl">
        {steam?.displayName ? `${steam.displayName}'s games` : 'Shared games'}
      </h1>
      <SWRFallback
        fallback={{
          ...(steam ? { [swrKeys.profileGames(token)]: steam } : {}),
          ...(recent ? { [swrKeys.profileRecentGames(token)]: recent } : {})
        }}
      >
        <ProfileGamesClient
          token={token}
          initialSteam={steam ?? undefined}
          initialRecent={recent ?? undefined}
        />
      </SWRFallback>
    </PageContainer>
  )
}
