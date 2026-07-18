import type { Metadata } from 'next'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { PublicGamesService } from '@/lib/gen/games/v1/public_pb'
import ProfileGamesLibrary from '@/components/profile/ProfileGamesLibrary'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { PageContainer } from '@/components/ui/page-container'

// Token URLs are capability links — keep them out of search indexes.
export const metadata: Metadata = {
  title: 'Shared library',
  robots: { index: false, follow: false }
}

export default async function ProfileGamesLibraryPage({
  params
}: {
  params: Promise<{ token: string }>
}) {
  const { token } = await params
  const client = await createServerClient(PublicGamesService)
  const steam = await fetchOrNull(() => client.getSharedSteam({ token }))

  return (
    <PageContainer className="p-6">
      <Breadcrumb
        className="mb-6"
        items={[
          {
            label: steam?.displayName ? `${steam.displayName}'s games` : 'Games',
            href: `/profile/games/${token}`
          },
          { label: 'Library' }
        ]}
      />

      <h1 className="mb-6 text-3xl font-bold">Library</h1>

      <SWRFallback fallback={steam ? { [swrKeys.profileGames(token)]: steam } : {}}>
        <ProfileGamesLibrary token={token} initialData={steam ?? undefined} />
      </SWRFallback>
    </PageContainer>
  )
}
