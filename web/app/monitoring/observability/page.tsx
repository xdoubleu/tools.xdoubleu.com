import { PageContainer } from '@/components/ui/page-container'
import { PageHeader } from '@/components/ui/page-header'
import ObservabilityClient from '@/components/monitoring/ObservabilityClient'

// No server-side prefetch: the extra round trip on the SSR path dominated
// p95; ObservabilityClient's SWR hook fetches client-side.
export default function MonitoringObservabilityPage() {
  return (
    <PageContainer>
      <PageHeader title="Observability" />
      <ObservabilityClient />
    </PageContainer>
  )
}
