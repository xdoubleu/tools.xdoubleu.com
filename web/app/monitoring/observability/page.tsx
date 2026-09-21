import { PageContainer } from '@/components/ui/page-container'
import ObservabilityClient from '@/components/monitoring/ObservabilityClient'

// No server-side prefetch here (contrast the RSC + SWRFallback pattern
// documented in web/CLAUDE.md's "Data Flow"): issue #1714 traced a 341% p95
// regression on this page to GetAutomatedActions being awaited on the SSR
// critical path. An EXPLAIN ANALYZE against a synthetic 2M-row
// global.automated_actions table showed the query itself runs in
// sub-millisecond time even at that scale (00052_automated_actions_covering_index.sql
// tightened it further), so the added latency was the extra network round
// trip itself, not the query — removing the blocking prefetch and letting
// ObservabilityClient's own SWR hook fetch client-side (it already renders a
// "Loading…" state with no data) takes that round trip off the page's
// response path entirely.
export default function MonitoringObservabilityPage() {
  return (
    <PageContainer className="p-6">
      <h1 className="mb-6 text-3xl font-bold">Observability</h1>
      <ObservabilityClient />
    </PageContainer>
  )
}
