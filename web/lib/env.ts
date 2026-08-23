declare global {
  interface Window {
    __ENV__: {
      API_URL: string
      SENTRY_DSN: string
      RELEASE: string
      KOBO_GATEWAY_RELEASE: string
    }
  }
}

export function getApiUrl(): string {
  if (typeof window !== 'undefined') return window.__ENV__?.API_URL ?? ''
  return process.env.API_URL ?? ''
}

export function getSentryDsn(): string {
  if (typeof window !== 'undefined') return window.__ENV__?.SENTRY_DSN ?? ''
  return process.env.SENTRY_DSN ?? ''
}

export function getRelease(): string {
  if (typeof window !== 'undefined') return window.__ENV__?.RELEASE ?? 'dev'
  return process.env.RELEASE ?? 'dev'
}

// getKoboGatewayRelease returns the release actually baked into the
// kobo-gateway .dmg/binary currently bundled in this deploy — can lag
// behind getRelease() when kobo-gateway's own build was skipped (unchanged
// source). gatewayNeedsUpdate compares against this, not getRelease(), so
// it detects a genuinely newer kobo-gateway build rather than every deploy.
export function getKoboGatewayRelease(): string {
  if (typeof window !== 'undefined') {
    return window.__ENV__?.KOBO_GATEWAY_RELEASE ?? 'dev'
  }
  return process.env.KOBO_GATEWAY_RELEASE ?? 'dev'
}

// getObservabilityIngestSecret authenticates web's own server-side POSTs to
// the api's shared-secret log ingest endpoint (OBSERVABILITY_INGEST_SECRET,
// see api/cmd/api/observability_ingest.go). Deliberately has no window.__ENV__
// fallback, unlike every other getter here — this must never reach the
// browser bundle. Callers on the client must instead route through
// app/logs/route.ts, which calls this server-side and attaches the header
// itself (lib/logger.ts does exactly this).
export function getObservabilityIngestSecret(): string {
  if (typeof window !== 'undefined') return ''
  return process.env.OBSERVABILITY_INGEST_SECRET ?? ''
}
