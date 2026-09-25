import { cache } from 'react'
import { cookies } from 'next/headers'
import { createConnectTransport } from '@connectrpc/connect-web'
import { createClient, type Client } from '@connectrpc/connect'
import type { DescService } from '@bufbuild/protobuf'
import { getApiUrl } from '@/lib/env'

// Server-side ConnectRPC client for RSCs. Unlike lib/client.ts, it forwards
// the request's Cookie header itself, so the transport is per request.

export function serverFetch(cookieHeader: string): typeof fetch {
  return (input, init) => {
    const headers = new Headers(init?.headers)
    if (cookieHeader) headers.set('cookie', cookieHeader)
    return fetch(input, { ...init, headers, cache: 'no-store' })
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
