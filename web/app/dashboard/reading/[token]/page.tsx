import { useRouteParams } from '@/hooks/navigation'
import { useSharedLibrary } from '@/hooks/useDashboardShare'
import ReadingDashboardPublicClient from '@/components/dashboard/ReadingDashboardPublicClient'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'

export default function ReadingDashboardPublicPage() {
  const { token } = useRouteParams<{ token: string }>()
  const library = useSharedLibrary(token).data

  return (
    <PageContainer
      size="wide"
      className="lg:flex lg:h-[calc(100dvh-9rem)] lg:flex-col lg:overflow-hidden"
    >
      <PageHeader
        title={library?.displayName ? `${library.displayName}'s reading` : 'Shared reading'}
        className="lg:mb-3"
      />
      <ReadingDashboardPublicClient token={token} />
    </PageContainer>
  )
}
