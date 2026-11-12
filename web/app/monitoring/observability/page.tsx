import AppSections from '@/components/AppSections'
import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import ObservabilityClient from '@/components/monitoring/ObservabilityClient'

export default function MonitoringObservabilityPage() {
  return (
    <PageContainer size="wide">
      <PageHeader title="Automated actions" />
      <AppSections app="monitoring" />
      <ObservabilityClient />
    </PageContainer>
  )
}
