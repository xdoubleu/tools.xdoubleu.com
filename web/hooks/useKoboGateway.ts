import useSWR from 'swr'
import { probeGateway, type GatewayStatus } from '@/lib/reading/gatewayClient'
import { swrKeys } from '@/lib/swrKeys'

const POLL_INTERVAL_MS = 2000

/**
 * Polls the local kobo-gateway helper for its status. Resolves to `null`
 * data (not an error) when the gateway isn't reachable, so callers show a
 * download prompt instead of an error state.
 */
export function useGatewayStatus() {
  return useSWR<GatewayStatus | null>(swrKeys.gatewayStatus, () => probeGateway(), {
    refreshInterval: POLL_INTERVAL_MS
  })
}
