import type { Metadata } from 'next'
import Link from 'next/link'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { PublicGamesService } from '@/lib/gen/games/v1/public_pb'
import ProfileGamesClient from '@/components/profile/ProfileGamesClient'
import { Button } from '@/components/ui/button'
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
    <PageContainer className="p-6">
      <div className="mb-6 flex items-center justify-between gap-4">
        <h1 className="text-3xl font-bold">Games</h1>
        <Button asChild variant="secondary" size="sm">
          <Link href={`/profile/${token}`}>Back to profile</Link>
        </Button>
      </div>
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
