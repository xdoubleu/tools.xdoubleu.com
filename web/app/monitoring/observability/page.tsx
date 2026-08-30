import ObservabilityClient from '@/components/monitoring/ObservabilityClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { ObservabilityService } from '@/lib/gen/observability/v1/observability_pb'

// Matches the default window in ObservabilityClient so the prefetch lands on
// the same SWR key the client reads on mount.
const DEFAULT_WINDOW_DAYS = 30

export default async function MonitoringObservabilityPage() {
  const client = await createServerClient(ObservabilityService)

  const [jobStats, storageStats, databaseStats, hostMetrics, transactionLatencyHistory] =
    await Promise.all([
      fetchOrNull(() => client.getJobStats({ windowDays: DEFAULT_WINDOW_DAYS })),
      fetchOrNull(() => client.getStorageStats({})),
      fetchOrNull(() => client.getDatabaseStats({ windowDays: DEFAULT_WINDOW_DAYS })),
      fetchOrNull(() => client.getHostMetrics({})),
      fetchOrNull(() => client.getTransactionLatencyHistory({ windowDays: DEFAULT_WINDOW_DAYS }))
    ])

  const fallback: Record<string, unknown> = {}
  if (storageStats) fallback[swrKeys.monitoringStorageStats] = storageStats
  if (hostMetrics) fallback[swrKeys.monitoringHostMetrics] = hostMetrics

  const keyed: [readonly unknown[], unknown][] = []
  if (jobStats) keyed.push([swrKeys.monitoringJobStats(DEFAULT_WINDOW_DAYS), jobStats])
  if (databaseStats) {
    keyed.push([swrKeys.monitoringDatabaseStats(DEFAULT_WINDOW_DAYS), databaseStats])
  }
  if (transactionLatencyHistory) {
    keyed.push([
      swrKeys.monitoringTransactionLatencyHistory(DEFAULT_WINDOW_DAYS),
      transactionLatencyHistory
    ])
  }

  return (
    <SWRFallback fallback={fallback} keyed={keyed}>
      <ObservabilityClient />
    </SWRFallback>
  )
}
