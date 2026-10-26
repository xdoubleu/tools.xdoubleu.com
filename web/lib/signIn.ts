import { Code, ConnectError } from '@connectrpc/connect'

// A token-shared dashboard: /dashboard/<app>/<token>/…
const SHARED_DASHBOARD = /^\/dashboard\/[^/]+\/[^/]+/

// Pages a signed-out visitor may stay on: the home page and auth pages (both
// show the sign-in form), OAuth consent (which sends to sign-in itself), and
// shared dashboards.
const PUBLIC_PATHS = [/^\/$/, /^\/auth\//, /^\/oauth\//, SHARED_DASHBOARD]

/** True on a page someone opened through a share link, where owner chrome is hidden. */
export function isSharedDashboard(pathname: string): boolean {
  return SHARED_DASHBOARD.test(pathname)
}

/** True when the api answered that there's no session. A network error isn't one, so offline reads keep working. */
export function isSignedOut(error: unknown): boolean {
  return error instanceof ConnectError && error.code === Code.Unauthenticated
}

/** Where to send a visitor whose current-user lookup failed with `error`, or null to stay. */
export function signInRedirect(error: unknown, pathname: string, search: string): string | null {
  if (!isSignedOut(error) || PUBLIC_PATHS.some((p) => p.test(pathname))) return null
  const next = search ? `${pathname}?${search}` : pathname
  return `/auth/sign-in?next=${encodeURIComponent(next)}`
}
