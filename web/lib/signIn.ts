import { Code, ConnectError } from '@connectrpc/connect'

// Pages a signed-out visitor may stay on: the home page and auth pages (both
// show the sign-in form), OAuth consent (which sends to sign-in itself), and
// token-shared dashboards.
const PUBLIC_PATHS = [/^\/$/, /^\/auth\//, /^\/oauth\//, /^\/dashboard\/(games|reading)\/[^/]+/]

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
