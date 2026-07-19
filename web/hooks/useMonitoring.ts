import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { ObservabilityService } from '@/lib/gen/observability/v1/observability_pb'
import type {
  GetJobStatsResponse,
  GetUsageStatsResponse,
  GetStorageStatsResponse,
  GetDatabaseStatsResponse
} from '@/lib/gen/observability/v1/observability_pb'
import { swrKeys } from '@/lib/swrKeys'

export function useJobStats(windowDays: number) {
  const client = createServiceClient(ObservabilityService)
  return useSWR<GetJobStatsResponse, Error>(swrKeys.monitoringJobStats(windowDays), () =>
    client.getJobStats({ windowDays })
  )
}

export function useUsageStats(windowDays: number) {
  const client = createServiceClient(ObservabilityService)
  return useSWR<GetUsageStatsResponse, Error>(swrKeys.monitoringUsageStats(windowDays), () =>
    client.getUsageStats({ windowDays })
  )
}

export function useStorageStats() {
  const client = createServiceClient(ObservabilityService)
  return useSWR<GetStorageStatsResponse, Error>(swrKeys.monitoringStorageStats, () =>
    client.getStorageStats({})
  )
}

export function useDatabaseStats() {
  const client = createServiceClient(ObservabilityService)
  return useSWR<GetDatabaseStatsResponse, Error>(swrKeys.monitoringDatabaseStats, () =>
    client.getDatabaseStats({})
  )
}
