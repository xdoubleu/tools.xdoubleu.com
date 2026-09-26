import { PageContainer } from '@/components/ui/page-container'
import ObservabilityClient from '@/components/monitoring/ObservabilityClient'

// No server-side prefetch: the extra round trip on the SSR path dominated
// p95; ObservabilityClient's SWR hook fetches client-side.
export default function MonitoringObservabilityPage() {
  return (
    <PageContainer>
      <h1 className="mb-6 text-3xl font-bold">Observability</h1>
      <ObservabilityClient />
    </PageContainer>
  )
}
