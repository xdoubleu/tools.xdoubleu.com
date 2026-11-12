import AppSections from '@/components/AppSections'
import ReadingDashboard from '@/components/dashboard/ReadingDashboard'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader, PageHeaderSettingsLink } from '@/components/ui/page-header'

export default function ReadingDashboardPage() {
  return (
    <PageContainer
      size="wide"
      className="lg:flex lg:h-[calc(100dvh-9rem)] lg:flex-col lg:overflow-hidden"
    >
      <PageHeader
        title="Books"
        className="mb-4 lg:mb-3"
        actions={<PageHeaderSettingsLink href="/books/settings" />}
      />
      <AppSections app="books" />

      <ReadingDashboard />
    </PageContainer>
  )
}
