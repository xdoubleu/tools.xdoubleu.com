import AppSections from '@/components/AppSections'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import MonitoringSettingsClient from '@/components/monitoring/MonitoringSettingsClient'

export default function MonitoringConnectionsPage() {
  return (
    <PageContainer size="wide">
      <PageHeader title="Connections" />
      <AppSections app="monitoring" />
      <MonitoringSettingsClient />
    </PageContainer>
  )
}
