import ObservabilityClient from '@/components/monitoring/ObservabilityClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { ObservabilityService } from '@/lib/gen/observability/v1/observability_pb'

// Matches the default window in ObservabilityClient so the prefetch lands on
// the same SWR key the client reads on mount.
const DEFAULT_WINDOW_DAYS = 30

export default async function MonitoringPage() {
  const client = await createServerClient(ObservabilityService)

  const [jobStats, usageStats, storageStats, databaseStats] = await Promise.all([
    fetchOrNull(() => client.getJobStats({ windowDays: DEFAULT_WINDOW_DAYS })),
    fetchOrNull(() => client.getUsageStats({ windowDays: DEFAULT_WINDOW_DAYS })),
    fetchOrNull(() => client.getStorageStats({})),
    fetchOrNull(() => client.getDatabaseStats({}))
  ])

  const fallback: Record<string, unknown> = {}
  if (storageStats) fallback[swrKeys.monitoringStorageStats] = storageStats
  if (databaseStats) fallback[swrKeys.monitoringDatabaseStats] = databaseStats

  const keyed: [readonly unknown[], unknown][] = []
  if (jobStats) keyed.push([swrKeys.monitoringJobStats(DEFAULT_WINDOW_DAYS), jobStats])
  if (usageStats) keyed.push([swrKeys.monitoringUsageStats(DEFAULT_WINDOW_DAYS), usageStats])

  return (
    <SWRFallback fallback={fallback} keyed={keyed}>
      <ObservabilityClient />
    </SWRFallback>
  )
}
