// Runtime config the api injects as /env.js (the dev server serves the same),
// so one build serves every environment.
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
  return window.__ENV__?.API_URL ?? ''
}

export function getSentryDsn(): string {
  return window.__ENV__?.SENTRY_DSN_WEB ?? ''
}

export function getRelease(): string {
  return window.__ENV__?.RELEASE ?? 'dev'
}

// getKoboGatewayRelease returns the bundled kobo-gateway's release, which can
// lag getRelease() when its build cache-hit; gatewayNeedsUpdate compares
// against this.
export function getKoboGatewayRelease(): string {
  return window.__ENV__?.KOBO_GATEWAY_RELEASE ?? 'dev'
}

// PostHog Cloud (EU) client key; ships in the bundle anyway.
export function getPostHogKey(): string {
  return window.__ENV__?.POSTHOG_KEY ?? ''
}

export function getPostHogHost(): string {
  return window.__ENV__?.POSTHOG_HOST ?? ''
}
