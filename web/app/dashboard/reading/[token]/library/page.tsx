import { useRouteParams } from '@/hooks/navigation'
import { useSharedLibrary } from '@/hooks/useDashboardShare'
import ReadingDashboardPublicLibrary from '@/components/dashboard/ReadingDashboardPublicLibrary'
import { PageHeader } from '@/components/ui/page-header'
import { PageContainer } from '@/components/ui/page-container'

export default function ReadingDashboardPublicLibraryPage() {
  const { token } = useRouteParams<{ token: string }>()
  const library = useSharedLibrary(token).data

  return (
    <PageContainer>
      <PageHeader
        title="Shared books library"
        breadcrumb={[
          {
            label: library?.displayName ? `${library.displayName}'s reading` : 'Reading',
            href: `/dashboard/reading/${token}`
          },
          { label: 'Library' }
        ]}
      />

      <ReadingDashboardPublicLibrary token={token} />
    </PageContainer>
  )
}
