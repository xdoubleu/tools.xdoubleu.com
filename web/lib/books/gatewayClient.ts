/**
 * Client for the local kobo-gateway's loopback HTTPS API (HTTPS so Safari
 * allows it from an HTTPS page). The browser makes all authenticated calls
 * and hands only the sync URL to the gateway, which patches the Kobo config.
 */

import { getKoboGatewayRelease } from '@/lib/env'

const GATEWAY_PORT = 41132
// middleware.ts allows this in the CSP connect-src.
export const GATEWAY_URL = `https://127.0.0.1:${GATEWAY_PORT}`

/**
 * Minimum gateway protocol version; older triggers a self-update. Bump only
 * for HTTP API/file-handling breaks — gatewayNeedsUpdate's release check
 * catches routine releases.
 */
export const REQUIRED_GATEWAY_VERSION = 2

// The download button's .dmg; the self-updater fetches the raw binary instead.
export const GATEWAY_DOWNLOAD_PATH = '/downloads/kobo-gateway.dmg'

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

/** Detects a running gateway; resolves to null on any failure. */
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
 * True when the gateway is below the required protocol version or its release
 * differs from the bundled artifact's (getKoboGatewayRelease(), which can lag
 * web's own release when its build cache-hit).
 */
export function gatewayNeedsUpdate(status: GatewayStatus): boolean {
  if (status.version < REQUIRED_GATEWAY_VERSION) return true

  const current = getKoboGatewayRelease()
  if (current === 'dev' || status.release === 'dev') return false

  return status.release !== current
}

/**
 * Asks the gateway to download the latest binary, replace itself, and
 * restart; poll probeGateway afterwards.
 */
export function updateGateway(): Promise<{ updating: boolean }> {
  return gatewayPost('/update', {})
}
