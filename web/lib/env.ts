declare global {
  interface Window {
    __ENV__: {
      API_URL: string
      SENTRY_DSN_WEB: string
      RELEASE: string
      KOBO_GATEWAY_RELEASE: string
      POSTHOG_KEY: string
      POSTHOG_HOST: string
    }
  }
}

export function getApiUrl(): string {
  if (typeof window !== 'undefined') return window.__ENV__?.API_URL ?? ''
  return process.env.API_URL ?? ''
}

export function getSentryDsn(): string {
  if (typeof window !== 'undefined') return window.__ENV__?.SENTRY_DSN_WEB ?? ''
  return process.env.SENTRY_DSN_WEB ?? ''
}

export function getRelease(): string {
  if (typeof window !== 'undefined') return window.__ENV__?.RELEASE ?? 'dev'
  return process.env.RELEASE ?? 'dev'
}

// getKoboGatewayRelease returns the bundled kobo-gateway's release, which can
// lag getRelease() when its build cache-hit; gatewayNeedsUpdate compares
// against this.
export function getKoboGatewayRelease(): string {
  if (typeof window !== 'undefined') {
    return window.__ENV__?.KOBO_GATEWAY_RELEASE ?? 'dev'
  }
  return process.env.KOBO_GATEWAY_RELEASE ?? 'dev'
}

// PostHog Cloud (EU) client key; ships in the bundle anyway.
export function getPostHogKey(): string {
  if (typeof window !== 'undefined') return window.__ENV__?.POSTHOG_KEY ?? ''
  return process.env.POSTHOG_KEY ?? ''
}

export function getPostHogHost(): string {
  if (typeof window !== 'undefined') return window.__ENV__?.POSTHOG_HOST ?? ''
  return process.env.POSTHOG_HOST ?? ''
}

// getObservabilityIngestSecret authenticates web's server-side log POSTs.
// No window.__ENV__ fallback: it must never reach the browser (clients go
// through app/logs/route.ts).
export function getObservabilityIngestSecret(): string {
  if (typeof window !== 'undefined') return ''
  return process.env.OBSERVABILITY_INGEST_SECRET ?? ''
}
