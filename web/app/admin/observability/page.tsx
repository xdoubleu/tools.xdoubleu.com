import ObservabilityClient from '@/components/admin/observability/ObservabilityClient'
import SWRFallback from '@/components/SWRFallback'
import { createServerClient } from '@/lib/server/client'
import { fetchOrNull } from '@/lib/server/fetchers'
import { swrKeys } from '@/lib/swrKeys'
import { AdminService } from '@/lib/gen/admin/v1/admin_pb'

// Matches the default window in ObservabilityClient so the prefetch lands on
// the same SWR key the client reads on mount.
const DEFAULT_WINDOW_DAYS = 30

export default async function ObservabilityPage() {
  const client = await createServerClient(AdminService)

  const [jobStats, usageStats, storageStats, databaseStats] = await Promise.all([
    fetchOrNull(() => client.getJobStats({ windowDays: DEFAULT_WINDOW_DAYS })),
    fetchOrNull(() => client.getUsageStats({ windowDays: DEFAULT_WINDOW_DAYS })),
    fetchOrNull(() => client.getStorageStats({})),
    fetchOrNull(() => client.getDatabaseStats({}))
  ])

  const fallback: Record<string, unknown> = {}
  if (storageStats) fallback[swrKeys.adminStorageStats] = storageStats
  if (databaseStats) fallback[swrKeys.adminDatabaseStats] = databaseStats

  const keyed: [readonly unknown[], unknown][] = []
  if (jobStats) keyed.push([swrKeys.adminJobStats(DEFAULT_WINDOW_DAYS), jobStats])
  if (usageStats) keyed.push([swrKeys.adminUsageStats(DEFAULT_WINDOW_DAYS), usageStats])

  return (
    <SWRFallback fallback={fallback} keyed={keyed}>
      <ObservabilityClient />
    </SWRFallback>
  )
}
