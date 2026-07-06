import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { AdminService } from '@/lib/gen/admin/v1/admin_pb'
import type {
  GetJobStatsResponse,
  GetUsageStatsResponse,
  GetStorageStatsResponse,
  GetDatabaseStatsResponse
} from '@/lib/gen/admin/v1/admin_pb'
import { swrKeys } from '@/lib/swrKeys'

export function useJobStats(windowDays: number) {
  const client = createServiceClient(AdminService)
  return useSWR<GetJobStatsResponse, Error>(swrKeys.adminJobStats(windowDays), () =>
    client.getJobStats({ windowDays })
  )
}

export function useUsageStats(windowDays: number) {
  const client = createServiceClient(AdminService)
  return useSWR<GetUsageStatsResponse, Error>(swrKeys.adminUsageStats(windowDays), () =>
    client.getUsageStats({ windowDays })
  )
}

export function useStorageStats() {
  const client = createServiceClient(AdminService)
  return useSWR<GetStorageStatsResponse, Error>(swrKeys.adminStorageStats, () =>
    client.getStorageStats({})
  )
}

export function useDatabaseStats() {
  const client = createServiceClient(AdminService)
  return useSWR<GetDatabaseStatsResponse, Error>(swrKeys.adminDatabaseStats, () =>
    client.getDatabaseStats({})
  )
}
