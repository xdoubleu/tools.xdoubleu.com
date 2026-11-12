import AppSections from '@/components/AppSections'
import GamesDashboard from '@/components/dashboard/GamesDashboard'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader, PageHeaderSettingsLink } from '@/components/ui/page-header'

export default function GamesDashboardPage() {
  return (
    <PageContainer
      size="wide"
      className="lg:flex lg:h-[calc(100dvh-9rem)] lg:flex-col lg:overflow-hidden"
    >
      <PageHeader
        title="Games"
        className="mb-4 lg:mb-3"
        actions={<PageHeaderSettingsLink href="/games/settings" />}
      />
      <AppSections app="games" />

      <GamesDashboard />
    </PageContainer>
  )
}
