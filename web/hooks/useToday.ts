import useSWR from 'swr'
import { createServiceClient } from '@/lib/client'
import { TodayService } from '@/lib/gen/dashboard/v1/today_pb'
import type { GetTodayResponse } from '@/lib/gen/dashboard/v1/today_pb'
import { swrKeys } from '@/lib/swrKeys'

/** The Today summary: one block per app the user can use. */
export function useToday() {
  const client = createServiceClient(TodayService)
  return useSWR<GetTodayResponse, Error>(swrKeys.today, () => client.getToday({}))
}
