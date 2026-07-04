import { cache } from 'react'
import { cookies } from 'next/headers'
import { createConnectTransport } from '@connectrpc/connect-web'
import { createClient, type Client } from '@connectrpc/connect'
import type { DescService } from '@bufbuild/protobuf'
import { getApiUrl } from '@/lib/env'

// Server-side ConnectRPC client factory for React Server Components.
//
// Unlike lib/client.ts (browser: shared transport, cookies attached by the
// browser via credentials:'include'), the server must forward the incoming
// request's Cookie header itself, so the transport is built per request.
// getApiUrl() resolves process.env.API_URL on the server.

export function serverFetch(cookieHeader: string): typeof fetch {
  return (input, init) => {
    const headers = new Headers(init?.headers)
    if (cookieHeader) headers.set('cookie', cookieHeader)
    return fetch(input, { ...init, headers, cache: 'no-store' })
  }
}

// cache() memoizes per RSC render pass, so parallel fetches in one request
// share a single transport (and a single cookies() read).
const getTransport = cache(async () => {
  const store = await cookies()
  // The refresh token is deliberately NOT forwarded: a server component
  // cannot persist rotated cookies, so a server-triggered refresh (e.g. via
  // GetCurrentUser) would invalidate the refresh token the browser still
  // holds. Expired sessions therefore 401 here and recover through the
  // client-side SWR fetch, which refreshes in the browser.
  const cookieHeader = store
    .getAll()
    .filter((c) => c.name !== 'refreshToken')
    .map((c) => `${c.name}=${c.value}`)
    .join('; ')
  return createConnectTransport({
    baseUrl: getApiUrl(),
    useBinaryFormat: true,
    fetch: serverFetch(cookieHeader)
  })
})

export async function createServerClient<T extends DescService>(service: T): Promise<Client<T>> {
  return createClient(service, await getTransport())
}
