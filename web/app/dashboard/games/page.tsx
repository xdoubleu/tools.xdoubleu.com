import DashboardSwitch from '@/components/dashboard/DashboardSwitch'
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
        actions={
          <>
            <DashboardSwitch current="games" />
            <PageHeaderSettingsLink href="/games/settings" />
          </>
        }
      />

      <GamesDashboard />
    </PageContainer>
  )
}
