import type { Metadata } from 'next'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { PublicLibraryService } from '@/lib/gen/reading/v1/public_pb'
import ProfileBooksLibrary from '@/components/profile/ProfileBooksLibrary'
import { Breadcrumb } from '@/components/ui/breadcrumb'
import { PageContainer } from '@/components/ui/page-container'

// Token URLs are capability links — keep them out of search indexes.
export const metadata: Metadata = {
  title: 'Shared library',
  robots: { index: false, follow: false }
}

export default async function ProfileBooksLibraryPage({
  params
}: {
  params: Promise<{ token: string }>
}) {
  const { token } = await params
  const client = await createServerClient(PublicLibraryService)
  const library = await fetchOrNull(() => client.getSharedLibrary({ token }))

  return (
    <PageContainer className="p-6">
      <Breadcrumb
        className="mb-6"
        items={[
          {
            label: library?.displayName ? `${library.displayName}'s books` : 'Books',
            href: `/profile/reading/${token}`
          },
          { label: 'Library' }
        ]}
      />

      <h1 className="mb-6 text-3xl font-bold">Library</h1>

      <SWRFallback fallback={library ? { [swrKeys.profileBooks(token)]: library } : {}}>
        <ProfileBooksLibrary token={token} initialData={library ?? undefined} />
      </SWRFallback>
    </PageContainer>
  )
}
