import useSWR from 'swr'
import { probeGateway, type GatewayStatus } from '@/lib/books/gatewayClient'
import { swrKeys } from '@/lib/swrKeys'

const POLL_INTERVAL_MS = 2000

/** Polls the local kobo-gateway; unreachable resolves to `null`, not an error. */
export function useGatewayStatus() {
  return useSWR<GatewayStatus | null>(swrKeys.gatewayStatus, () => probeGateway(), {
    refreshInterval: POLL_INTERVAL_MS
  })
}
