import type { Metadata } from 'next'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { PublicGamesDashboardService } from '@/lib/gen/dashboard/v1/games_pb'
import GamesDashboardPublicClient from '@/components/dashboard/GamesDashboardPublicClient'
import { PageContainer } from '@/components/ui/page-container'

// Token URLs are capability links — keep them out of search indexes.
export const metadata: Metadata = {
  title: 'Shared games',
  robots: { index: false, follow: false }
}

export default async function GamesDashboardPublicPage({
  params
}: {
  params: Promise<{ token: string }>
}) {
  const { token } = await params
  const client = await createServerClient(PublicGamesDashboardService)
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
          ...(steam ? { [swrKeys.dashboardGames(token)]: steam } : {}),
          ...(recent ? { [swrKeys.dashboardRecentGames(token)]: recent } : {})
        }}
      >
        <GamesDashboardPublicClient
          token={token}
          initialSteam={steam ?? undefined}
          initialRecent={recent ?? undefined}
        />
      </SWRFallback>
    </PageContainer>
  )
}
