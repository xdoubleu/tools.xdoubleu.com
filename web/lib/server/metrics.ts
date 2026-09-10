import { Histogram, Registry, collectDefaultMetrics } from 'prom-client'

// The standalone Node server exposes a Prometheus scrape endpoint at
// /metrics (app/metrics/route.ts) so Grafana's FrontendP95High alert rule
// (issue #1528) can evaluate real browser Web-Vitals latency instead of
// leaning on Sentry. The registry is kept on globalThis so Next's dev-mode
// module reloads don't register the same collector twice.
type MetricsState = {
  registry: Registry
  webVitals: Histogram<'metric'>
}

const globalForMetrics = globalThis as typeof globalThis & {
  __webMetrics__?: MetricsState
}

function createState(): MetricsState {
  const registry = new Registry()
  registry.setDefaultLabels({ job: 'web' })
  collectDefaultMetrics({ register: registry })

  const webVitals = new Histogram({
    name: 'web_vitals_seconds',
    help: 'Core Web Vitals reported by the browser, in seconds.',
    labelNames: ['metric'],
    // CLS is unitless and INP/LCP/FCP/TTFB are milliseconds converted to
    // seconds on ingest; these buckets mirror the api request histogram.
    buckets: [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10],
    registers: [registry]
  })

  return { registry, webVitals }
}

const state = (globalForMetrics.__webMetrics__ ??= createState())

export const metricsRegistry = state.registry

// KNOWN_WEB_VITALS bounds the "metric" label cardinality — anything the
// browser reports outside this set is dropped rather than trusted.
const KNOWN_WEB_VITALS = new Set(['CLS', 'FCP', 'INP', 'LCP', 'TTFB', 'FID'])

// recordWebVital observes one browser-reported metric. valueMs is the raw
// value from useReportWebVitals (milliseconds for timing metrics, a small
// unitless number for CLS); both are stored on the seconds-scaled histogram.
export function recordWebVital(name: string, valueMs: number): boolean {
  if (!KNOWN_WEB_VITALS.has(name) || !Number.isFinite(valueMs) || valueMs < 0) {
    return false
  }
  state.webVitals.labels(name).observe(valueMs / 1000)
  return true
}
