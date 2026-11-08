import DashboardSwitch from '@/components/dashboard/DashboardSwitch'
import ReadingDashboard from '@/components/dashboard/ReadingDashboard'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader, PageHeaderSettingsLink } from '@/components/ui/page-header'

export default function ReadingDashboardPage() {
  return (
    <PageContainer className="lg:flex lg:h-[calc(100dvh-9rem)] lg:flex-col lg:overflow-hidden">
      <PageHeader
        title="Books"
        className="mb-4 lg:mb-3"
        actions={
          <>
            <DashboardSwitch current="reading" />
            <PageHeaderSettingsLink href="/books/settings" />
          </>
        }
      />

      <ReadingDashboard />
    </PageContainer>
  )
}
