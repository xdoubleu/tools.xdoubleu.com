import { cache } from 'react'
import { cookies } from 'next/headers'
import { createConnectTransport } from '@connectrpc/connect-web'
import { createClient, type Client } from '@connectrpc/connect'
import type { DescService } from '@bufbuild/protobuf'
import { getApiUrl } from '@/lib/env'

// Server-side ConnectRPC client for RSCs. Unlike lib/client.ts, it forwards
// the request's Cookie header itself, so the transport is per request.

// kamal-proxy returns 502 while the api container is mid-restart (rolling
// deploy); the RSC fetch is a read-only prefetch, so retry briefly rather
// than surface a transient ConnectError in Sentry.
const retryableStatuses = new Set([502])
const maxRetries = 2
const retryDelayMs = 250

export function serverFetch(cookieHeader: string): typeof fetch {
  return async (input, init) => {
    const headers = new Headers(init?.headers)
    if (cookieHeader) headers.set('cookie', cookieHeader)
    const request = { ...init, headers, cache: 'no-store' as const }
    for (let attempt = 0; ; attempt++) {
      const res = await fetch(input, request)
      if (!retryableStatuses.has(res.status) || attempt >= maxRetries) return res
      await new Promise((resolve) => setTimeout(resolve, retryDelayMs))
    }
  }
}

// Memoized per render pass.
const getTransport = cache(async () => {
  const store = await cookies()
  // Never forward the refresh token: an RSC can't persist rotated cookies, so a
  // refresh here would invalidate the browser's token. Expired sessions 401 and
  // recover via the client-side SWR fetch.
  const cookieHeader = store
    .getAll()
    .filter((c) => c.name !== 'refreshToken')
    .map((c) => `${c.name}=${c.value}`)
    .join('; ')
  return createConnectTransport({
    baseUrl: getApiUrl(),
    useBinaryFormat: true,
    // Node fetch has no default timeout, so a hung api would block the render
    // forever. DeadlineExceeded → fetchOrNull returns null; the header also
    // cancels the api handler. The browser transport stays uncapped for slow
    // uploads.
    defaultTimeoutMs: 10000,
    fetch: serverFetch(cookieHeader)
  })
})

export async function createServerClient<T extends DescService>(service: T): Promise<Client<T>> {
  return createClient(service, await getTransport())
}
