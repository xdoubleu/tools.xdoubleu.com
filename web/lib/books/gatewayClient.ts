/**
 * Client for the local kobo-gateway: a downloadable macOS helper that
 * exposes a loopback-only HTTP API (see api/internal/kobogateway). The
 * browser keeps making all authenticated API calls itself and hands only
 * the resulting sync URL to the gateway, which patches the USB-mounted
 * Kobo's config file.
 */

const GATEWAY_PORT = 41132
const GATEWAY_URL = `http://127.0.0.1:${GATEWAY_PORT}`

/**
 * Minimum gateway protocol version this web app can drive. When a probe
 * reports an older version the UI triggers a self-update via updateGateway.
 */
export const REQUIRED_GATEWAY_VERSION = 1

export const GATEWAY_DOWNLOAD_PATH = '/downloads/kobo-gateway-darwin-arm64'

export interface GatewayKobo {
  volumePath: string
  serial: string
  currentEndpoint: string
}

export interface GatewayStatus {
  version: number
  release: string
  kobos: GatewayKobo[]
}

async function gatewayFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${GATEWAY_URL}${path}`, init)
  const body: unknown = await res.json().catch(() => null)
  if (!res.ok) {
    const message =
      body && typeof body === 'object' && 'error' in body && typeof body.error === 'string'
        ? body.error
        : `Gateway request failed (${res.status})`
    throw new Error(message)
  }
  // Response shapes are defined by the gateway (api/internal/kobogateway/types.go).
  // eslint-disable-next-line @typescript-eslint/no-unsafe-type-assertion
  return body as T
}

function gatewayPost<T>(path: string, payload: Record<string, unknown>): Promise<T> {
  return gatewayFetch<T>(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload)
  })
}

/**
 * Detects a running gateway. Resolves to null on any failure (not
 * installed, not running, blocked fetch, timeout) so callers can fall back
 * to the other setup flows.
 */
export async function probeGateway(timeoutMs = 1500): Promise<GatewayStatus | null> {
  try {
    return await gatewayFetch<GatewayStatus>('/status', {
      signal: AbortSignal.timeout(timeoutMs)
    })
  } catch {
    return null
  }
}

export function configureGateway(
  syncUrl: string,
  volumePath?: string
): Promise<{ serial: string; originalEndpoint: string }> {
  return gatewayPost('/configure', { syncUrl, ...(volumePath ? { volumePath } : {}) })
}

export function revertGateway(
  targetEndpoint: string,
  volumePath?: string
): Promise<{ serial: string }> {
  return gatewayPost('/revert', { targetEndpoint, ...(volumePath ? { volumePath } : {}) })
}

/**
 * Asks the gateway to download the latest binary from this web origin,
 * replace itself, and restart. Callers should poll probeGateway afterwards
 * until the new version reports in.
 */
export function updateGateway(): Promise<{ updating: boolean }> {
  return gatewayPost('/update', {})
}
