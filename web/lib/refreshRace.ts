// Refresh tokens are single-use, so when several requests carry the same
// expired session only the first rotates it; the others answer 401 although
// fresh cookies are on their way. Every request holds a shared Web Lock (all
// tabs); a 401 retries once under the exclusive lock, which waits for every
// in-flight request, including the one that rotated.

const lockName = 'tools-api-session'

// Their 401 means wrong credentials: a retry would count a second failed attempt.
const credentialRpcs = [
  'SignIn',
  'MFAChallenge',
  'ExchangeToken',
  'UpdatePassword',
  'ResetPassword',
  'MFAUnenroll',
  'RegenerateRecoveryCodes'
].map((rpc) => `/auth.v1.AuthService/${rpc}`)

// Without Web Locks, wait for this tab's requests instead.
const inFlight = new Set<Promise<Response>>()

function lockManager(): LockManager | undefined {
  return typeof navigator === 'undefined' ? undefined : navigator.locks
}

function isCredentialRpc(input: RequestInfo | URL): boolean {
  const url = input instanceof Request ? input.url : String(input)
  return credentialRpcs.some((path) => url.endsWith(path))
}

function track(request: Promise<Response>): Promise<Response> {
  inFlight.add(request)
  void request.finally(() => inFlight.delete(request)).catch(() => undefined)
  return request
}

/** Sends `send`, retrying once a 401 that may have lost the refresh-token race. */
export async function fetchSurvivingRefreshRace(
  input: RequestInfo | URL,
  send: () => Promise<Response>
): Promise<Response> {
  const locks = lockManager()
  const res = await track(locks ? locks.request(lockName, { mode: 'shared' }, send) : send())
  if (res.status !== 401 || isCredentialRpc(input)) return res

  if (locks) return locks.request(lockName, { mode: 'exclusive' }, send)
  await Promise.allSettled([...inFlight])
  return send()
}
