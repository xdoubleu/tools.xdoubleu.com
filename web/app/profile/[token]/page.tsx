import type { Metadata } from 'next'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { PublicLibraryService } from '@/lib/gen/books/v1/public_pb'
import { PublicGamesService } from '@/lib/gen/games/v1/public_pb'
import ProfileLanding from '@/components/profile/ProfileLanding'
import { PageContainer } from '@/components/ui/page-container'

// Token URLs are capability links — keep them out of search indexes.
export const metadata: Metadata = {
  title: 'Shared profile',
  robots: { index: false, follow: false }
}

export default async function ProfilePage({ params }: { params: Promise<{ token: string }> }) {
  const { token } = await params
  const booksClient = await createServerClient(PublicLibraryService)
  const gamesClient = await createServerClient(PublicGamesService)
  const [library, steam] = await Promise.all([
    fetchOrNull(() => booksClient.getSharedLibrary({ token })),
    fetchOrNull(() => gamesClient.getSharedSteam({ token }))
  ])

  return (
    <PageContainer className="p-6">
      <h1 className="mb-6 text-3xl font-bold">Shared profile</h1>
      <SWRFallback
        fallback={{
          ...(library ? { [swrKeys.profileBooks(token)]: library } : {}),
          ...(steam ? { [swrKeys.profileGames(token)]: steam } : {})
        }}
      >
        <ProfileLanding
          token={token}
          initialLibrary={library ?? undefined}
          initialSteam={steam ?? undefined}
        />
      </SWRFallback>
    </PageContainer>
  )
}
