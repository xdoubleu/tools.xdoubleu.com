import type { Metadata } from 'next'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { PublicLibraryService } from '@/lib/gen/books/v1/public_pb'
import ProfileBooksClient from '@/components/profile/ProfileBooksClient'
import { PageContainer } from '@/components/ui/page-container'

// Token URLs are capability links — keep them out of search indexes.
export const metadata: Metadata = {
  title: 'Shared books',
  robots: { index: false, follow: false }
}

export default async function ProfileBooksPage({ params }: { params: Promise<{ token: string }> }) {
  const { token } = await params
  const client = await createServerClient(PublicLibraryService)
  const library = await fetchOrNull(() => client.getSharedLibrary({ token }))

  return (
    <PageContainer className="p-6">
      <h1 className="mb-6 text-3xl font-bold">
        {library?.displayName ? `${library.displayName}'s books` : 'Shared books'}
      </h1>
      <SWRFallback fallback={library ? { [swrKeys.profileBooks(token)]: library } : {}}>
        <ProfileBooksClient token={token} initialData={library ?? undefined} />
      </SWRFallback>
    </PageContainer>
  )
}
