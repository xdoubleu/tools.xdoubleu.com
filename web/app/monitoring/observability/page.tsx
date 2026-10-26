import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import ObservabilityClient from '@/components/monitoring/ObservabilityClient'

export default function MonitoringObservabilityPage() {
  return (
    <PageContainer>
      <PageHeader title="Observability" />
      <ObservabilityClient />
    </PageContainer>
  )
}
